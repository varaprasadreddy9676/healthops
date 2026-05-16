package rca

import (
	"context"
	"time"
)

// SignalPoint is a single metric observation at a point in time.
type SignalPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Source    string    `json:"source"`
	Server    string    `json:"server,omitempty"`
}

// SignalSeries groups related signal points for trend analysis.
type SignalSeries struct {
	Name   string        `json:"name"`
	Source string        `json:"source"`
	Server string        `json:"server,omitempty"`
	Points []SignalPoint `json:"points"`
	Min    float64       `json:"min"`
	Max    float64       `json:"max"`
	Avg    float64       `json:"avg"`
	Trend  string        `json:"trend"` // rising, falling, stable, spike
}

// CorrelationContext is the multi-signal package sent to AI for analysis.
type CorrelationContext struct {
	IncidentID    string            `json:"incidentId"`
	CheckName     string            `json:"checkName"`
	CheckType     string            `json:"checkType"`
	Severity      string            `json:"severity"`
	Message       string            `json:"message"`
	StartedAt     time.Time         `json:"startedAt"`
	Duration      string            `json:"duration"`
	WindowStart   time.Time         `json:"windowStart"`
	WindowEnd     time.Time         `json:"windowEnd"`
	Signals       []SignalSeries    `json:"signals"`
	ErrorFamilies []ErrorFamilyRef  `json:"errorFamilies,omitempty"`
	RecentEvents  []TimelineEvent   `json:"recentEvents"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ErrorFamilyRef is a lightweight reference to a log error family.
type ErrorFamilyRef struct {
	ID              string `json:"id"`
	Pattern         string `json:"pattern"`
	Category        string `json:"category"`
	OccurrenceCount int    `json:"occurrenceCount"`
	LastSeenAt      string `json:"lastSeenAt"`
}

// TimelineEvent is a notable event during the incident window.
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // check_fail, check_recover, incident_open, log_spike, metric_anomaly
	Description string    `json:"description"`
	Severity    string    `json:"severity,omitempty"`
	Source      string    `json:"source,omitempty"`
}

// RCAHypothesis is a single ranked root cause hypothesis.
type RCAHypothesis struct {
	Rank        int      `json:"rank"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"` // 0.0 to 1.0
	Category    string   `json:"category"`   // resource, network, application, database, config
	Evidence    []string `json:"evidence"`   // Supporting signal names/observations
	Suggestion  string   `json:"suggestion"` // Recommended action
}

// RCAReport is the full root-cause analysis output.
type RCAReport struct {
	ID           string          `json:"id"`
	IncidentID   string          `json:"incidentId"`
	CreatedAt    time.Time       `json:"createdAt"`
	Status       string          `json:"status"` // pending, complete, failed
	Hypotheses   []RCAHypothesis `json:"hypotheses,omitempty"`
	Summary      string          `json:"summary,omitempty"`
	Timeline     []TimelineEvent `json:"timeline,omitempty"`
	SignalCount  int             `json:"signalCount"`
	WindowStart  time.Time       `json:"windowStart"`
	WindowEnd    time.Time       `json:"windowEnd"`
	ProviderUsed string          `json:"providerUsed,omitempty"`
	Error        string          `json:"error,omitempty"`
}

// AIProvider is the interface for sending prompts to an AI model.
type AIProvider interface {
	Analyze(ctx context.Context, systemMsg, userMsg string) (string, error)
}

// SignalSource provides check results for correlation.
type SignalSource interface {
	RecentResults(checkID string, limit int) []CheckResultRef
	AllRecentResults(since time.Time, limit int) []CheckResultRef
}

// CheckResultRef is a lightweight check result reference.
type CheckResultRef struct {
	CheckID    string             `json:"checkId"`
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Server     string             `json:"server,omitempty"`
	Status     string             `json:"status"`
	DurationMs int64              `json:"durationMs"`
	Timestamp  time.Time          `json:"timestamp"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
}

// IncidentRef is a lightweight incident reference.
type IncidentRef struct {
	ID        string    `json:"id"`
	CheckID   string    `json:"checkId"`
	CheckName string    `json:"checkName"`
	Severity  string    `json:"severity"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt"`
	Message   string    `json:"message"`
}
