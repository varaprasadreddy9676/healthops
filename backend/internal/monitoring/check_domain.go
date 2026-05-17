package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

type domainCheckExecutor struct{}

func init() { RegisterCheckExecutor(&domainCheckExecutor{}) }

func (e *domainCheckExecutor) Type() string { return "domain" }

func (e *domainCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.Domain == nil {
		c.Domain = &DomainCheckConfig{}
	}
	if c.Domain.WarningDays <= 0 {
		c.Domain.WarningDays = 30
	}
	if c.Domain.CriticalDays <= 0 {
		c.Domain.CriticalDays = 7
	}
}

func (e *domainCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	domain := ""
	if c.Domain != nil {
		domain = c.Domain.Domain
	}
	if domain == "" {
		domain = c.Target
	}
	if domain == "" {
		return fmt.Errorf("domain check requires domain.domain or target")
	}
	if c.Domain != nil && c.Domain.CriticalDays > c.Domain.WarningDays {
		return fmt.Errorf("domain.criticalDays (%d) must be <= domain.warningDays (%d)", c.Domain.CriticalDays, c.Domain.WarningDays)
	}
	return nil
}

func (e *domainCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	cfg := check.Domain
	if cfg == nil {
		cfg = &DomainCheckConfig{WarningDays: 30, CriticalDays: 7}
	}

	domain := cfg.Domain
	if domain == "" {
		domain = check.Target
	}
	// Allow URL-style targets like "https://example.com" — extract host
	domain = parseHostFromTarget(domain)

	// Primary: RDAP (works in containers without whois). Falls back to whois only
	// when RDAP returns an unrecoverable error.
	expiry, err := e.lookupRDAP(ctx, domain, cfg.RDAPEndpoint)
	if err != nil {
		// Fallback to whois if available
		if _, lookErr := exec.LookPath("whois"); lookErr == nil {
			whoisExpiry, whoisErr := e.lookupWhois(ctx, domain)
			if whoisErr != nil {
				return fmt.Errorf("rdap and whois both failed: rdap=%v, whois=%v", err, whoisErr)
			}
			expiry = whoisExpiry
		} else {
			return fmt.Errorf("rdap lookup for %s failed: %w (whois binary not installed)", domain, err)
		}
	}

	now := time.Now().UTC()
	daysUntilExpiry := math.Floor(expiry.Sub(now).Hours() / 24)

	result.Metrics["daysUntilExpiry"] = daysUntilExpiry

	// Already expired
	if now.After(expiry) {
		return fmt.Errorf("domain %s expired on %s", domain, expiry.Format("2006-01-02"))
	}

	warningDays := cfg.WarningDays
	if warningDays <= 0 {
		warningDays = 30
	}
	criticalDays := cfg.CriticalDays
	if criticalDays <= 0 {
		criticalDays = 7
	}

	if int(daysUntilExpiry) <= criticalDays {
		return fmt.Errorf("domain %s expires in %.0f days (critical threshold: %d)", domain, daysUntilExpiry, criticalDays)
	}

	if int(daysUntilExpiry) <= warningDays {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("domain %s expires in %.0f days (warning threshold: %d)", domain, daysUntilExpiry, warningDays)
		return nil
	}

	return nil
}

// lookupWhois runs the whois command and parses the result.
func (e *domainCheckExecutor) lookupWhois(ctx context.Context, domain string) (time.Time, error) {
	cmd := exec.CommandContext(ctx, "whois", domain)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return time.Time{}, fmt.Errorf("whois lookup for %s failed: %w", domain, err)
	}
	return e.parseExpiryDate(string(output))
}

// rdapBootstrap caches the IANA DNS RDAP bootstrap file.
var (
	rdapBootstrapMu       sync.RWMutex
	rdapBootstrapServices map[string]string // tld → base URL
	rdapBootstrapFetched  time.Time
)

const rdapBootstrapURL = "https://data.iana.org/rdap/dns.json"

