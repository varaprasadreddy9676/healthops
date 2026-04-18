package monitoring

import (
	"log"
	"sync"
	"testing"
	"time"
)

// MockStore for testing
type mockStore struct {
	mu     sync.Mutex
	state  State
	closed bool
}

func newMockStore(checks []CheckConfig) *mockStore {
	return &mockStore{
		state: State{
			Checks:  checks,
			Results: []CheckResult{},
		},
	}
}

func (m *mockStore) Snapshot() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *mockStore) DashboardSnapshot() DashboardSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return DashboardSnapshot{
		State:   m.state,
		Summary: buildSummary(m.state.Checks, m.state.Results, &m.state.LastRunAt),
	}
}

func (m *mockStore) Update(mutator func(*State) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return mutator(&m.state)
}

func (m *mockStore) ReplaceChecks(checks []CheckConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Checks = checks
	return nil
}

func (m *mockStore) UpsertCheck(check CheckConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update existing or append
	found := false
	for i, c := range m.state.Checks {
		if c.ID == check.ID {
			m.state.Checks[i] = check
			found = true
			break
		}
	}
	if !found {
		m.state.Checks = append(m.state.Checks, check)
	}
	return nil
}

func (m *mockStore) DeleteCheck(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	checks := make([]CheckConfig, 0, len(m.state.Checks))
	for _, c := range m.state.Checks {
		if c.ID != id {
			checks = append(checks, c)
		}
	}
	m.state.Checks = checks
	return nil
}

func (m *mockStore) AppendResults(results []CheckResult, retentionDays int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Results = append(m.state.Results, results...)
	pruneResults(&m.state.Results, retentionDays)
	return nil
}

func (m *mockStore) SetLastRun(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.LastRunAt = t
	return nil
}

// Test per-check interval scheduling
func TestCheckScheduler_PerCheckIntervals(t *testing.T) {
	tests := []struct {
		name              string
		checks            []CheckConfig
		expectedRunCounts map[string]int // How many times each check should run
		duration          time.Duration
	}{
		{
			name: "different intervals",
			checks: []CheckConfig{
				{
					ID:              "fast-check",
					Name:            "Fast Check",
					Type:            "api",
					Target:          "http://example.com/fast",
					IntervalSeconds: 1,
					Enabled:         boolPtr(true),
				},
				{
					ID:              "slow-check",
					Name:            "Slow Check",
					Type:            "api",
					Target:          "http://example.com/slow",
					IntervalSeconds: 3,
					Enabled:         boolPtr(true),
				},
			},
			expectedRunCounts: map[string]int{
				"fast-check": 4, // Initial + 3 more in 3 seconds (1s interval)
				"slow-check": 2, // Initial + 1 more in 3 seconds (3s interval)
			},
			duration: 3 * time.Second,
		},
		{
			name: "default interval fallback",
			checks: []CheckConfig{
				{
					ID:      "default-check",
					Name:    "Default Check",
					Type:    "api",
					Target:  "http://example.com/default",
					Enabled: boolPtr(true),
					// IntervalSeconds not set, should use default
				},
			},
			expectedRunCounts: map[string]int{
				"default-check": 1, // Should run ~1 time with default interval
			},
			duration: 2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore(tt.checks)
			cfg := &Config{
				CheckIntervalSeconds: 60, // Default interval
				Workers:              1,
			}
			runner := NewRunner(cfg, store)
			logger := log.New(log.Default().Writer(), "", log.Flags())

			scheduler := NewCheckScheduler(cfg, store, runner, logger)
			scheduler.Start()
			defer scheduler.Stop()

			// Wait for checks to run
			time.Sleep(tt.duration)

			// Verify run counts
			state := store.Snapshot()
			runCounts := make(map[string]int)
			for _, result := range state.Results {
				runCounts[result.CheckID]++
			}

			for checkID, expectedCount := range tt.expectedRunCounts {
				actualCount := runCounts[checkID]
				// Allow some tolerance for timing
				if actualCount < expectedCount-1 || actualCount > expectedCount+1 {
					t.Errorf("Check %s: expected ~%d runs, got %d", checkID, expectedCount, actualCount)
				}
			}
		})
	}
}

