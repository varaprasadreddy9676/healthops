package monitoring

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type logCheckExecutor struct{}

func init() { RegisterCheckExecutor(&logCheckExecutor{}) }

func (e *logCheckExecutor) Type() string { return "log" }

func (e *logCheckExecutor) ApplyDefaults(c *CheckConfig) {}

func (e *logCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.Path == "" {
		return fmt.Errorf("path is required for log checks")
	}
	if c.FreshnessSeconds <= 0 {
		return fmt.Errorf("freshnessSeconds is required for log checks")
	}
	return nil
}

func (e *logCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	if srv := r.resolveServer(check.ServerId); srv != nil {
		timeout := time.Duration(check.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		cmd := fmt.Sprintf("stat -c %%Y %q 2>/dev/null || stat -f %%m %q 2>/dev/null", check.Path, check.Path)
		output, err := sshDialAndRun(srv.ToSSHConfig(), cmd, timeout)
		if err != nil {
			return fmt.Errorf("remote log check on %s: %w", srv.Host, err)
		}
		var mtime int64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &mtime); err != nil {
			return fmt.Errorf("failed to parse remote file mtime: %s", strings.TrimSpace(string(output)))
		}
		age := time.Since(time.Unix(mtime, 0))
		result.Metrics["ageSeconds"] = age.Seconds()
		if check.FreshnessSeconds > 0 && age > time.Duration(check.FreshnessSeconds)*time.Second {
			return fmt.Errorf("log heartbeat stale: last update %.0fs ago", age.Seconds())
		}
		return nil
	}

	info, err := os.Stat(check.Path)
	if err != nil {
		return err
	}
	age := time.Since(info.ModTime())
	result.Metrics["ageSeconds"] = age.Seconds()
	if check.FreshnessSeconds > 0 && age > time.Duration(check.FreshnessSeconds)*time.Second {
		return fmt.Errorf("log heartbeat stale: last update %.0fs ago", age.Seconds())
	}
	return nil
}
