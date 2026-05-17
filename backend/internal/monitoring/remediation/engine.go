package remediation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfig holds credentials needed to SSH into a remote server.
// Mirrors the shape of monitoring.SSHCheckConfig / monitoring.RemoteServer.
type SSHConfig struct {
	Host               string
	Port               int
	User               string
	KeyPath            string
	KeyEnv             string
	Password           string
	PasswordEnv        string
	HostKeyFingerprint string
}

// CheckInfo carries the data the engine needs to execute remediation for a check.
type CheckInfo struct {
	CheckID    string
	CheckName  string
	CheckType  string
	Target     string
	ServerID   string
	SSH        *SSHConfig
	Ref        RemediationRef
	IncidentID string
	// InlineAction is used when a check defines its remediation inline
	// (no registry lookup). If set, the engine uses this directly and skips
	// repo.GetAction(Ref.ActionRef).
	InlineAction *AllowedAction
}

// RunnerFunc re-runs a check to verify remediation worked.
type RunnerFunc func(ctx context.Context, checkID string) (healthy bool, message string, err error)

// AuditFunc logs a remediation audit event.
type AuditFunc func(action, actor, target, targetID string, details map[string]interface{})

// RemediationSuccessCallback is called after a verified successful remediation.
// Parameters: checkID, incidentID, actionName, attemptID.
type RemediationSuccessCallback func(checkID, incidentID, actionName, attemptID string)

// Engine is the deterministic remediation engine. It is separate from the
// existing AI-assisted automation (suggestions + human approval). This engine
// executes pre-approved, allowlisted actions automatically.
type Engine struct {
	repo          Repository
	logger        *log.Logger
	aiCall        AIProvider                 // optional — used for failed-attempt analysis
	runCheck      RunnerFunc                 // optional — verify after remediation
	auditFn       AuditFunc                  // optional — audit log
	checkResolver CheckResolverFunc          // optional — resolves check ID to CheckInfo
	onSuccess     RemediationSuccessCallback // optional — fires on verified recovery
	running       int32                      // current concurrent executions (atomic)
	mu            sync.Mutex                 // guards per-check cooldown state
	cooldowns     map[string]time.Time       // checkID → earliest next attempt
}

// NewEngine creates a remediation engine.
func NewEngine(repo Repository, logger *log.Logger) *Engine {
	if logger == nil {
		logger = log.Default()
	}
	return &Engine{
		repo:      repo,
		logger:    logger,
		cooldowns: make(map[string]time.Time),
	}
}

func (e *Engine) SetAIProvider(ai AIProvider)                { e.aiCall = ai }
func (e *Engine) SetRunnerFunc(fn RunnerFunc)                { e.runCheck = fn }
func (e *Engine) SetAuditFunc(fn AuditFunc)                  { e.auditFn = fn }
func (e *Engine) SetCheckResolver(fn CheckResolverFunc)      { e.checkResolver = fn }
func (e *Engine) SetOnSuccess(fn RemediationSuccessCallback) { e.onSuccess = fn }

// TryRemediate is called when an incident is created or when a check continues
// to fail. It checks all prerequisites and executes the action if appropriate.
func (e *Engine) TryRemediate(info CheckInfo) {
	go e.tryRemediateAsync(info)
}