// Test retry logic on failure
func TestCheckScheduler_RetryLogic(t *testing.T) {
	tests := []struct {
		name             string
		check            CheckConfig
		retryCount       int
		retryDelay       int
		expectedAttempts int
		shouldSucceed    bool
	}{
		{
			name: "no retries",
			check: CheckConfig{
				ID:              "no-retry",
				Name:            "No Retry",
				Type:            "api",
				Target:          "http://invalid.example.com", // Will fail
				IntervalSeconds: 10,
				RetryCount:      0,
				Enabled:         boolPtr(true),
			},
			retryCount:       0,
			retryDelay:       1,
			expectedAttempts: 1, // Just initial attempt
			shouldSucceed:    false,
		},
		{
			name: "with retries",
			check: CheckConfig{
				ID:                "with-retry",
				Name:              "With Retry",
				Type:              "api",
				Target:            "http://invalid.example.com", // Will fail
				IntervalSeconds:   10,
				RetryCount:        2,
				RetryDelaySeconds: 1,
				Enabled:           boolPtr(true),
			},
			retryCount:       2,
			retryDelay:       1,
			expectedAttempts: 3, // Initial + 2 retries
			shouldSucceed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore([]CheckConfig{tt.check})
			cfg := &Config{
				CheckIntervalSeconds: 60,
				Workers:              1,
			}
			runner := NewRunner(cfg, store)
			logger := log.New(log.Default().Writer(), "", log.Flags())

			scheduler := NewCheckScheduler(cfg, store, runner, logger)
			scheduler.Start()

			// Wait for initial attempt + retries
			expectedDuration := time.Duration(tt.retryCount*tt.retryDelay) * time.Second
			time.Sleep(expectedDuration + 2*time.Second)

			scheduler.Stop()

			// Count attempts
			state := store.Snapshot()
			attempts := 0
			for _, result := range state.Results {
				if result.CheckID == tt.check.ID {
					attempts++
				}
			}

			if attempts < tt.expectedAttempts-1 || attempts > tt.expectedAttempts+1 {
				t.Errorf("Expected %d attempts, got %d", tt.expectedAttempts, attempts)
			}
		})
	}
}

// Test cooldown after failures
func TestCheckScheduler_Cooldown(t *testing.T) {
	check := CheckConfig{
		ID:                "cooldown-check",
		Name:              "Cooldown Check",
		Type:              "api",
		Target:            "http://invalid.example.com",
		IntervalSeconds:   1,
		RetryCount:        1,
		RetryDelaySeconds: 1,
		CooldownSeconds:   5,
		Enabled:           boolPtr(true),
	}

	store := newMockStore([]CheckConfig{check})
	cfg := &Config{
		CheckIntervalSeconds: 60,
		Workers:              1,
	}
	runner := NewRunner(cfg, store)
	logger := log.New(log.Default().Writer(), "", log.Flags())

	scheduler := NewCheckScheduler(cfg, store, runner, logger)
	scheduler.Start()

	// Wait for initial attempt + retry + cooldown setup
	time.Sleep(4 * time.Second)

	// Get attempt count after first round
	state := store.Snapshot()
	initialAttempts := len(state.Results)

	// Wait through cooldown period
	time.Sleep(6 * time.Second)

	// Check if new attempts were made during cooldown
	state = store.Snapshot()
	finalAttempts := len(state.Results)

	// Should have minimal new attempts during cooldown
	// (exactly 2: initial + retry, then cooldown prevents more)
	if finalAttempts > initialAttempts+2 {
		t.Errorf("Too many attempts during cooldown: initial=%d, final=%d", initialAttempts, finalAttempts)
	}

	scheduler.Stop()
}

// Test graceful shutdown
func TestCheckScheduler_GracefulShutdown(t *testing.T) {
	check := CheckConfig{
		ID:              "shutdown-check",
		Name:            "Shutdown Check",
		Type:            "api",
		Target:          "http://example.com",
		TimeoutSeconds:  10, // Long timeout to test interruption
		IntervalSeconds: 1,
		Enabled:         boolPtr(true),
	}

	store := newMockStore([]CheckConfig{check})
	cfg := &Config{
		CheckIntervalSeconds: 60,
		Workers:              1,
	}
	runner := NewRunner(cfg, store)
	logger := log.New(log.Default().Writer(), "", log.Flags())

	scheduler := NewCheckScheduler(cfg, store, runner, logger)
	scheduler.Start()

	// Let it start
	time.Sleep(500 * time.Millisecond)

	// Stop should be graceful
	start := time.Now()
	scheduler.Stop()
	duration := time.Since(start)

	// Should stop quickly (not wait for long timeout)
	if duration > 2*time.Second {
		t.Errorf("Stop took too long: %v", duration)
	}

	// Verify timers are cleaned up
	scheduler.mu.Lock()
	timerCount := len(scheduler.timers)
	scheduler.mu.Unlock()

	if timerCount != 0 {
		t.Errorf("Expected 0 timers after stop, got %d", timerCount)
	}
}

