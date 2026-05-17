package monitoring

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
)

type pingCheckExecutor struct{}

func init() { RegisterCheckExecutor(&pingCheckExecutor{}) }

func (e *pingCheckExecutor) Type() string { return "ping" }

func (e *pingCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.Ping == nil {
		c.Ping = &PingCheckConfig{}
	}
	if c.Ping.Count <= 0 {
		c.Ping.Count = 3
	}
	if c.Ping.MaxPacketLossPercent <= 0 {
		c.Ping.MaxPacketLossPercent = 50
	}
}

func (e *pingCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	host := e.resolveHost(c)
	if host == "" {
		return fmt.Errorf("ping check requires host, target, or ping.host")
	}
	return nil
}

func (e *pingCheckExecutor) resolveHost(c *CheckConfig) string {
	if c.Ping != nil && c.Ping.Host != "" {
		return c.Ping.Host
	}
	if c.Host != "" {
		return c.Host
	}
	return c.Target
}

func (e *pingCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	cfg := check.Ping
	if cfg == nil {
		cfg = &PingCheckConfig{Count: 3, MaxPacketLossPercent: 50}
	}

	host := e.resolveHost(&check)
	count := cfg.Count
	if count <= 0 {
		count = 3
	}

	countStr := strconv.Itoa(count)

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, "ping", "-c", countStr, "-W", "3000", host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", countStr, "-W", "3", host)
	}

	output, err := cmd.CombinedOutput()
	out := string(output)

	// Parse packet loss
	lossPercent := 100.0
	lossRe := regexp.MustCompile(`([\d.]+)% packet loss`)
	if m := lossRe.FindStringSubmatch(out); len(m) > 1 {
		if v, e := strconv.ParseFloat(m[1], 64); e == nil {
			lossPercent = v
		}
	}

	// Parse average latency from "min/avg/max" line
	var avgLatency float64
	// macOS: "round-trip min/avg/max/stddev = 1.2/3.4/5.6/0.8 ms"
	// Linux: "rtt min/avg/max/mdev = 1.2/3.4/5.6/0.8 ms"
	avgRe := regexp.MustCompile(`= ([\d.]+)/([\d.]+)/([\d.]+)`)
	if m := avgRe.FindStringSubmatch(out); len(m) > 2 {
		if v, e := strconv.ParseFloat(m[2], 64); e == nil {
			avgLatency = v
		}
	}

	result.Metrics["packetLossPercent"] = lossPercent
	result.Metrics["avgLatencyMs"] = avgLatency
	result.Metrics["probeCount"] = float64(count)

	// If ping command failed and 100% loss
	if err != nil && lossPercent >= 100 {
		return fmt.Errorf("ping %s: 100%% packet loss", host)
	}

	maxLoss := cfg.MaxPacketLossPercent
	if maxLoss <= 0 {
		maxLoss = 50
	}

	if int(lossPercent) > maxLoss {
		return fmt.Errorf("ping %s: %.0f%% packet loss exceeds threshold %d%%", host, lossPercent, maxLoss)
	}

	// Check average latency warning
	if cfg.MaxAvgLatencyMs > 0 && avgLatency > float64(cfg.MaxAvgLatencyMs) {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("ping %s: avg latency %.1fms exceeds threshold %dms", host, avgLatency, cfg.MaxAvgLatencyMs)
		return nil
	}

	// Partial loss with high threshold = warning
	if lossPercent > 0 {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("ping %s: %.0f%% packet loss", host, lossPercent)
		return nil
	}

	return nil
}
