package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type fakeStore struct {
	snapshot     State
	updateCalled atomic.Int32
	updateFunc   func(*State) error
}

func (f *fakeStore) Snapshot() State {
	return cloneState(f.snapshot)
}

func (f *fakeStore) DashboardSnapshot() DashboardSnapshot {
	return buildDashboardSnapshot(f.snapshot)
}

func (f *fakeStore) Update(fn func(*State) error) error {
	f.updateCalled.Add(1)
	if f.updateFunc != nil {
		return f.updateFunc(&f.snapshot)
	}
	return fn(&f.snapshot)
}

func (f *fakeStore) ReplaceChecks([]CheckConfig) error      { return nil }
func (f *fakeStore) UpsertCheck(CheckConfig) error          { return nil }
func (f *fakeStore) DeleteCheck(string) error               { return nil }
func (f *fakeStore) AppendResults([]CheckResult, int) error { return nil }
func (f *fakeStore) SetLastRun(time.Time) error             { return nil }

func TestRunnerRunOnceWithNoEnabledChecks(t *testing.T) {
	store := &fakeStore{
		snapshot: State{
			Checks: []CheckConfig{
				{ID: "c1", Name: "C1", Type: "api", Target: "https://example.com", Enabled: boolPtr(false)},
			},
		},
	}
	cfg := &Config{Workers: 2, RetentionDays: 7}
	runner := NewRunner(cfg, store)

	ctx := context.Background()
	summary, err := runner.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if summary.Skipped {
		t.Fatal("expected run to execute, not skip")
	}
	if len(summary.Results) != 0 {
		t.Errorf("expected no results, got %d", len(summary.Results))
	}
	// Allow FinishedAt to be >= StartedAt (they could be equal for very fast operations)
	if summary.FinishedAt.Before(summary.StartedAt) {
		t.Error("FinishedAt should not be before StartedAt")
	}
}

func TestRunnerRunOnceSkipsWhenAlreadyRunning(t *testing.T) {
	// Use an actual API check that will take time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := &fakeStore{
		snapshot: State{
			Checks: []CheckConfig{
				{ID: "c1", Name: "C1", Type: "api", Target: server.URL, ExpectedStatus: 200, Enabled: boolPtr(true)},
			},
		},
		updateFunc: func(s *State) error {
			return nil
		},
	}
	cfg := &Config{Workers: 2, RetentionDays: 7}
	runner := NewRunner(cfg, store)

	// Start a run that will take time
	runComplete := make(chan struct{})
	go func() {
		runner.RunOnce(context.Background())
		close(runComplete)
	}()

	// Give the first run time to acquire the lock and start working
	time.Sleep(20 * time.Millisecond)

	// Try to run again - should skip
	summary, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !summary.Skipped {
		t.Error("expected run to be skipped when already running")
	}

	// Wait for the first run to complete
	<-runComplete
}

func TestRunnerRunOnceConcurrentExecution(t *testing.T) {
	// Create a test server that tracks concurrent requests
	var concurrentCalls atomic.Int32
	var maxConcurrent atomic.Int32
	var running atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := running.Add(1)
		defer running.Add(-1)

		// Update max if needed
		for {
			max := maxConcurrent.Load()
			if current <= max || maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}

		concurrentCalls.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := &fakeStore{
		snapshot: State{
			Checks: []CheckConfig{
				{ID: "c1", Name: "C1", Type: "api", Target: server.URL, ExpectedStatus: 200, Enabled: boolPtr(true)},
				{ID: "c2", Name: "C2", Type: "api", Target: server.URL, ExpectedStatus: 200, Enabled: boolPtr(true)},
				{ID: "c3", Name: "C3", Type: "api", Target: server.URL, ExpectedStatus: 200, Enabled: boolPtr(true)},
				{ID: "c4", Name: "C4", Type: "api", Target: server.URL, ExpectedStatus: 200, Enabled: boolPtr(true)},
			},
		},
		updateFunc: func(state *State) error {
			state.Results = nil // Don't accumulate in test
			state.LastRunAt = time.Now().UTC()
			return nil
		},
	}

	cfg := &Config{Workers: 4, RetentionDays: 7}
	runner := NewRunner(cfg, store)

	ctx := context.Background()
	summary, err := runner.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(summary.Results) != 4 {
		t.Errorf("expected 4 results, got %d", len(summary.Results))
	}

	max := maxConcurrent.Load()
	if max < 2 {
		t.Errorf("expected concurrent execution (max >= 2), got %d", max)
	}

	for _, result := range summary.Results {
		if result.Status != "healthy" {
			t.Errorf("check %s: expected healthy, got %s: %s", result.CheckID, result.Status, result.Message)
		}
	}
}

func TestRunnerAPICheckSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-test",
		Name:           "API Test",
		Type:           "api",
		Target:         server.URL,
		ExpectedStatus: 200,
		TimeoutSeconds: 5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s: %s", result.Status, result.Message)
	}
	if result.Healthy != true {
		t.Error("expected Healthy=true")
	}
	if result.DurationMs < 0 {
		t.Errorf("expected non-negative duration, got %d", result.DurationMs)
	}
	if _, ok := result.Metrics["latencyMs"]; !ok {
		t.Errorf("expected latencyMs metric, got %v", result.Metrics)
	}
	if statusCode, ok := result.Metrics["statusCode"]; !ok || statusCode != 200 {
		t.Errorf("expected statusCode=200, got %v", statusCode)
	}
}

func TestRunnerAPICheckWrongStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-test",
		Name:           "API Test",
		Type:           "api",
		Target:         server.URL,
		ExpectedStatus: 200,
		TimeoutSeconds: 5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical, got %s", result.Status)
	}
	if result.Healthy != false {
		t.Error("expected Healthy=false")
	}
	if result.Message == "" {
		t.Error("expected error message")
	}
}

func TestRunnerAPICheckBodyContains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("System operational"))
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "api-test",
		Name:             "API Test",
		Type:             "api",
		Target:           server.URL,
		ExpectedStatus:   200,
		ExpectedContains: "operational",
		TimeoutSeconds:   5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerAPICheckBodyNotContains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("System operational"))
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "api-test",
		Name:             "API Test",
		Type:             "api",
		Target:           server.URL,
		ExpectedStatus:   200,
		ExpectedContains: "ERROR",
		TimeoutSeconds:   5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerAPICheckSlowResponseWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:                 "api-test",
		Name:               "API Test",
		Type:               "api",
		Target:             server.URL,
		ExpectedStatus:     200,
		WarningThresholdMs: 100,
		TimeoutSeconds:     5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "warning" {
		t.Errorf("expected warning for slow response, got %s: %s", result.Status, result.Message)
	}
	if result.Healthy != false {
		t.Error("expected Healthy=false for warning")
	}
}

func TestRunnerAPICheckTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-test",
		Name:           "API Test",
		Type:           "api",
		Target:         server.URL,
		ExpectedStatus: 200,
		TimeoutSeconds: 1,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical on timeout, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerTCPCheckSuccess(t *testing.T) {
	// Use the test server address
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract host:port
	addr := server.Listener.Addr().String()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "tcp-test",
		Name:           "TCP Test",
		Type:           "tcp",
		Host:           "127.0.0.1",
		Port:           0, // Will be set below
		TimeoutSeconds: 5,
	}

	// Parse the address to get port
	check.Target = addr

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s: %s", result.Status, result.Message)
	}
	if latency, ok := result.Metrics["latencyMs"]; !ok || latency < 0 {
		t.Errorf("expected latencyMs metric, got %v", result.Metrics)
	}
}

func TestRunnerTCPCheckFailure(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "tcp-test",
		Name:           "TCP Test",
		Type:           "tcp",
		Host:           "localhost",
		Port:           9999, // Hopefully nothing listening here
		TimeoutSeconds: 1,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical for connection failure, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerProcessCheckSuccess(t *testing.T) {
	// The test itself is a Go process, which should match "go" or "test"
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:     "proc-test",
		Name:   "Process Test",
		Type:   "process",
		Target: "go", // Should find the running `go test` process
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s: %s", result.Status, result.Message)
	}
	if matched, ok := result.Metrics["matchedProcesses"]; !ok || matched <= 0 {
		t.Errorf("expected matchedProcesses > 0, got %v", result.Metrics)
	}
}