// Test config change rescheduling
func TestCheckScheduler_RescheduleOnConfigChange(t *testing.T) {
	initialChecks := []CheckConfig{
		{
			ID:              "check1",
			Name:            "Check 1",
			Type:            "api",
			Target:          "http://example.com/1",
			IntervalSeconds: 5,
			Enabled:         boolPtr(true),
		},
	}

	store := newMockStore(initialChecks)
	cfg := &Config{
		CheckIntervalSeconds: 60,
		Workers:              1,
	}
	runner := NewRunner(cfg, store)
	logger := log.New(log.Default().Writer(), "", log.Flags())

	scheduler := NewCheckScheduler(cfg, store, runner, logger)
	scheduler.Start()
	defer scheduler.Stop()

	// Verify initial timer exists
	scheduler.mu.Lock()
	initialTimerCount := len(scheduler.timers)
	_, hasCheck1 := scheduler.timers["check1"]
	scheduler.mu.Unlock()

	if initialTimerCount != 1 || !hasCheck1 {
		t.Errorf("Expected 1 timer for check1, got %d timers, hasCheck1=%v", initialTimerCount, hasCheck1)
	}

	// Add new check
	newCheck := CheckConfig{
		ID:              "check2",
		Name:            "Check 2",
		Type:            "api",
		Target:          "http://example.com/2",
		IntervalSeconds: 3,
		Enabled:         boolPtr(true),
	}
	store.UpsertCheck(newCheck)

	// Trigger reschedule
	scheduler.RescheduleAll()

	// Verify new timer exists
	scheduler.mu.Lock()
	newTimerCount := len(scheduler.timers)
	_, hasCheck2 := scheduler.timers["check2"]
	scheduler.mu.Unlock()

	if newTimerCount != 2 || !hasCheck2 {
		t.Errorf("Expected 2 timers including check2, got %d timers, hasCheck2=%v", newTimerCount, hasCheck2)
	}

	// Delete check
	store.DeleteCheck("check1")
	scheduler.RescheduleAll()

	// Verify timer is removed
	scheduler.mu.Lock()
	finalTimerCount := len(scheduler.timers)
	_, stillHasCheck1 := scheduler.timers["check1"]
	scheduler.mu.Unlock()

	if finalTimerCount != 1 || stillHasCheck1 {
		t.Errorf("Expected 1 timer without check1, got %d timers, stillHasCheck1=%v", finalTimerCount, stillHasCheck1)
	}
}

// Test disabled checks are not scheduled
func TestCheckScheduler_DisabledChecksNotScheduled(t *testing.T) {
	checks := []CheckConfig{
		{
			ID:              "enabled-check",
			Name:            "Enabled Check",
			Type:            "api",
			Target:          "http://example.com/enabled",
			IntervalSeconds: 1,
			Enabled:         boolPtr(true),
		},
		{
			ID:              "disabled-check",
			Name:            "Disabled Check",
			Type:            "api",
			Target:          "http://example.com/disabled",
			IntervalSeconds: 1,
			Enabled:         boolPtr(false),
		},
	}

	store := newMockStore(checks)
	cfg := &Config{
		CheckIntervalSeconds: 60,
		Workers:              1,
	}
	runner := NewRunner(cfg, store)
	logger := log.New(log.Default().Writer(), "", log.Flags())

	scheduler := NewCheckScheduler(cfg, store, runner, logger)
	scheduler.Start()
	defer scheduler.Stop()

	// Wait a bit
	time.Sleep(2 * time.Second)

	// Verify only enabled check ran
	state := store.Snapshot()
	enabledCount := 0
	disabledCount := 0
	for _, result := range state.Results {
		if result.CheckID == "enabled-check" {
			enabledCount++
		}
		if result.CheckID == "disabled-check" {
			disabledCount++
		}
	}

	if enabledCount == 0 {
		t.Error("Enabled check should have run at least once")
	}
	if disabledCount > 0 {
		t.Errorf("Disabled check should not have run, but ran %d times", disabledCount)
	}
}

// Test concurrent access safety
func TestCheckScheduler_ConcurrentAccess(t *testing.T) {
	checks := []CheckConfig{
		{
			ID:              "concurrent-check",
			Name:            "Concurrent Check",
			Type:            "api",
			Target:          "http://example.com",
			IntervalSeconds: 1,
			Enabled:         boolPtr(true),
		},
	}

	store := newMockStore(checks)
	cfg := &Config{
		CheckIntervalSeconds: 60,
		Workers:              1,
	}
	runner := NewRunner(cfg, store)
	logger := log.New(log.Default().Writer(), "", log.Flags())

	scheduler := NewCheckScheduler(cfg, store, runner, logger)
	scheduler.Start()
	defer scheduler.Stop()

	// Perform concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduler.RescheduleAll()
		}()
	}

	// Update check concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			check := CheckConfig{
				ID:              "concurrent-check",
				Name:            "Updated Check",
				Type:            "api",
				Target:          "http://example.com/updated",
				IntervalSeconds: 2 + i,
				Enabled:         boolPtr(true),
			}
			store.UpsertCheck(check)
		}(i)
	}

	wg.Wait()

	// Should not panic or deadlock
}
