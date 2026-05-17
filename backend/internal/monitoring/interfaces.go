package monitoring

import "time"

// NotificationOutboxRepository defines generic notification queue operations.
type NotificationOutboxRepository interface {
	Enqueue(evt NotificationEvent) error
	ListPending(limit int) ([]NotificationEvent, error)
	ListAll(limit int, status string, channel string) ([]NotificationEvent, error)
	MarkSent(id string) error
	MarkFailed(id string, reason string) error
	PruneBefore(cutoff time.Time) error
	AllNotifications() []NotificationEvent
}

// AIQueueRepository defines generic AI analysis queue operations.
type AIQueueRepository interface {
	Enqueue(incidentID string, promptVersion string) error
	ClaimPending(limit int) ([]AIQueueItem, error)
	Complete(incidentID string, result AIAnalysisResult) error
	Fail(incidentID string, reason string) error
	PruneBefore(cutoff time.Time) error
	ListPendingItems(limit int) ([]AIQueueItem, error)
	AllItems() []AIQueueItem
	// GetResults returns AI analysis results for a specific incident.
	GetResults(incidentID string) []AIAnalysisResult
	// AllResults returns the most recent AI analysis results, up to limit.
	AllResults(limit int) []AIAnalysisResult
}

// MySQLMetricsRepository defines persistence for MySQL samples and deltas.
type MySQLMetricsRepository interface {
	AppendSample(sample MySQLSample) (string, error)
	ComputeAndAppendDelta(sampleID string) (MySQLDelta, error)
	LatestSample(checkID string) (MySQLSample, error)
	RecentSamples(checkID string, limit int) ([]MySQLSample, error)
	RecentDeltas(checkID string, limit int) ([]MySQLDelta, error)
}

// ServerMetricsStore abstracts server metric snapshot persistence.
type ServerMetricsStore interface {
	Save(snap ServerSnapshot) error
	GetSnapshots(serverID string, since, until time.Time) ([]ServerSnapshot, error)
	GetLatest(serverID string) (*ServerSnapshot, error)
	PruneBefore(cutoff time.Time) error
}

// MaintenanceWindowStore abstracts maintenance window persistence.
type MaintenanceWindowStore interface {
	Create(mw MaintenanceWindow) error
	Update(id string, mutator func(*MaintenanceWindow) error) error
	Delete(id string) error
	Get(id string) (MaintenanceWindow, error)
	List() []MaintenanceWindow
	ListActive() []MaintenanceWindow
	IsCheckInMaintenance(check CheckConfig) bool
	PruneExpired(cutoff time.Time) (int, error)
}

// CustomDashboardRepository abstracts custom dashboard persistence.
type CustomDashboardRepository interface {
	Create(d CustomDashboard) (*CustomDashboard, error)
	Get(id string) (*CustomDashboard, error)
	List(owner string) []CustomDashboard
	Update(id string, update CustomDashboard) (*CustomDashboard, error)
	Delete(id string) error
	Duplicate(id, newName string) (*CustomDashboard, error)
}

// StatusPageRepository abstracts status page persistence.
type StatusPageRepository interface {
	Create(cfg StatusPageConfig) (*StatusPageConfig, error)
	Get(id string) (*StatusPageConfig, error)
	GetBySlug(slug string) (*StatusPageConfig, error)
	List() []StatusPageConfig
	Update(id string, update StatusPageConfig) (*StatusPageConfig, error)
	UpdatePartial(id string, update StatusPageConfigUpdate) (*StatusPageConfig, error)
	Delete(id string) error
}

// ChatRepository abstracts AI chat conversation persistence.
type ChatRepository interface {
	CreateConversation(title, owner, ctx string) *ChatConversation
	GetConversation(id string) (*ChatConversation, error)
	ListConversations(owner string) []ChatConversation
	AddMessage(conversationID string, msg ChatMessage) error
	DeleteConversation(id string) error
	PruneOld(maxAge time.Duration) int
}
