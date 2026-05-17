package monitoring

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type apiCheckExecutor struct{}

func init() { RegisterCheckExecutor(&apiCheckExecutor{}) }

func (e *apiCheckExecutor) Type() string { return "api" }

func (e *apiCheckExecutor) ApplyDefaults(c *CheckConfig) {
	// api defaults are already applied in CheckConfig.applyDefaults()
}

func (e *apiCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.Target == "" {
		return fmt.Errorf("target is required for api checks")
	}
	return nil
}

func (e *apiCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.Target, nil)
	if err != nil {
		return err
	}

	start := time.Now()
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	result.Metrics["statusCode"] = float64(resp.StatusCode)
	result.Metrics["bodyBytes"] = float64(len(body))
	result.Metrics["latencyMs"] = float64(time.Since(start).Milliseconds())

	if resp.StatusCode != check.ExpectedStatus {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	if check.ExpectedContains != "" && !bytes.Contains(body, []byte(check.ExpectedContains)) {
		return fmt.Errorf("expected response to contain %q", check.ExpectedContains)
	}
	if check.WarningThresholdMs > 0 && result.Metrics["latencyMs"] > float64(check.WarningThresholdMs) {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("slow api response: %.0fms", result.Metrics["latencyMs"])
	}
	return nil
}
