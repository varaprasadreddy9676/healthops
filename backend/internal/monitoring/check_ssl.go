package monitoring

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/url"
	"strings"
	"time"
)

type sslCheckExecutor struct{}

func init() { RegisterCheckExecutor(&sslCheckExecutor{}) }

func (e *sslCheckExecutor) Type() string { return "ssl" }

func (e *sslCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.SSL == nil {
		c.SSL = &SSLCheckConfig{}
	}
	if c.SSL.Port <= 0 {
		c.SSL.Port = 443
	}
	if c.SSL.WarningDays <= 0 {
		c.SSL.WarningDays = 30
	}
	if c.SSL.CriticalDays <= 0 {
		c.SSL.CriticalDays = 7
	}
}

func (e *sslCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	host := e.resolveHost(c)
	if host == "" {
		return fmt.Errorf("ssl check requires host, target, or ssl.host")
	}
	if c.SSL != nil && c.SSL.CriticalDays > c.SSL.WarningDays {
		return fmt.Errorf("ssl.criticalDays (%d) must be <= ssl.warningDays (%d)", c.SSL.CriticalDays, c.SSL.WarningDays)
	}
	return nil
}

func (e *sslCheckExecutor) resolveHost(c *CheckConfig) string {
	if c.SSL != nil && c.SSL.Host != "" {
		return c.SSL.Host
	}
	if c.Host != "" {
		return c.Host
	}
	return parseHostFromTarget(c.Target)
}

// parseHostFromTarget extracts the host portion from a target string.
// Accepts plain hosts ("example.com"), host:port pairs, and full URLs
// ("https://example.com/path"). Returns empty string for empty input.
func parseHostFromTarget(target string) string {
	if target == "" {
		return ""
	}
	// Full URL form
	if strings.Contains(target, "://") {
		if u, err := url.Parse(target); err == nil && u.Host != "" {
			if h := u.Hostname(); h != "" {
				return h
			}
			return u.Host
		}
	}
	// host:port form
	if host, _, err := net.SplitHostPort(target); err == nil {
		return host
	}
	return target
}

func (e *sslCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	cfg := check.SSL
	if cfg == nil {
		cfg = &SSLCheckConfig{Port: 443, WarningDays: 30, CriticalDays: 7}
	}

	host := e.resolveHost(&check)
	port := cfg.Port
	if port <= 0 {
		port = 443
	}
	serverName := cfg.ServerName
	if serverName == "" {
		serverName = host
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp connect to %s failed: %w", addr, err)
	}

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	defer tlsConn.Close()

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("tls handshake with %s failed: %w", addr, err)
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return fmt.Errorf("no certificates returned by %s", addr)
	}

	leaf := certs[0]
	now := time.Now().UTC()
	daysUntilExpiry := math.Floor(leaf.NotAfter.Sub(now).Hours() / 24)

	result.Metrics["daysUntilExpiry"] = daysUntilExpiry
	result.Metrics["chainLength"] = float64(len(certs))
	result.Metrics["port"] = float64(port)

	// Already expired
	if now.After(leaf.NotAfter) {
		return fmt.Errorf("certificate for %s expired on %s", host, leaf.NotAfter.Format("2006-01-02"))
	}

	// Not yet valid
	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("certificate for %s not valid until %s", host, leaf.NotBefore.Format("2006-01-02"))
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
		return fmt.Errorf("certificate for %s expires in %.0f days (critical threshold: %d)", host, daysUntilExpiry, criticalDays)
	}

	if int(daysUntilExpiry) <= warningDays {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("certificate for %s expires in %.0f days (warning threshold: %d)", host, daysUntilExpiry, warningDays)
		return nil
	}

	return nil
}
