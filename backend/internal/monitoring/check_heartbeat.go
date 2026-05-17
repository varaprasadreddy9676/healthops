package monitoring

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// HeartbeatPing represents a single ping from a heartbeat client.
type HeartbeatPing struct {
	Token     string    `json:"token"`
	Status    string    `json:"status,omitempty"`
	Message   string    `json:"message,omitempty"`
	PingedAt  time.Time `json:"pingedAt"`
	IPAddress string    `json:"ipAddress,omitempty"`
}

// HeartbeatState tracks the current state of a heartbeat check.
type HeartbeatState struct {
	Token        string         `json:"token"`
	CheckID      string         `json:"checkId"`
	LastPing     *HeartbeatPing `json:"lastPing,omitempty"`
	PingCount    int64          `json:"pingCount"`
	MissedCount  int64          `json:"missedCount"`
	CurrentState string         `json:"currentState"`
}

// HeartbeatStore stores heartbeat pings in memory.
type HeartbeatStore struct {
	mu     sync.RWMutex
	states map[string]*HeartbeatState
}

var globalHeartbeatStore = &HeartbeatStore{
	states: make(map[string]*HeartbeatState),
}

// GetHeartbeatStore returns the global heartbeat store.
func GetHeartbeatStore() *HeartbeatStore {
	return globalHeartbeatStore
}

// Register registers a heartbeat token for a check ID.
func (s *HeartbeatStore) Register(token, checkID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.states[token]; !exists {
		s.states[token] = &HeartbeatState{
			Token:        token,
			CheckID:      checkID,
			CurrentState: "missed",
		}
	}
}

// RecordPing records a heartbeat ping.
func (s *HeartbeatStore) RecordPing(ping HeartbeatPing) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.states[ping.Token]
	if !exists {
		return fmt.Errorf("unknown heartbeat token")
	}
	state.LastPing = &ping
	state.PingCount++
	state.CurrentState = "healthy"
	return nil
}

// GetState returns the heartbeat state for a token.
func (s *HeartbeatStore) GetState(token string) (HeartbeatState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, exists := s.states[token]
	if !exists {
		return HeartbeatState{}, false
	}
	return *state, true
}

// AllStates returns all heartbeat states.
func (s *HeartbeatStore) AllStates() []HeartbeatState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HeartbeatState, 0, len(s.states))
	for _, state := range s.states {
		out = append(out, *state)
	}
	return out
}

// GenerateHeartbeatToken creates a cryptographically random heartbeat token.
func GenerateHeartbeatToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("hb-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

type heartbeatCheckExecutor struct{}

func init() { RegisterCheckExecutor(&heartbeatCheckExecutor{}) }

func (e *heartbeatCheckExecutor) Type() string { return "heartbeat" }

func (e *heartbeatCheckExecutor) ApplyDefaults(c *CheckConfig) {
	if c.Heartbeat == nil {
		c.Heartbeat = &HeartbeatCheckConfig{}
	}
	if c.Heartbeat.Token == "" {
		c.Heartbeat.Token = GenerateHeartbeatToken()
	}
	if c.Heartbeat.GraceSeconds <= 0 {
		c.Heartbeat.GraceSeconds = 60
	}
	GetHeartbeatStore().Register(c.Heartbeat.Token, c.ID)
}

func (e *heartbeatCheckExecutor) Validate(c *CheckConfig, cfg *Config) error {
	if c.Heartbeat == nil {
		return fmt.Errorf("heartbeat config block is required for heartbeat checks")
	}
	if c.Heartbeat.ExpectedIntervalSeconds <= 0 {
		return fmt.Errorf("heartbeat.expectedIntervalSeconds must be > 0")
	}
	return nil
}

func (e *heartbeatCheckExecutor) Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error {
	cfg := check.Heartbeat
	if cfg == nil {
		return fmt.Errorf("heartbeat config is nil")
	}

	store := GetHeartbeatStore()
	state, exists := store.GetState(cfg.Token)

	result.Metrics["expectedIntervalSeconds"] = float64(cfg.ExpectedIntervalSeconds)
	result.Metrics["graceSeconds"] = float64(cfg.GraceSeconds)

	if !exists || state.LastPing == nil {
		result.Metrics["missedCount"] = 0
		result.Metrics["pingCount"] = 0
		return fmt.Errorf("no heartbeat ping received yet for check %s", check.Name)
	}

	result.Metrics["pingCount"] = float64(state.PingCount)
	result.Metrics["missedCount"] = float64(state.MissedCount)

	now := time.Now().UTC()
	age := now.Sub(state.LastPing.PingedAt)
	result.Metrics["lastPingAgeSeconds"] = age.Seconds()

	deadline := time.Duration(cfg.ExpectedIntervalSeconds+cfg.GraceSeconds) * time.Second

	if state.LastPing.Status == "fail" {
		return fmt.Errorf("heartbeat %s reported failure: %s", check.Name, state.LastPing.Message)
	}

	if age > deadline {
		store.mu.Lock()
		if s, ok := store.states[cfg.Token]; ok {
			s.MissedCount++
			s.CurrentState = "missed"
		}
		store.mu.Unlock()
		return fmt.Errorf("heartbeat %s is late: last ping %.0fs ago, expected every %ds (+%ds grace)",
			check.Name, age.Seconds(), cfg.ExpectedIntervalSeconds, cfg.GraceSeconds)
	}

	warnThreshold := time.Duration(cfg.ExpectedIntervalSeconds) * time.Second
	if age > warnThreshold {
		result.Status = "warning"
		result.Healthy = false
		result.Message = fmt.Sprintf("heartbeat %s: last ping %.0fs ago (within grace period, deadline at %ds)",
			check.Name, age.Seconds(), cfg.ExpectedIntervalSeconds+cfg.GraceSeconds)
		return nil
	}

	return nil
}
