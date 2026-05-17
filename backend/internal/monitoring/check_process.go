package monitoring

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type processCheckExecutor struct{}

func init() { RegisterCheckExecutor(&processCheckExecutor{}) }

func (e *processCheckExecutor) Type() string { return "process" }

func (e *processCheckExecutor) ApplyDefaults(c *CheckConfig) {}

func (e *processCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.Target == "" {
		return fmt.Errorf("target is required for process checks")
	}
	return nil
}

func (e *processCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	var output []byte
	var err error

	if srv := r.resolveServer(check.ServerId); srv != nil {
		timeout := time.Duration(check.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		output, err = sshDialAndRun(srv.ToSSHConfig(), "ps -eo pid=,args=", timeout)
		if err != nil {
			return fmt.Errorf("remote process check on %s: %w", srv.Host, err)
		}
	} else {
		binary, args := processListCommand()
		cmd := exec.CommandContext(ctx, binary, args...)
		output, err = cmd.Output()
		if err != nil {
			return err
		}
	}

	matched := 0
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, check.Target) {
			matched++
		}
	}

	result.Metrics["matchedProcesses"] = float64(matched)
	if matched == 0 {
		return fmt.Errorf("process %q not found", check.Target)
	}
	return nil
}
