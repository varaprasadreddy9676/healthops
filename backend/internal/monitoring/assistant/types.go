package assistant

import "time"

// Message represents a single message in a conversation.
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// AskRequest is the payload for a natural-language question.
type AskRequest struct {
	Question        string    `json:"question"`
	History         []Message `json:"history,omitempty"`         // optional prior context
	LookbackMinutes int       `json:"lookbackMinutes,omitempty"` // 0 = use default (48h)
}

// Reference points to a telemetry entity referenced in the answer.
type Reference struct {
	Type string `json:"type"` // "check", "incident", "server", "log_family"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AskResponse contains the AI-generated answer plus metadata.
type AskResponse struct {
	Answer     string      `json:"answer"`
	References []Reference `json:"references,omitempty"`
	Duration   int64       `json:"durationMs"`
	Provider   string      `json:"provider,omitempty"`
}

// TelemetryContext is the summarized system state fed to the AI.
type TelemetryContext struct {
	Checks    []CheckSummary    `json:"checks"`
	Incidents []IncidentSummary `json:"incidents"`
	Servers   []ServerSummary   `json:"servers"`
}

// CheckSummary is a compact view of a check's recent status.
type CheckSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Status   string `json:"status"`
	Latency  int64  `json:"latencyMs,omitempty"`
	LastRun  string `json:"lastRun,omitempty"`
	Message  string `json:"message,omitempty"`
	Failures int    `json:"recentFailures,omitempty"`
}

// IncidentSummary is a compact view of a recent incident.
type IncidentSummary struct {
	ID        string `json:"id"`
	CheckID   string `json:"checkId"`
	CheckName string `json:"checkName"`
	Server    string `json:"server"`
	Status    string `json:"status"`
	Severity  string `json:"severity"`
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message,omitempty"`
}

// ServerSummary is a compact view of server health.
type ServerSummary struct {
	Name         string `json:"name"`
	TotalChecks  int    `json:"totalChecks"`
	Healthy      int    `json:"healthy"`
	Unhealthy    int    `json:"unhealthy"`
	WorstLatency int64  `json:"worstLatencyMs,omitempty"`
}
