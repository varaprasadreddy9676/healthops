package monitoring

import (
	"context"
	"fmt"
	"net"
	"time"
)

type tcpCheckExecutor struct{}

func init() { RegisterCheckExecutor(&tcpCheckExecutor{}) }

func (e *tcpCheckExecutor) Type() string { return "tcp" }

func (e *tcpCheckExecutor) ApplyDefaults(c *CheckConfig) {}

func (e *tcpCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.Port <= 0 {
		return fmt.Errorf("port is required for tcp checks")
	}
	if c.Host == "" && c.Target == "" {
		return fmt.Errorf("host or target is required for tcp checks")
	}
	return nil
}

func (e *tcpCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	addr := resolveTCPAddress(check)
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	_ = conn.Close()
	result.Metrics["latencyMs"] = float64(time.Since(start).Milliseconds())
	result.Metrics["port"] = float64(check.Port)
	if check.WarningThresholdMs > 0 && result.Metrics["latencyMs"] > float64(check.WarningThresholdMs) {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("slow tcp connect: %.0fms", result.Metrics["latencyMs"])
	}
	return nil
}