// lookupRDAP queries the RDAP service for a domain's expiry date.
func (e *domainCheckExecutor) lookupRDAP(ctx context.Context, domain, overrideEndpoint string) (time.Time, error) {
	baseURL := overrideEndpoint
	if baseURL == "" {
		var err error
		baseURL, err = e.rdapBaseForDomain(ctx, domain)
		if err != nil {
			return time.Time{}, fmt.Errorf("resolve rdap endpoint: %w", err)
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	url := fmt.Sprintf("%s/domain/%s", baseURL, domain)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Accept", "application/rdap+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("rdap request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return time.Time{}, fmt.Errorf("rdap returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Events []struct {
			EventAction string `json:"eventAction"`
			EventDate   string `json:"eventDate"`
		} `json:"events"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return time.Time{}, fmt.Errorf("decode rdap response: %w", err)
	}

	for _, ev := range payload.Events {
		if strings.EqualFold(ev.EventAction, "expiration") {
			if t, err := time.Parse(time.RFC3339, ev.EventDate); err == nil {
				return t.UTC(), nil
			}
			if t, err := e.parseDate(ev.EventDate); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("no expiration event in rdap response for %s", domain)
}

// rdapBaseForDomain returns the RDAP base URL for the TLD of the given domain,
// using IANA's bootstrap registry (cached for 24h).
func (e *domainCheckExecutor) rdapBaseForDomain(ctx context.Context, domain string) (string, error) {
	parts := strings.Split(strings.TrimSuffix(domain, "."), ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid domain %q", domain)
	}
	tld := strings.ToLower(parts[len(parts)-1])

	services, err := e.loadRDAPBootstrap(ctx)
	if err != nil {
		return "", err
	}
	base, ok := services[tld]
	if !ok {
		return "", fmt.Errorf("no rdap service registered for tld %q", tld)
	}
	return base, nil
}

// loadRDAPBootstrap fetches and caches the IANA bootstrap registry.
func (e *domainCheckExecutor) loadRDAPBootstrap(ctx context.Context) (map[string]string, error) {
	rdapBootstrapMu.RLock()
	if rdapBootstrapServices != nil && time.Since(rdapBootstrapFetched) < 24*time.Hour {
		defer rdapBootstrapMu.RUnlock()
		return rdapBootstrapServices, nil
	}
	rdapBootstrapMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rdapBootstrapURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch rdap bootstrap: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rdap bootstrap HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Services [][]interface{} `json:"services"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode rdap bootstrap: %w", err)
	}

	// services entries are [[tlds...], [urls...]] pairs
	out := make(map[string]string)
	for _, svc := range payload.Services {
		if len(svc) < 2 {
			continue
		}
		tlds, _ := svc[0].([]interface{})
		urls, _ := svc[1].([]interface{})
		if len(urls) == 0 {
			continue
		}
		// pick the first https url
		var chosen string
		for _, u := range urls {
			if s, ok := u.(string); ok && strings.HasPrefix(s, "https://") {
				chosen = s
				break
			}
		}
		if chosen == "" {
			if s, ok := urls[0].(string); ok {
				chosen = s
			}
		}
		for _, t := range tlds {
			if s, ok := t.(string); ok {
				out[strings.ToLower(s)] = chosen
			}
		}
	}

	rdapBootstrapMu.Lock()
	rdapBootstrapServices = out
	rdapBootstrapFetched = time.Now()
	rdapBootstrapMu.Unlock()

	return out, nil
}

// parseExpiryDate extracts the domain expiry date from WHOIS output.
// Handles common WHOIS formats from major registrars and registries.
func (e *domainCheckExecutor) parseExpiryDate(whois string) (time.Time, error) {
	// Common WHOIS expiry field labels
	patterns := []string{
		`(?i)Registry Expiry Date:\s*(.+)`,
		`(?i)Registrar Registration Expiration Date:\s*(.+)`,
		`(?i)Expir(?:y|ation|es) Date:\s*(.+)`,
		`(?i)paid-till:\s*(.+)`,
		`(?i)Expiry date:\s*(.+)`,
		`(?i)renewal date:\s*(.+)`,
		`(?i)expire:\s*(.+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		m := re.FindStringSubmatch(whois)
		if len(m) > 1 {
			dateStr := strings.TrimSpace(m[1])
			t, err := e.parseDate(dateStr)
			if err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("could not find expiry date in WHOIS output")
}

// parseDate tries multiple date formats common in WHOIS responses.
func (e *domainCheckExecutor) parseDate(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"January 02 2006",
		"02/01/2006",
		"2006/01/02",
		"20060102",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date %q", s)
}
