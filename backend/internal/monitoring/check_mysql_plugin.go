package monitoring

import (
	"context"
	"fmt"
)

type mysqlCheckExecutor struct{}

func init() { RegisterCheckExecutor(&mysqlCheckExecutor{}) }

func (e *mysqlCheckExecutor) Type() string { return "mysql" }

func (e *mysqlCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.MySQL == nil {
		return
	}
	if c.MySQL.Port <= 0 {
		c.MySQL.Port = 3306
	}
	if c.MySQL.ConnectTimeoutSeconds <= 0 {
		c.MySQL.ConnectTimeoutSeconds = 3
	}
	if c.MySQL.QueryTimeoutSeconds <= 0 {
		c.MySQL.QueryTimeoutSeconds = 5
	}
	if c.MySQL.ProcesslistLimit <= 0 {
		c.MySQL.ProcesslistLimit = 50
	}
	if c.MySQL.StatementLimit <= 0 {
		c.MySQL.StatementLimit = 20
	}
	if c.MySQL.HostUserLimit <= 0 {
		c.MySQL.HostUserLimit = 20
	}
	if c.IntervalSeconds <= 0 {
		c.IntervalSeconds = 15
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 10
	}
}

func (e *mysqlCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.MySQL == nil {
		return fmt.Errorf("mysql config block is required for mysql checks")
	}
	hasDirect := c.MySQL.Host != "" && c.MySQL.Username != ""
	hasEnv := c.MySQL.DSNEnv != ""
	if !hasDirect && !hasEnv {
		return fmt.Errorf("mysql config requires either host+username or dsnEnv")
	}
	return nil
}

func (e *mysqlCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	return r.runMySQL(ctx, check, result)
}
