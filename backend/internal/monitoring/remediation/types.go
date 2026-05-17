package remediation

import "time"

// ActionType classifies how the remediation is executed.
type ActionType string

const (
	ActionCommand    ActionType = "command"     // local shell command
	ActionSSHCommand ActionType = "ssh_command" // remote command over SSH (reuses check's SSH config)
	ActionHTTP       ActionType = "http"        // HTTP call to trigger restart via API
)

// RiskLevel indicates potential impact of the action.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// AttemptStatus tracks the state of a remediation attempt.
type AttemptStatus string

const (
	AttemptPending   AttemptStatus = "pending"
	AttemptRunning   AttemptStatus = "running"
	AttemptSuccess   AttemptStatus = "success"
	AttemptFailed    AttemptStatus = "failed"
	AttemptDryRun    AttemptStatus = "dry_run"
	AttemptTimedOut  AttemptStatus = "timed_out"
	AttemptSkipped   AttemptStatus = "skipped" // cooldown or max attempts
	AttemptEscalated AttemptStatus = "escalated"
)

// GlobalConfig holds the system-wide remediation settings.
type GlobalConfig struct {
	Enabled          bool   `json:"enabled" bson:"enabled"`
	DryRun           bool   `json:"dryRun" bson:"dryRun"`
	MaxConcurrent    int    `json:"maxConcurrent" bson:"maxConcurrent"`
	OutputLimitBytes int    `json:"outputLimitBytes" bson:"outputLimitBytes"`
	UpdatedAt        string `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

// DefaultGlobalConfig returns a safe default (disabled, dry-run).
func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Enabled:          false,
		DryRun:           true,
		MaxConcurrent:    2,
		OutputLimitBytes: 8192,
	}
}

// AllowedAction defines a pre-approved remediation action in the server-side
// registry. Checks reference these by ID via actionRef — raw commands are never
// stored on checks to prevent check-edit permissions from becoming RCE.
type AllowedAction struct {
	ID             string            `json:"id" bson:"_id"`
	Name           string            `json:"name" bson:"name"`
	Type           ActionType        `json:"type" bson:"type"`
	Command        string            `json:"command,omitempty" bson:"command,omitempty"` // for command / ssh_command
	URL            string            `json:"url,omitempty" bson:"url,omitempty"`         // for http
	Method         string            `json:"method,omitempty" bson:"method,omitempty"`   // for http (GET/POST)
	Headers        map[string]string `json:"headers,omitempty" bson:"headers,omitempty"` // for http
	TimeoutSeconds int               `json:"timeoutSeconds" bson:"timeoutSeconds"`
	Risk           RiskLevel         `json:"risk" bson:"risk"`
	Description    string            `json:"description" bson:"description"`
	CreatedAt      time.Time         `json:"createdAt" bson:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt" bson:"updatedAt"`
}

// RemediationRef is the remediation configuration attached to a CheckConfig.
// It references an AllowedAction by ID and controls execution policy.
type RemediationRef struct {
	ActionRef                   string `json:"actionRef" bson:"actionRef"`
	MaxAttempts                 int    `json:"maxAttempts,omitempty" bson:"maxAttempts,omitempty"`
	CooldownSeconds             int    `json:"cooldownSeconds,omitempty" bson:"cooldownSeconds,omitempty"`
	ConsecutiveFailuresRequired int    `json:"consecutiveFailuresRequired,omitempty" bson:"consecutiveFailuresRequired,omitempty"`
	VerifyAfterSeconds          int    `json:"verifyAfterSeconds,omitempty" bson:"verifyAfterSeconds,omitempty"`
	NotifyOnRemediation         bool   `json:"notifyOnRemediation,omitempty" bson:"notifyOnRemediation,omitempty"`
	EscalateOnExhaustion        bool   `json:"escalateOnExhaustion,omitempty" bson:"escalateOnExhaustion,omitempty"`
}

// Defaults fills zero-value fields with safe defaults.
func (r *RemediationRef) Defaults() {
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = 3
	}
	if r.CooldownSeconds <= 0 {
		r.CooldownSeconds = 300 // 5 minutes
	}
	if r.VerifyAfterSeconds <= 0 {
		r.VerifyAfterSeconds = 10
	}
}

// Attempt records a single remediation execution.
type Attempt struct {
	ID            string        `json:"id" bson:"_id"`
	CheckID       string        `json:"checkId" bson:"checkId"`
	IncidentID    string        `json:"incidentId" bson:"incidentId"`
	ActionID      string        `json:"actionId" bson:"actionId"`
	ActionName    string        `json:"actionName" bson:"actionName"`
	ActionType    ActionType    `json:"actionType" bson:"actionType"`
	Command       string        `json:"command" bson:"command"`
	AttemptNumber int           `json:"attemptNumber" bson:"attemptNumber"`
	Status        AttemptStatus `json:"status" bson:"status"`
	DryRun        bool          `json:"dryRun" bson:"dryRun"`
	ExitCode      int           `json:"exitCode" bson:"exitCode"`
	Output        string        `json:"output" bson:"output"`
	Error         string        `json:"error,omitempty" bson:"error,omitempty"`
	Verified      *bool         `json:"verified,omitempty" bson:"verified,omitempty"`
	AIAnalysis    string        `json:"aiAnalysis,omitempty" bson:"aiAnalysis,omitempty"`
	DurationMs    int64         `json:"durationMs" bson:"durationMs"`
	TriggeredBy   string        `json:"triggeredBy" bson:"triggeredBy"` // "system" or username
	CreatedAt     time.Time     `json:"createdAt" bson:"createdAt"`
}

// Repository defines persistence for remediation state.
type Repository interface {
	// Global config
	GetConfig() (GlobalConfig, error)
	SaveConfig(cfg GlobalConfig) error

	// Allowed actions registry
	ListActions() ([]AllowedAction, error)
	GetAction(id string) (AllowedAction, error)
	CreateAction(action AllowedAction) error
	UpdateAction(action AllowedAction) error
	DeleteAction(id string) error

	// Attempts
	ListAttempts(filter AttemptFilter) ([]Attempt, error)
	GetAttempt(id string) (Attempt, error)
	CreateAttempt(attempt Attempt) error
	UpdateAttempt(id string, mutator func(*Attempt)) error
	CountAttempts(checkID, incidentID string) (int, error)
	LastAttempt(checkID string) (*Attempt, error)

	// Retention
	PruneBefore(cutoff time.Time) error
}

// AttemptFilter controls which attempts to retrieve.
type AttemptFilter struct {
	CheckID    string `json:"checkId,omitempty"`
	IncidentID string `json:"incidentId,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}

// AIProvider calls the configured AI model for remediation analysis.
type AIProvider func(systemMsg, userMsg string) (string, error)

// CheckResolverFunc looks up a check and builds a CheckInfo from it.
// Used by the handler for manual remediation.
type CheckResolverFunc func(checkID string) (CheckInfo, error)

// SuggestCommandRequest is sent to AI to suggest a remediation command.
type SuggestCommandRequest struct {
	CheckType   string `json:"checkType"`
	CheckTarget string `json:"checkTarget"`
	ServerHost  string `json:"serverHost,omitempty"`
	FailMessage string `json:"failMessage,omitempty"`
}
