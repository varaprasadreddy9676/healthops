package evidence

import "time"

// SignalEvent is the common envelope for all operational signals. Every
// collector, normalizer, correlator, and the AI context builder operate on
// this type. It implements the "Common Signal Schema" from the roadmap.
type SignalEvent struct {
	ID              string            `json:"id" bson:"_id"`
	TenantID        string            `json:"tenantId" bson:"tenantId"`
	Type            SignalType        `json:"type" bson:"type"`
	Timestamp       time.Time         `json:"timestamp" bson:"timestamp"`
	Severity        string            `json:"severity" bson:"severity"`
	Service         string            `json:"service" bson:"service"`
	Environment     string            `json:"environment" bson:"environment"`
	Host            string            `json:"host" bson:"host"`
	Source          string            `json:"source" bson:"source"`
	Fingerprint     string            `json:"fingerprint,omitempty" bson:"fingerprint,omitempty"`
	TraceID         string            `json:"traceId,omitempty" bson:"traceId,omitempty"`
	SpanID          string            `json:"spanId,omitempty" bson:"spanId,omitempty"`
	DeploymentID    string            `json:"deploymentId,omitempty" bson:"deploymentId,omitempty"`
	IncidentID      string            `json:"incidentId,omitempty" bson:"incidentId,omitempty"`
	Message         string            `json:"message" bson:"message"`
	Attributes      map[string]string `json:"attributes,omitempty" bson:"attributes,omitempty"`
	RedactionStatus string            `json:"redactionStatus" bson:"redactionStatus"`
}

// SignalType identifies the kind of operational signal.
type SignalType string

const (
	SignalTypeCheck      SignalType = "check"
	SignalTypeMetric     SignalType = "metric"
	SignalTypeLog        SignalType = "log"
	SignalTypeDeployment SignalType = "deployment"
	SignalTypeHeartbeat  SignalType = "heartbeat"
	SignalTypeAudit      SignalType = "audit"
	SignalTypeSecurity   SignalType = "security"
	SignalTypeMySQL      SignalType = "mysql"
	SignalTypeServer     SignalType = "server"
)

// IncidentEvent is a timestamped entry in an incident's timeline. It records
// what happened, when, and what evidence backs it. This is the building block
// for the AI Incident Brief's evidence-cited narrative.
type IncidentEvent struct {
	ID         string            `json:"id" bson:"_id"`
	IncidentID string            `json:"incidentId" bson:"incidentId"`
	Type       IncidentEventType `json:"type" bson:"type"`
	Timestamp  time.Time         `json:"timestamp" bson:"timestamp"`
	Source     string            `json:"source" bson:"source"`
	Message    string            `json:"message" bson:"message"`
	SignalIDs  []string          `json:"signalIds,omitempty" bson:"signalIds,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty" bson:"attributes,omitempty"`
}

// IncidentEventType categorizes timeline entries.
type IncidentEventType string

const (
	EventTypeCreated        IncidentEventType = "incident_created"
	EventTypeAcknowledged   IncidentEventType = "incident_acknowledged"
	EventTypeResolved       IncidentEventType = "incident_resolved"
	EventTypeEscalated      IncidentEventType = "incident_escalated"
	EventTypeCheckFailed    IncidentEventType = "check_failed"
	EventTypeCheckRecovered IncidentEventType = "check_recovered"
	EventTypeMySQLAnomaly   IncidentEventType = "mysql_anomaly"
	EventTypeServerAnomaly  IncidentEventType = "server_anomaly"
	EventTypeAuditAction    IncidentEventType = "audit_action"
	EventTypeAIBrief        IncidentEventType = "ai_brief_generated"
)

// TimeWindow defines a bounded time range for evidence retrieval.
type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// IncidentBrief is the structured output of the AI Incident Brief v1.
type IncidentBrief struct {
	IncidentID      string             `json:"incidentId" bson:"incidentId"`
	GeneratedAt     time.Time          `json:"generatedAt" bson:"generatedAt"`
	LikelyCause     string             `json:"likelyCause" bson:"likelyCause"`
	Confidence      ConfidenceScore    `json:"confidence" bson:"confidence"`
	EvidenceSummary []EvidenceCitation `json:"evidenceSummary" bson:"evidenceSummary"`
	NextActions     []string           `json:"nextActions" bson:"nextActions"`
	ImpactSummary   string             `json:"impactSummary,omitempty" bson:"impactSummary,omitempty"`
	Timeline        []TimelineEntry    `json:"timeline,omitempty" bson:"timeline,omitempty"`
	Metadata        BriefMetadata      `json:"metadata" bson:"metadata"`
	RawAIResponse   string             `json:"rawAiResponse,omitempty" bson:"rawAiResponse,omitempty"`
}

// EvidenceCitation links a claim in the brief to specific evidence.
type EvidenceCitation struct {
	Category    string `json:"category" bson:"category"`
	Description string `json:"description" bson:"description"`
	SignalID    string `json:"signalId,omitempty" bson:"signalId,omitempty"`
	Timestamp   string `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
}

// TimelineEntry is a condensed timeline item for the brief output.
type TimelineEntry struct {
	Time        string `json:"time" bson:"time"`
	Description string `json:"description" bson:"description"`
}

// ConfidenceScore is the deterministic, evidence-weighted confidence.
type ConfidenceScore struct {
	Score float64 `json:"score" bson:"score"`
	Band  string  `json:"band" bson:"band"` // "low", "medium", "high"
	// Breakdown shows which factors contributed.
	Breakdown ConfidenceBreakdown `json:"breakdown" bson:"breakdown"`
}

// ConfidenceBreakdown shows the per-factor contributions.
type ConfidenceBreakdown struct {
	HasDeploymentCorrelation bool    `json:"hasDeploymentCorrelation" bson:"hasDeploymentCorrelation"`
	HasMetricAnomaly         bool    `json:"hasMetricAnomaly" bson:"hasMetricAnomaly"`
	HasLogFingerprintSpike   bool    `json:"hasLogFingerprintSpike" bson:"hasLogFingerprintSpike"`
	HasSimilarPastIncident   bool    `json:"hasSimilarPastIncident" bson:"hasSimilarPastIncident"`
	EvidenceCount            int     `json:"evidenceCount" bson:"evidenceCount"`
	EvidenceCountNormalized  float64 `json:"evidenceCountNormalized" bson:"evidenceCountNormalized"`
	AvailableCategories      int     `json:"availableCategories" bson:"availableCategories"`
	TotalPossibleCategories  int     `json:"totalPossibleCategories" bson:"totalPossibleCategories"`
}

// BriefMetadata records which evidence categories were available vs possible.
type BriefMetadata struct {
	AvailableCategories []string `json:"availableCategories" bson:"availableCategories"`
	MissingCategories   []string `json:"missingCategories" bson:"missingCategories"`
	EvidenceCount       int      `json:"evidenceCount" bson:"evidenceCount"`
	EvidenceCap         int      `json:"evidenceCap" bson:"evidenceCap"`
	WasCapped           bool     `json:"wasCapped" bson:"wasCapped"`
	DurationMs          int64    `json:"durationMs" bson:"durationMs"`
}