func (e *Engine) tryRemediateAsync(info CheckInfo) {
	cfg, err := e.repo.GetConfig()
	if err != nil {
		e.logger.Printf("[remediation] failed to load config: %v", err)
		return
	}
	if !cfg.Enabled {
		return
	}

	ref := info.Ref
	ref.Defaults()

	// Resolve the action: prefer inline, fall back to registry lookup
	var action AllowedAction
	if info.InlineAction != nil {
		action = *info.InlineAction
	} else {
		a, err := e.repo.GetAction(ref.ActionRef)
		if err != nil {
			e.logger.Printf("[remediation] action %q not found for check %s: %v", ref.ActionRef, info.CheckID, err)
			return
		}
		action = a
	}

	// Check max concurrent
	current := atomic.LoadInt32(&e.running)
	maxConc := int32(cfg.MaxConcurrent)
	if maxConc <= 0 {
		maxConc = 2
	}
	if current >= maxConc {
		e.logger.Printf("[remediation] skipping %s: max concurrent (%d) reached", info.CheckID, maxConc)
		return
	}

	// Check cooldown
	e.mu.Lock()
	if earliest, ok := e.cooldowns[info.CheckID]; ok && time.Now().Before(earliest) {
		e.mu.Unlock()
		e.logger.Printf("[remediation] skipping %s: cooldown active until %s", info.CheckID, earliest.Format(time.RFC3339))
		return
	}
	e.mu.Unlock()

	// Check max attempts for this incident
	attemptCount, err := e.repo.CountAttempts("", info.IncidentID)
	if err != nil {
		e.logger.Printf("[remediation] failed to count attempts: %v", err)
		return
	}
	if attemptCount >= ref.MaxAttempts {
		e.logger.Printf("[remediation] skipping %s: max attempts (%d) reached for incident %s", info.CheckID, ref.MaxAttempts, info.IncidentID)
		if ref.EscalateOnExhaustion {
			e.logEscalation(info, attemptCount)
		}
		return
	}

	// Execute
	e.execute(info, action, cfg, attemptCount+1)
}

// ManualRemediate triggers a remediation manually (by an operator).
func (e *Engine) ManualRemediate(info CheckInfo, actor string) (*Attempt, error) {
	cfg, err := e.repo.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("remediation is disabled globally")
	}

	ref := info.Ref
	ref.Defaults()

	var action AllowedAction
	if info.InlineAction != nil {
		action = *info.InlineAction
	} else {
		a, err := e.repo.GetAction(ref.ActionRef)
		if err != nil {
			return nil, fmt.Errorf("action %q not found: %w", ref.ActionRef, err)
		}
		action = a
	}

	// Check cooldown
	e.mu.Lock()
	if earliest, ok := e.cooldowns[info.CheckID]; ok && time.Now().Before(earliest) {
		e.mu.Unlock()
		return nil, fmt.Errorf("cooldown active until %s", earliest.Format(time.RFC3339))
	}
	e.mu.Unlock()

	// Check max attempts
	attemptCount, err := e.repo.CountAttempts("", info.IncidentID)
	if err != nil {
		return nil, fmt.Errorf("count attempts: %w", err)
	}
	if attemptCount >= ref.MaxAttempts {
		return nil, fmt.Errorf("max attempts (%d) reached for incident %s", ref.MaxAttempts, info.IncidentID)
	}

	attempt := e.execute(info, action, cfg, attemptCount+1)
	if attempt != nil {
		attempt.TriggeredBy = actor
		_ = e.repo.UpdateAttempt(attempt.ID, func(a *Attempt) {
			a.TriggeredBy = actor
		})
	}
	return attempt, nil
}

