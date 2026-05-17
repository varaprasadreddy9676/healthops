package monitoring

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

type dnsCheckExecutor struct{}

func init() { RegisterCheckExecutor(&dnsCheckExecutor{}) }

func (e *dnsCheckExecutor) Type() string { return "dns" }

func (e *dnsCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.DNS == nil {
		c.DNS = &DNSCheckConfig{}
	}
	if c.DNS.RecordType == "" {
		c.DNS.RecordType = "A"
	}
}

func (e *dnsCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	name := ""
	if c.DNS != nil {
		name = c.DNS.Name
	}
	if name == "" {
		name = c.Target
	}
	if name == "" {
		return fmt.Errorf("dns check requires dns.name or target")
	}
	if c.DNS != nil {
		rt := strings.ToUpper(c.DNS.RecordType)
		switch rt {
		case "A", "AAAA", "CNAME", "TXT", "MX", "NS":
			// valid
		default:
			return fmt.Errorf("unsupported dns recordType %q (use A, AAAA, CNAME, TXT, MX, NS)", rt)
		}
	}
	return nil
}

func (e *dnsCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	cfg := check.DNS
	if cfg == nil {
		cfg = &DNSCheckConfig{RecordType: "A"}
	}

	name := cfg.Name
	if name == "" {
		name = check.Target
	}

	recordType := strings.ToUpper(cfg.RecordType)
	if recordType == "" {
		recordType = "A"
	}

	// Use custom resolver if specified
	resolver := net.DefaultResolver
	if cfg.Resolver != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", cfg.Resolver)
			},
		}
	}

	start := time.Now()
	records, err := e.lookup(ctx, resolver, name, recordType)
	latency := time.Since(start)

	result.Metrics["latencyMs"] = float64(latency.Milliseconds())
	result.Metrics["recordCount"] = float64(len(records))

	if err != nil {
		return fmt.Errorf("dns lookup %s %s failed: %w", recordType, name, err)
	}

	if len(records) == 0 {
		return fmt.Errorf("dns lookup %s %s returned no records", recordType, name)
	}

	// Check expected values
	if len(cfg.Expected) > 0 {
		recordSet := make(map[string]bool, len(records))
		for _, r := range records {
			recordSet[r] = true
		}
		for _, exp := range cfg.Expected {
			found := false
			if recordType == "TXT" {
				// substring match for TXT
				for _, rec := range records {
					if strings.Contains(rec, exp) {
						found = true
						break
					}
				}
			} else {
				found = recordSet[exp]
			}
			if !found {
				return fmt.Errorf("dns %s %s: expected value %q not found in %v", recordType, name, exp, records)
			}
		}
	}

	// Check mustNotContain
	for _, blocked := range cfg.MustNotContain {
		for _, rec := range records {
			if rec == blocked || (recordType == "TXT" && strings.Contains(rec, blocked)) {
				return fmt.Errorf("dns %s %s: forbidden value %q found", recordType, name, blocked)
			}
		}
	}

	return nil
}

func (e *dnsCheckExecutor) lookup(ctx context.Context, resolver *net.Resolver, name, recordType string) ([]string, error) {
	switch recordType {
	case "A":
		ips, err := resolver.LookupIPAddr(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ip := range ips {
			if ip.IP.To4() != nil {
				out = append(out, ip.IP.String())
			}
		}
		sort.Strings(out)
		return out, nil

	case "AAAA":
		ips, err := resolver.LookupIPAddr(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ip := range ips {
			if ip.IP.To4() == nil {
				out = append(out, ip.IP.String())
			}
		}
		sort.Strings(out)
		return out, nil

	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, name)
		if err != nil {
			return nil, err
		}
		return []string{strings.TrimSuffix(cname, ".")}, nil

	case "TXT":
		txts, err := resolver.LookupTXT(ctx, name)
		if err != nil {
			return nil, err
		}
		sort.Strings(txts)
		return txts, nil

	case "MX":
		mxs, err := resolver.LookupMX(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, mx := range mxs {
			out = append(out, fmt.Sprintf("%d %s", mx.Pref, strings.TrimSuffix(mx.Host, ".")))
		}
		sort.Strings(out)
		return out, nil

	case "NS":
		nss, err := resolver.LookupNS(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ns := range nss {
			out = append(out, strings.TrimSuffix(ns.Host, "."))
		}
		sort.Strings(out)
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported record type %q", recordType)
	}
}
