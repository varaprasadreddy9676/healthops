package monitoring

import (
	"context"
	"fmt"
)

type sshCheckExecutor struct{}

func init() { RegisterCheckExecutor(&sshCheckExecutor{}) }

func (e *sshCheckExecutor) Type() string { return "ssh" }

func (e *sshCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.SSH != nil && c.SSH.Port <= 0 {
		c.SSH.Port = 22
	}
}

func (e *sshCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.SSH == nil {
		return fmt.Errorf("ssh config block is required for ssh checks")
	}
	if c.SSH.Host == "" {
		return fmt.Errorf("ssh.host is required for ssh checks")
	}
	if c.SSH.User == "" {
		return fmt.Errorf("ssh.user is required for ssh checks")
	}
	hasKey := c.SSH.KeyPath != "" || c.SSH.KeyEnv != ""
	hasPassword := c.SSH.Password != "" || c.SSH.PasswordEnc != "" || c.SSH.PasswordEnv != ""
	if !hasKey && !hasPassword {
		return fmt.Errorf("ssh auth required: set keyPath/keyEnv or password/passwordEnv")
	}
	return nil
}

func (e *sshCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	return r.runSSH(ctx, check, result)
}