func (e *Engine) execute(info CheckInfo, action AllowedAction, cfg GlobalConfig, attemptNum int) *Attempt {
	atomic.AddInt32(&e.running, 1)
	defer atomic.AddInt32(&e.running, -1)

	attempt := Attempt{
		ID:            generateAttemptID(),
		CheckID:       info.CheckID,
		IncidentID:    info.IncidentID,
		ActionID:      action.ID,
		ActionName:    action.Name,
		ActionType:    action.Type,
		Command:       action.Command,
		AttemptNumber: attemptNum,
		DryRun:        cfg.DryRun,
		TriggeredBy:   "system",
		CreatedAt:     time.Now().UTC(),
	}

	// Persist the attempt as "running"
	attempt.Status = AttemptRunning
	if cfg.DryRun {
		attempt.Status = AttemptDryRun
	}
	if err := e.repo.CreateAttempt(attempt); err != nil {
		e.logger.Printf("[remediation] failed to persist attempt: %v", err)
		return nil
	}

	e.logger.Printf("[remediation] %s attempt #%d for check %s (action: %s, dry-run: %v)",
		action.Type, attemptNum, info.CheckID, action.ID, cfg.DryRun)

	if cfg.DryRun {
		attempt.Output = fmt.Sprintf("[DRY RUN] would execute: %s %s", action.Type, action.Command)
		attempt.Status = AttemptDryRun
		attempt.ExitCode = 0
		e.updateAttempt(attempt)
		e.audit("remediation.dry_run", info, &attempt)
		return &attempt
	}

	// Execute the action
	start := time.Now()
	timeout := time.Duration(action.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var output string
	var exitCode int
	var execErr error

	switch action.Type {
	case ActionCommand:
		output, exitCode, execErr = e.execLocalCommand(action.Command, timeout)
	case ActionSSHCommand:
		output, exitCode, execErr = e.execSSHCommand(info.SSH, action.Command, timeout)
	case ActionHTTP:
		output, exitCode, execErr = e.execHTTPAction(action, timeout)
	default:
		execErr = fmt.Errorf("unsupported action type: %s", action.Type)
	}

	attempt.DurationMs = time.Since(start).Milliseconds()

	// Cap output
	limit := cfg.OutputLimitBytes
	if limit <= 0 {
		limit = 8192
	}
	output = redactAndCap(output, limit)

	if execErr != nil {
		attempt.Status = AttemptFailed
		attempt.Error = execErr.Error()
		attempt.Output = output
		attempt.ExitCode = exitCode
		e.updateAttempt(attempt)
		e.audit("remediation.failed", info, &attempt)

		// AI analysis on failure
		e.analyzeFailure(info, &attempt)

		e.setCooldown(info.CheckID, info.Ref.CooldownSeconds)
		return &attempt
	}

	attempt.Output = output
	attempt.ExitCode = exitCode
	if exitCode != 0 {
		attempt.Status = AttemptFailed
		e.updateAttempt(attempt)
		e.audit("remediation.failed", info, &attempt)
		e.analyzeFailure(info, &attempt)
		e.setCooldown(info.CheckID, info.Ref.CooldownSeconds)
		return &attempt
	}

	attempt.Status = AttemptSuccess
	e.updateAttempt(attempt)
	e.audit("remediation.executed", info, &attempt)

	// Set cooldown
	e.setCooldown(info.CheckID, info.Ref.CooldownSeconds)

	// Verify after delay
	ref := info.Ref
	ref.Defaults()
	if e.runCheck != nil && ref.VerifyAfterSeconds > 0 {
		go e.verifyAfter(info, &attempt, ref.VerifyAfterSeconds)
	}

	return &attempt
}

func (e *Engine) verifyAfter(info CheckInfo, attempt *Attempt, delaySec int) {
	time.Sleep(time.Duration(delaySec) * time.Second)

	healthy, msg, err := e.runCheck(context.Background(), info.CheckID)
	if err != nil {
		e.logger.Printf("[remediation] verify failed for %s: %v", info.CheckID, err)
		return
	}

	verified := healthy
	_ = e.repo.UpdateAttempt(attempt.ID, func(a *Attempt) {
		a.Verified = &verified
		if !verified {
			a.Error = fmt.Sprintf("verification failed: %s", msg)
		}
	})

	if verified {
		e.logger.Printf("[remediation] verified: check %s is healthy after remediation", info.CheckID)
		e.audit("remediation.verified", info, attempt)
		if e.onSuccess != nil && info.IncidentID != "" {
			e.onSuccess(info.CheckID, info.IncidentID, attempt.ActionName, attempt.ID)
		}
	} else {
		e.logger.Printf("[remediation] verification failed: check %s still unhealthy: %s", info.CheckID, msg)
		e.audit("remediation.verify_failed", info, attempt)
		// Analyze why it's still failing
		attempt.Error = fmt.Sprintf("Remediation ran successfully (exit 0) but check still failing: %s", msg)
		e.analyzeFailure(info, attempt)
	}
}

// analyzeFailure sends remediation output + context to AI for diagnosis.
func (e *Engine) analyzeFailure(info CheckInfo, attempt *Attempt) {
	if e.aiCall == nil {
		return
	}

	systemMsg := `You are an infrastructure reliability engineer analyzing a failed remediation attempt. 
Given the remediation details and output, explain:
1. What went wrong (be specific)
2. The likely root cause
3. What command or action to try next
Keep your response concise (3-5 sentences). Be specific to the actual error output.`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Check: %s (type: %s, target: %s)\n", info.CheckName, info.CheckType, info.Target))
	sb.WriteString(fmt.Sprintf("Remediation action: %s (type: %s)\n", attempt.ActionName, attempt.ActionType))
	sb.WriteString(fmt.Sprintf("Command: %s\n", attempt.Command))
	sb.WriteString(fmt.Sprintf("Exit code: %d\n", attempt.ExitCode))
	if attempt.Error != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", attempt.Error))
	}
	if attempt.Output != "" {
		sb.WriteString(fmt.Sprintf("Output:\n%s\n", attempt.Output))
	}
	if info.SSH != nil {
		sb.WriteString(fmt.Sprintf("Server: %s (user: %s)\n", info.SSH.Host, info.SSH.User))
	}

	analysis, err := e.aiCall(systemMsg, sb.String())
	if err != nil {
		e.logger.Printf("[remediation] AI analysis failed: %v", err)
		return
	}

	_ = e.repo.UpdateAttempt(attempt.ID, func(a *Attempt) {
		a.AIAnalysis = strings.TrimSpace(analysis)
	})
	e.logger.Printf("[remediation] AI analysis saved for attempt %s", attempt.ID)
}

