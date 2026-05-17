package monitoring

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type commandCheckExecutor struct{}

func init() { RegisterCheckExecutor(&commandCheckExecutor{}) }

func (e *commandCheckExecutor) Type() string { return "command" }

func (e *commandCheckExecutor) ApplyDefaults(c *CheckConfig) {}

// Validate checks that the command is set and that command checks are enabled.
// SECURITY: Command checks execute arbitrary shell commands. They must be
// explicitly enabled via Config.AllowCommandChecks. Config files must be
// tightly controlled; never pass user input directly into check.Command.
func (e *commandCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.Command == "" {
		return fmt.Errorf("command is required for command checks")
	}
	if !cfg.AllowCommandChecks {
		return fmt.Errorf("command checks are disabled for security; set allowCommandChecks=true to enable (use with caution)")
	}
	return nil
}

// Execute runs the shell command.
// SECURITY: This function executes arbitrary shell commands from the config.
// The command is executed via 'sh -c' which allows shell operators.
// NEVER pass user input directly into check.Command.
func (e *commandCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	var output []byte
	var err error

	if srv := r.resolveServer(check.ServerId); srv != nil {
		timeout := time.Duration(check.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		output, err = sshDialAndRun(srv.ToSSHConfig(), check.Command, timeout)
		if err != nil {
			return fmt.Errorf("remote command on %s failed: %w: %s", srv.Host, err, strings.TrimSpace(string(output)))
		}
	} else {
		cmd := exec.CommandContext(ctx, "sh", "-c", check.Command)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}

	result.Metrics["outputBytes"] = float64(len(output))
	if check.ExpectedContains != "" && !bytes.Contains(output, []byte(check.ExpectedContains)) {
		return fmt.Errorf("expected command output to contain %q", check.ExpectedContains)
	}
	return nil
}
