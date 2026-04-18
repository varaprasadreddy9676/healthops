package monitoring

import (
	"context"
	"io"
	"log"
	"sync"
	"time"
)

// CheckScheduler manages per-check scheduling with retry logic and cooldown tracking.
type CheckScheduler struct {
	repo          CheckRepository
	runner        *Runner
	logger        *log.Logger
	alertCallback func([]CheckResult) // Optional callback to evaluate alerts

	mu        sync.Mutex
	timers    map[string]*time.Timer
	cooldowns map[string]time.Time
	cancel    context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup
	running   bool
	config    *Config
}

// CheckRepository defines the interface for accessing check configurations.
type CheckRepository interface {
	Snapshot() State
	UpsertCheck(CheckConfig) error
	DeleteCheck(string) error
	ReplaceChecks([]CheckConfig) error
}

// NewCheckScheduler creates a new CheckScheduler instance.
func NewCheckScheduler(cfg *Config, repo CheckRepository, runner *Runner, logger *log.Logger) *CheckScheduler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &CheckScheduler{
		repo:          repo,
		runner:        runner,
		logger:        logger,
		alertCallback: nil, // Can be set via SetAlertCallback
		timers:        make(map[string]*time.Timer),
		cooldowns:     make(map[string]time.Time),
		cancel:        cancel,
		ctx:           ctx,
		config:        cfg,
	}
}

// SetAlertCallback sets the callback function to evaluate alerts after check runs.
func (s *CheckScheduler) SetAlertCallback(callback func([]CheckResult)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertCallback = callback
}

// Start begins scheduling checks based on their individual intervals.
func (s *CheckScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.rescheduleAll()
	s.logger.Printf("scheduler started with %d checks", len(s.getEnabledChecks()))
}

// Stop gracefully shuts down the scheduler, waiting for active checks to complete.
func (s *CheckScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false

	// Stop all timers
	for checkID, timer := range s.timers {
		timer.Stop()
		delete(s.timers, checkID)
	}

	s.cancel()
	s.mu.Unlock()

	// Wait for active checks to complete
	s.wg.Wait()
	s.logger.Printf("scheduler stopped")
}

// RescheduleAll updates all check schedules when configuration changes.
func (s *CheckScheduler) RescheduleAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.rescheduleAll()
}

// UpsertSchedule updates schedule for a single check.
func (s *CheckScheduler) UpsertSchedule(check CheckConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.scheduleCheck(check)
}

// RemoveSchedule removes a scheduled check timer by ID.
func (s *CheckScheduler) RemoveSchedule(checkID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timer, exists := s.timers[checkID]; exists {
		timer.Stop()
		delete(s.timers, checkID)
	}
	delete(s.cooldowns, checkID)
}

// rescheduleAll clears existing timers and creates new ones for all enabled checks.
// Must be called while holding the mutex.
func (s *CheckScheduler) rescheduleAll() {
	// Stop existing timers
	for checkID, timer := range s.timers {
		timer.Stop()
		delete(s.timers, checkID)
	}

	// Schedule enabled checks
	checks := s.getEnabledChecks()
	for _, check := range checks {
		s.scheduleCheck(check)
	}
}

// scheduleCheck sets up a timer for a single check based on its interval.
// Must be called while holding the mutex.
func (s *CheckScheduler) scheduleCheck(check CheckConfig) {
	// Stop existing timer if any
	if timer, exists := s.timers[check.ID]; exists {
		timer.Stop()
		delete(s.timers, check.ID)
	}

	if !check.IsEnabled() {
		return
	}

	// Use check-specific interval or fall back to default
	interval := time.Duration(s.getCheckInterval(check)) * time.Second
	if interval <= 0 {
		interval = time.Duration(s.config.CheckIntervalSeconds) * time.Second
	}

	// Schedule immediate first run, then continue on interval.
	timer := time.AfterFunc(0, func() {
		s.runCheck(check)

		// Reschedule for next run
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(interval):
				// Check if still enabled
				state := s.repo.Snapshot()
				currentCheck, exists := findCheckByID(state.Checks, check.ID)
				if !exists || !currentCheck.IsEnabled() {
					return
				}

				// Check if in cooldown
				s.mu.Lock()
				cooldownUntil, inCooldown := s.cooldowns[check.ID]
				s.mu.Unlock()

				if inCooldown && time.Now().Before(cooldownUntil) {
					// Still in cooldown, skip this run
					continue
				}

				s.runCheck(currentCheck)
			}
		}
	})

	s.timers[check.ID] = timer
}