func TestRunnerProcessCheckNotFound(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:     "proc-test",
		Name:   "Process Test",
		Type:   "process",
		Target: "definitely-not-a-real-process-name-xyz123",
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerCommandCheckSuccess(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:      "cmd-test",
		Name:    "Command Test",
		Type:    "command",
		Command: "echo 'success'",
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerCommandCheckFailure(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:      "cmd-test",
		Name:    "Command Test",
		Type:    "command",
		Command: "exit 1",
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerCommandCheckExpectedOutput(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "cmd-test",
		Name:             "Command Test",
		Type:             "command",
		Command:          "echo 'system is operational'",
		ExpectedContains: "operational",
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerCommandCheckMissingExpectedOutput(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "cmd-test",
		Name:             "Command Test",
		Type:             "command",
		Command:          "echo 'system is operational'",
		ExpectedContains: "ERROR",
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerLogCheckFresh(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create a fresh log file
	if err := os.WriteFile(logPath, []byte("log entry\n"), 0644); err != nil {
		t.Fatalf("create log: %v", err)
	}

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "log-test",
		Name:             "Log Test",
		Type:             "log",
		Path:             logPath,
		FreshnessSeconds: 300, // 5 minutes
		TimeoutSeconds:   5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "healthy" {
		t.Errorf("expected healthy for fresh log, got %s: %s", result.Status, result.Message)
	}
	if age, ok := result.Metrics["ageSeconds"]; !ok || age < 0 {
		t.Errorf("expected ageSeconds >= 0, got %v", result.Metrics)
	}
}

func TestRunnerLogCheckStale(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create an old log file
	oldTime := time.Now().Add(-1 * time.Hour)
	if err := os.WriteFile(logPath, []byte("old log entry\n"), 0644); err != nil {
		t.Fatalf("create log: %v", err)
	}
	if err := os.Chtimes(logPath, oldTime, oldTime); err != nil {
		t.Fatalf("set mod time: %v", err)
	}

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "log-test",
		Name:             "Log Test",
		Type:             "log",
		Path:             logPath,
		FreshnessSeconds: 300, // 5 minutes
		TimeoutSeconds:   5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical for stale log, got %s: %s", result.Status, result.Message)
	}
	if result.Message == "" {
		t.Error("expected error message about stale log")
	}
}

func TestRunnerLogCheckMissing(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:               "log-test",
		Name:             "Log Test",
		Type:             "log",
		Path:             "/nonexistent/path/to/log/file.log",
		FreshnessSeconds: 300,
		TimeoutSeconds:   5,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical for missing log, got %s: %s", result.Status, result.Message)
	}
}

func TestRunnerUnsupportedCheckType(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:   "bad-type",
		Name: "Bad Type",
		Type: "unsupported-type",
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical, got %s: %s", result.Status, result.Message)
	}
	if result.Message == "" {
		t.Error("expected error message")
	}
}

func TestRunnerResultTimestamps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-test",
		Name:           "API Test",
		Type:           "api",
		Target:         server.URL,
		ExpectedStatus: 200,
		TimeoutSeconds: 5,
	}

	before := time.Now()
	ctx := context.Background()
	result := runner.executeCheck(ctx, check)
	after := time.Now()

	if result.StartedAt.Before(before) || result.StartedAt.After(after) {
		t.Errorf("StartedAt %v outside expected range [%v, %v]", result.StartedAt, before, after)
	}
	if result.FinishedAt.Before(result.StartedAt) || result.FinishedAt.After(after.Add(time.Second)) {
		t.Errorf("FinishedAt %v outside expected range", result.FinishedAt)
	}
	if result.DurationMs < 0 {
		t.Errorf("expected non-negative DurationMs, got %d", result.DurationMs)
	}
}

func TestRunnerResultTagsAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:          "api-test",
		Name:        "API Test",
		Type:        "api",
		Server:      "prod-1",
		Application: "medics",
		Target:      server.URL,
		Tags:        []string{"api", "critical"},
		Metadata:    map[string]string{"env": "production", "owner": "team-a"},
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Server != "prod-1" {
		t.Errorf("expected Server=prod-1, got %q", result.Server)
	}
	if result.Application != "medics" {
		t.Errorf("expected Application=medics, got %q", result.Application)
	}
	if len(result.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(result.Tags))
	}
	// Tags should be cloned
	if len(result.Tags) > 0 && result.Tags[0] != "api" {
		t.Errorf("expected tag 'api', got %q", result.Tags[0])
	}
}

func TestRunnerResultMetricsInitialized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-test",
		Name:           "API Test",
		Type:           "api",
		Target:         server.URL,
		ExpectedStatus: 200,
	}

	ctx := context.Background()
	result := runner.executeCheck(ctx, check)

	if result.Metrics == nil {
		t.Fatal("expected metrics map to be initialized")
	}
}

func TestRunnerCheckCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-test",
		Name:           "API Test",
		Type:           "api",
		Target:         server.URL,
		TimeoutSeconds: 10,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := runner.executeCheck(ctx, check)

	if result.Status != "critical" {
		t.Errorf("expected critical on cancellation, got %s: %s", result.Status, result.Message)
	}
}

func TestPruneResults(t *testing.T) {
	now := time.Now().UTC()
	results := []CheckResult{
		{ID: "r1", CheckID: "c1", FinishedAt: now.Add(-48 * time.Hour)}, // Should be pruned
		{ID: "r2", CheckID: "c1", FinishedAt: now.Add(-24 * time.Hour)}, // Should be pruned if retention < 2
		{ID: "r3", CheckID: "c1", FinishedAt: now.Add(-12 * time.Hour)}, // Keep
		{ID: "r4", CheckID: "c1", FinishedAt: now},                      // Keep
	}

	// Test with 1 day retention
	pruneResults(&results, 1)
	if len(results) != 2 {
		t.Errorf("expected 2 results after pruning with 1 day retention, got %d", len(results))
	}
	for _, r := range results {
		if r.ID == "r1" || r.ID == "r2" {
			t.Errorf("old result %s should have been pruned", r.ID)
		}
	}

	// Test with 3 day retention (should keep all)
	results = []CheckResult{
		{ID: "r1", CheckID: "c1", FinishedAt: now.Add(-48 * time.Hour)},
		{ID: "r2", CheckID: "c1", FinishedAt: now.Add(-24 * time.Hour)},
		{ID: "r3", CheckID: "c1", FinishedAt: now.Add(-12 * time.Hour)},
		{ID: "r4", CheckID: "c1", FinishedAt: now},
	}
	pruneResults(&results, 3)
	if len(results) != 4 {
		t.Errorf("expected 4 results with 3 day retention, got %d", len(results))
	}
}

func TestCloneTags(t *testing.T) {
	tags := []string{"a", "b", "c"}
	cloned := cloneTags(tags)

	if len(cloned) != len(tags) {
		t.Errorf("length mismatch: %d vs %d", len(cloned), len(tags))
	}

	// Modify original - should not affect clone
	tags[0] = "z"
	if cloned[0] == "z" {
		t.Error("clone was not independent")
	}

	// Test nil
	if cloneTags(nil) != nil {
		t.Error("expected nil for nil input")
	}

	// Test empty
	empty := cloneTags([]string{})
	if len(empty) != 0 {
		t.Error("expected empty slice")
	}
}

func TestProcessListCommand(t *testing.T) {
	binary, args := processListCommand()

	if binary != "ps" {
		t.Errorf("expected 'ps' binary, got %q", binary)
	}

	if len(args) == 0 {
		t.Error("expected args, got empty slice")
	}

	// Just verify it can run
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.Output()
	if err != nil {
		t.Logf("Note: ps command failed (may be expected in some environments): %v", err)
	}
	if len(output) == 0 {
		t.Logf("Note: ps produced no output (may be expected in some environments)")
	}
}