// SuggestCommand asks AI to suggest a remediation command for a check.
func (e *Engine) SuggestCommand(req SuggestCommandRequest) (string, error) {
	if e.aiCall == nil {
		return "", fmt.Errorf("AI provider not configured")
	}

	systemMsg := `You are an infrastructure engineer. Given the check details, suggest the most appropriate remediation command.
Return ONLY the command, no explanation. The command should be safe, specific, and executable on a Linux server.
Examples:
- For a process check targeting "nginx": sudo systemctl restart nginx
- For a MySQL check: sudo systemctl restart mysql
- For a Docker container API check: docker restart <container>
- For a disk full scenario: sudo find /tmp -type f -mtime +7 -delete`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Check type: %s\n", req.CheckType))
	sb.WriteString(fmt.Sprintf("Check target: %s\n", req.CheckTarget))
	if req.ServerHost != "" {
		sb.WriteString(fmt.Sprintf("Server: %s\n", req.ServerHost))
	}
	if req.FailMessage != "" {
		sb.WriteString(fmt.Sprintf("Failure message: %s\n", req.FailMessage))
	}

	result, err := e.aiCall(systemMsg, sb.String())
	if err != nil {
		return "", fmt.Errorf("AI call failed: %w", err)
	}
	return strings.TrimSpace(result), nil
}

// ---------- Command execution ----------

func (e *Engine) execLocalCommand(command string, timeout time.Duration) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[stderr]\n" + stderr.String()
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return output, -1, fmt.Errorf("command timed out after %s", timeout)
		} else {
			return output, -1, fmt.Errorf("exec failed: %w", err)
		}
	}
	return output, exitCode, nil
}

func (e *Engine) execSSHCommand(sshCfg *SSHConfig, command string, timeout time.Duration) (string, int, error) {
	if sshCfg == nil {
		return "", -1, fmt.Errorf("SSH config not available for this check — cannot execute ssh_command")
	}

	authMethods, err := buildSSHAuth(sshCfg)
	if err != nil {
		return "", -1, fmt.Errorf("ssh auth: %w", err)
	}

	port := sshCfg.Port
	if port <= 0 {
		port = 22
	}

	clientCfg := &ssh.ClientConfig{
		User:            sshCfg.User,
		Auth:            authMethods,
		HostKeyCallback: makeHostKeyCallback(sshCfg),
		Timeout:         timeout,
	}

	addr := net.JoinHostPort(sshCfg.Host, strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return "", -1, fmt.Errorf("ssh connect %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", -1, fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[stderr]\n" + stderr.String()
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return output, -1, fmt.Errorf("ssh command failed: %w", err)
		}
	}
	return output, exitCode, nil
}

func (e *Engine) execHTTPAction(action AllowedAction, timeout time.Duration) (string, int, error) {
	if action.URL == "" {
		return "", -1, fmt.Errorf("HTTP action URL is required")
	}

	method := action.Method
	if method == "" {
		method = "POST"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, action.URL, nil)
	if err != nil {
		return "", -1, fmt.Errorf("create request: %w", err)
	}
	for k, v := range action.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", -1, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	output := fmt.Sprintf("HTTP %d %s\n%s", resp.StatusCode, resp.Status, string(body))

	exitCode := 0
	if resp.StatusCode >= 400 {
		exitCode = 1
	}
	return output, exitCode, nil
}