// runCheck executes a single check with retry logic.
func (s *CheckScheduler) runCheck(check CheckConfig) {
	s.wg.Add(1)
	defer s.wg.Done()

	select {
	case <-s.ctx.Done():
		return
	default:
	}

	// Check if in cooldown
	s.mu.Lock()
	cooldownUntil, inCooldown := s.cooldowns[check.ID]
	s.mu.Unlock()

	if inCooldown && time.Now().Before(cooldownUntil) {
		s.logger.Printf("check %s in cooldown until %v, skipping", check.ID, cooldownUntil)
		return
	}

	// Execute with retries
	var lastResult CheckResult
	maxRetries := s.getCheckRetryCount(check)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(s.getCheckRetryDelay(check)) * time.Second
			s.logger.Printf("retry attempt %d/%d for check %s after %v", attempt, maxRetries, check.ID, delay)
			select {
			case <-time.After(delay):
			case <-s.ctx.Done():
				return
			}
		}

		// Run this specific check only.
		result, err := s.runner.RunCheck(s.ctx, check)
		if err != nil {
			s.logger.Printf("check %s run failed: %v", check.ID, err)
			continue
		}

		// Evaluate alerts if callback is configured
		s.mu.Lock()
		alertCallback := s.alertCallback
		s.mu.Unlock()
		if alertCallback != nil {
			alertCallback([]CheckResult{result})
		}

		lastResult = result

		// If check passed, no need to retry
		if lastResult.Healthy {
			s.logger.Printf("check %s passed on attempt %d", check.ID, attempt+1)
			return
		}

		s.logger.Printf("check %s failed on attempt %d: %s", check.ID, attempt+1, lastResult.Message)
	}

	// All retries exhausted - apply cooldown if configured
	cooldownSeconds := s.getCheckCooldown(check)
	if cooldownSeconds > 0 {
		cooldownUntil := time.Now().Add(time.Duration(cooldownSeconds) * time.Second)
		s.mu.Lock()
		s.cooldowns[check.ID] = cooldownUntil
		s.mu.Unlock()
		s.logger.Printf("check %s in cooldown until %v after %d failed attempts", check.ID, cooldownUntil, maxRetries+1)
	}
}

// getCheckInterval returns the interval for a check, using check-specific or default.
func (s *CheckScheduler) getCheckInterval(check CheckConfig) int {
	if check.IntervalSeconds > 0 {
		return check.IntervalSeconds
	}
	return s.config.CheckIntervalSeconds
}

// getCheckRetryCount returns the retry count for a check, using check-specific or default.
func (s *CheckScheduler) getCheckRetryCount(check CheckConfig) int {
	if check.RetryCount >= 0 {
		return check.RetryCount
	}
	return 0 // Default: no retries
}

// getCheckRetryDelay returns the retry delay for a check, using check-specific or default.
func (s *CheckScheduler) getCheckRetryDelay(check CheckConfig) int {
	if check.RetryDelaySeconds > 0 {
		return check.RetryDelaySeconds
	}
	return 5 // Default: 5 seconds
}

// getCheckCooldown returns the cooldown duration for a check after failures.
func (s *CheckScheduler) getCheckCooldown(check CheckConfig) int {
	if check.CooldownSeconds > 0 {
		return check.CooldownSeconds
	}
	return 0 // Default: no cooldown
}

// getEnabledChecks returns all enabled checks from the repository.
func (s *CheckScheduler) getEnabledChecks() []CheckConfig {
	state := s.repo.Snapshot()
	checks := make([]CheckConfig, 0, len(state.Checks))
	for _, check := range state.Checks {
		if check.IsEnabled() {
			checks = append(checks, check)
		}
	}
	return checks
}

// findCheckByID finds a check by its ID in a slice.
func findCheckByID(checks []CheckConfig, id string) (CheckConfig, bool) {
	for _, check := range checks {
		if check.ID == id {
			return check, true
		}
	}
	return CheckConfig{}, false
}
