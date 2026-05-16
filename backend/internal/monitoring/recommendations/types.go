package recommendations

import "time"

// Category classifies a recommendation.
type Category string

const (
	CategoryThreshold Category = "threshold"
	CategoryCoverage  Category = "coverage"
	CategoryStuck     Category = "stuck"
)

// Priority indicates urgency.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// Recommendation is a single actionable suggestion.
type Recommendation struct {
	ID          string            `json:"id"`
	Category    Category          `json:"category"`
	Priority    Priority          `json:"priority"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	CheckID     string            `json:"checkId,omitempty"`
	Server      string            `json:"server,omitempty"`
	Current     map[string]any    `json:"current,omitempty"`
	Suggested   map[string]any    `json:"suggested,omitempty"`
	Reason      string            `json:"reason"`
	CreatedAt   time.Time         `json:"createdAt"`
	Dismissed   bool              `json:"dismissed"`
	DismissedAt *time.Time        `json:"dismissedAt,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// GenerateRequest is the input for on-demand generation.
type GenerateRequest struct {
	UseAI bool `json:"useAi"`
}

// GenerateResponse wraps generated recommendations.
type GenerateResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
	GeneratedAt     time.Time        `json:"generatedAt"`
	AIEnriched      bool             `json:"aiEnriched"`
}

// DismissRequest marks a recommendation as dismissed.
type DismissRequest struct {
	Reason string `json:"reason,omitempty"`
}
