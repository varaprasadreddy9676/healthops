package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Server.ReadTimeoutSeconds <= 0 {
		c.Server.ReadTimeoutSeconds = 10
	}
	if c.Server.WriteTimeoutSeconds <= 0 {
		c.Server.WriteTimeoutSeconds = 10
	}
	if c.Server.IdleTimeoutSeconds <= 0 {
		c.Server.IdleTimeoutSeconds = 60
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = 7
	}
	if c.CheckIntervalSeconds <= 0 {
		c.CheckIntervalSeconds = 60
	}
	if c.Workers <= 0 {
		c.Workers = 8
	}
	for i := range c.Servers {
		c.Servers[i].applyDefaults()
	}
	for i := range c.Checks {
		c.Checks[i].applyDefaults()
	}
}

func (s *RemoteServer) applyDefaults() {
	if s.Port <= 0 {
		s.Port = 22
	}
	if s.Enabled == nil {
		enabled := true
		s.Enabled = &enabled
	}
}

func (s *RemoteServer) validate() error {
	if err := validateName(s.Name); err != nil {
		return err
	}
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}
	if s.User == "" {
		return fmt.Errorf("user is required")
	}
	hasKey := s.KeyPath != "" || s.KeyEnv != ""
	hasPassword := s.Password != "" || s.PasswordEnc != "" || s.PasswordEnv != ""
	if !hasKey && !hasPassword {
		return fmt.Errorf("auth required: set keyPath/keyEnv or password/passwordEnv")
	}
	return nil
}

func (c *Config) validate() error {
	// Validate servers
	serverIDs := map[string]struct{}{}
	for i := range c.Servers {
		srv := &c.Servers[i]
		if srv.ID == "" {
			return fmt.Errorf("server at index %d must have an id", i)
		}
		if _, exists := serverIDs[srv.ID]; exists {
			return fmt.Errorf("duplicate server id %q", srv.ID)
		}
		serverIDs[srv.ID] = struct{}{}
		if err := srv.validate(); err != nil {
			return fmt.Errorf("server %q: %w", srv.ID, err)
		}
	}

	if len(c.Checks) == 0 {
		return fmt.Errorf("config must define at least one check")
	}

	seen := map[string]struct{}{}
	for i := range c.Checks {
		check := &c.Checks[i]
		if check.ID == "" {
			check.ID = buildCheckID(check)
		}
		if check.ID == "" {
			return fmt.Errorf("check at index %d must have an id or name", i)
		}
		if _, exists := seen[check.ID]; exists {
			return fmt.Errorf("duplicate check id %q", check.ID)
		}
		seen[check.ID] = struct{}{}

		// Validate serverId references an existing server
		if check.ServerId != "" {
			if _, ok := serverIDs[check.ServerId]; !ok {
				return fmt.Errorf("check %q references unknown server %q", check.ID, check.ServerId)
			}
		}

		if err := check.validate(c); err != nil {
			return fmt.Errorf("check %q: %w", check.ID, err)
		}
	}
	return nil
}

func (c *CheckConfig) applyDefaults() {
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 5
	}
	if c.ExpectedStatus == 0 {
		c.ExpectedStatus = 200
	}
	if c.Enabled == nil {
		defaultEnabled := true
		c.Enabled = &defaultEnabled
	}
	c.Type = strings.ToLower(strings.TrimSpace(c.Type))
	if c.Server == "" {
		c.Server = "default"
	}
	if c.FailuresToOpen <= 0 {
		c.FailuresToOpen = 1
	}
	if c.SuccessesToResolve <= 0 {
		c.SuccessesToResolve = 1
	}
	// Delegate defaults for plugin-registered check types.
	if exec, ok := LookupCheckExecutor(c.Type); ok {
		exec.ApplyDefaults(c)
	}
}

func (c *CheckConfig) validate(cfg *Config) error {
	if err := validateName(c.Name); err != nil {
		return err
	}

	// Validate per-check scheduling configuration
	if c.IntervalSeconds > 0 && c.IntervalSeconds < 10 {
		return fmt.Errorf("intervalSeconds must be >= 10 (got %d)", c.IntervalSeconds)
	}
	if c.RetryCount < 0 {
		return fmt.Errorf("retryCount must be >= 0 (got %d)", c.RetryCount)
	}
	if c.RetryCount > 0 && c.RetryDelaySeconds < 1 {
		return fmt.Errorf("retryDelaySeconds must be >= 1 when retryCount > 0 (got %d)", c.RetryDelaySeconds)
	}
	if c.CooldownSeconds < 0 {
		return fmt.Errorf("cooldownSeconds must be >= 0 (got %d)", c.CooldownSeconds)
	}

	exec, ok := LookupCheckExecutor(c.Type)
	if !ok {
		return fmt.Errorf("unsupported type %q", c.Type)
	}
	return exec.Validate(c, cfg)
}

func (c CheckConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

func buildCheckID(check *CheckConfig) string {
	base := strings.TrimSpace(check.ID)
	if base == "" {
		base = check.Name
	}
	if base == "" {
		base = check.Target
	}
	return slugify(base)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlug.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "check"
	}
	return value
}

func resolveTCPAddress(check CheckConfig) string {
	if check.Target != "" && strings.Contains(check.Target, ":") {
		return check.Target
	}
	host := check.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", host, check.Port)
}

func ensureConfigPath(path string) string {
	if path == "" {
		return filepath.Join("config", "default.json")
	}
	return path
}