// ---------- SSH auth helpers ----------

func buildSSHAuth(cfg *SSHConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try key auth first
	keyPath := cfg.KeyPath
	if keyPath == "" && cfg.KeyEnv != "" {
		keyPath = os.Getenv(cfg.KeyEnv)
	}
	if keyPath != "" {
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read ssh key %s: %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// Try password auth
	password := cfg.Password
	if password == "" && cfg.PasswordEnv != "" {
		password = os.Getenv(cfg.PasswordEnv)
	}
	if password != "" {
		methods = append(methods, ssh.Password(password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH auth method configured (need keyPath/keyEnv or password/passwordEnv)")
	}
	return methods, nil
}

func makeHostKeyCallback(cfg *SSHConfig) ssh.HostKeyCallback {
	if cfg.HostKeyFingerprint == "" {
		return ssh.InsecureIgnoreHostKey() //nolint:gosec
	}
	expected := cfg.HostKeyFingerprint
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		got := ssh.FingerprintSHA256(key)
		if got != expected {
			return fmt.Errorf("ssh host key mismatch for %s: got %s, want %s", hostname, got, expected)
		}
		return nil
	}
}

// ---------- Helpers ----------

func (e *Engine) setCooldown(checkID string, seconds int) {
	if seconds <= 0 {
		seconds = 300
	}
	e.mu.Lock()
	e.cooldowns[checkID] = time.Now().Add(time.Duration(seconds) * time.Second)
	e.mu.Unlock()
}

func (e *Engine) updateAttempt(attempt Attempt) {
	_ = e.repo.UpdateAttempt(attempt.ID, func(a *Attempt) {
		a.Status = attempt.Status
		a.Output = attempt.Output
		a.ExitCode = attempt.ExitCode
		a.Error = attempt.Error
		a.DurationMs = attempt.DurationMs
		a.DryRun = attempt.DryRun
	})
}

func (e *Engine) logEscalation(info CheckInfo, attempts int) {
	e.logger.Printf("[remediation] ESCALATION: all %d attempts exhausted for check %s (incident %s)",
		attempts, info.CheckID, info.IncidentID)
	e.audit("remediation.escalated", info, nil)
}

func (e *Engine) audit(action string, info CheckInfo, attempt *Attempt) {
	if e.auditFn == nil {
		return
	}
	details := map[string]interface{}{
		"checkId":    info.CheckID,
		"checkName":  info.CheckName,
		"incidentId": info.IncidentID,
	}
	if attempt != nil {
		details["attemptId"] = attempt.ID
		details["actionId"] = attempt.ActionID
		details["exitCode"] = attempt.ExitCode
		details["dryRun"] = attempt.DryRun
		if attempt.Error != "" {
			details["error"] = attempt.Error
		}
	}
	e.auditFn(action, "system", "check", info.CheckID, details)
}

func redactAndCap(s string, limit int) string {
	// Redact common secret patterns
	for _, pattern := range []string{"password=", "passwd=", "secret=", "token=", "api_key=", "apikey="} {
		idx := strings.Index(strings.ToLower(s), pattern)
		for idx >= 0 {
			end := idx + len(pattern)
			// Find end of value (next space, newline, or &)
			valueEnd := end
			for valueEnd < len(s) && s[valueEnd] != ' ' && s[valueEnd] != '\n' && s[valueEnd] != '&' {
				valueEnd++
			}
			s = s[:end] + "***REDACTED***" + s[valueEnd:]
			idx = strings.Index(strings.ToLower(s[end:]), pattern)
			if idx >= 0 {
				idx += end
			}
		}
	}

	if len(s) > limit {
		s = s[:limit] + "\n... [output truncated at " + strconv.Itoa(limit) + " bytes]"
	}
	return s
}

func generateAttemptID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("rem_%d", time.Now().UnixNano())
	}
	return "rem_" + hex.EncodeToString(b)
}
