package automation

import "time"

// ActionType classifies what the automation suggests.
type ActionType string

const (
	ActionRestart          ActionType = "restart"
	ActionDrainNode        ActionType = "drain_node"
	ActionRotateCredential ActionType = "rotate_credential"
	ActionInspectQueries   ActionType = "inspect_queries"
	ActionScaleUp          ActionType = "scale_up"
	ActionClearQueue       ActionType = "clear_queue"
	ActionCustom           ActionType = "custom"
)

// ApprovalStatus tracks the human decision.
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusRejected ApprovalStatus = "rejected"
	StatusExpired  ApprovalStatus = "expired"
)

// RiskLevel indicates potential impact.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Action is a suggested remediation that requires human approval.
type Action struct {
	ID          string         `json:"id"`
	Type        ActionType     `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Risk        RiskLevel      `json:"risk"`
	CheckID     string         `json:"checkId,omitempty"`
	Server      string         `json:"server,omitempty"`
	IncidentID  string         `json:"incidentId,omitempty"`
	Command     string         `json:"command,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	Reason      string         `json:"reason"`
	Status      ApprovalStatus `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	ExpiresAt   time.Time      `json:"expiresAt"`
	ApprovedBy  string         `json:"approvedBy,omitempty"`
	ApprovedAt  *time.Time     `json:"approvedAt,omitempty"`
	RejectedBy  string         `json:"rejectedBy,omitempty"`
	RejectedAt  *time.Time     `json:"rejectedAt,omitempty"`
	ExecutedAt  *time.Time     `json:"executedAt,omitempty"`
	Result      string         `json:"result,omitempty"`
}

// AuditEntry records every AI suggestion and human decision.
type AuditEntry struct {
	ID        string    `json:"id"`
	ActionID  string    `json:"actionId"`
	Actor     string    `json:"actor"`
	Event     string    `json:"event"` // "suggested", "approved", "rejected", "executed", "expired"
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// SuggestRequest triggers AI-powered action suggestions.
type SuggestRequest struct {
	IncidentID string `json:"incidentId,omitempty"`
	CheckID    string `json:"checkId,omitempty"`
	Context    string `json:"context,omitempty"`
}

// ApproveRequest approves a pending action.
type ApproveRequest struct {
	Actor string `json:"actor"`
}

// RejectRequest rejects a pending action.
type RejectRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason,omitempty"`
}
