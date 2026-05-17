package logs

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// LogEntry represents a single ingested log line.
type LogEntry struct {
	ID          string                 `json:"id" bson:"_id"`
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
	Level       string                 `json:"level" bson:"level"`
	Message     string                 `json:"message" bson:"message"`
	Source      string                 `json:"source" bson:"source"`
	Server      string                 `json:"server,omitempty" bson:"server,omitempty"`
	Category    string                 `json:"category,omitempty" bson:"category,omitempty"`
	StackTrace  string                 `json:"stackTrace,omitempty" bson:"stackTrace,omitempty"`
	Fingerprint string                 `json:"fingerprint" bson:"fingerprint"`
	FamilyID    string                 `json:"familyId" bson:"familyId"`
	Tags        []string               `json:"tags,omitempty" bson:"tags,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty" bson:"meta,omitempty"`
}

// ErrorFamily groups similar log entries together.
type ErrorFamily struct {
	ID              string    `json:"id" bson:"_id"`
	Fingerprint     string    `json:"fingerprint" bson:"fingerprint"`
	Title           string    `json:"title" bson:"title"`
	Category        string    `json:"category" bson:"category"`
	Severity        string    `json:"severity" bson:"severity"`
	Pattern         string    `json:"pattern" bson:"pattern"`
	Source          string    `json:"source" bson:"source"`
	FirstSeenAt     time.Time `json:"firstSeenAt" bson:"firstSeenAt"`
	LastSeenAt      time.Time `json:"lastSeenAt" bson:"lastSeenAt"`
	OccurrenceCount int       `json:"occurrenceCount" bson:"occurrenceCount"`
	SampleMessages  []string  `json:"sampleMessages,omitempty" bson:"sampleMessages,omitempty"`
	Servers         []string  `json:"servers,omitempty" bson:"servers,omitempty"`
	AILabel         string    `json:"aiLabel,omitempty" bson:"aiLabel,omitempty"`
	AISummary       string    `json:"aiSummary,omitempty" bson:"aiSummary,omitempty"`
	Status          string    `json:"status" bson:"status"`
}

// LogIngestRequest is the API payload for pushing log entries.
type LogIngestRequest struct {
	Entries []LogIngestEntry `json:"entries"`
}

// LogIngestEntry is a single entry in an ingest request.
type LogIngestEntry struct {
	Timestamp  string                 `json:"timestamp,omitempty"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Source     string                 `json:"source"`
	Server     string                 `json:"server,omitempty"`
	StackTrace string                 `json:"stackTrace,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

// LogFamilyStats provides aggregated stats for dashboard display.
type LogFamilyStats struct {
	TotalFamilies  int            `json:"totalFamilies"`
	ActiveFamilies int            `json:"activeFamilies"`
	TotalEntries   int            `json:"totalEntries"`
	CategoryCounts map[string]int `json:"categoryCounts"`
	SeverityCounts map[string]int `json:"severityCounts"`
	TopFamilies    []ErrorFamily  `json:"topFamilies"`
}

// Predefined log categories for AI labeling.
const (
	CategoryDBAuth           = "db_auth"
	CategoryTimeout          = "timeout"
	CategoryThreadExhaustion = "thread_exhaustion"
	CategorySlowQuery        = "slow_query"
	CategoryDatabase         = "database"
	CategoryNetwork          = "network"
	CategoryAppBug           = "app_bug"
	CategoryApplication      = "application"
	CategoryMemory           = "memory"
	CategoryConfig           = "config"
	CategoryPermission       = "permission"
	CategorySecurity         = "security"
	CategoryRateLimit        = "rate_limit"
	CategoryAccessLog        = "access_log"
	CategoryAudit            = "audit"
	CategoryDiskIO           = "disk_io"
	CategoryUnknown          = "unknown"
)

// AllCategories returns all predefined categories.
func AllCategories() []string {
	return []string{
		CategoryDBAuth, CategoryTimeout, CategoryThreadExhaustion,
		CategorySlowQuery, CategoryDatabase, CategoryNetwork, CategoryAppBug,
		CategoryApplication, CategoryMemory, CategoryConfig, CategoryPermission,
		CategorySecurity, CategoryRateLimit, CategoryAccessLog, CategoryAudit,
		CategoryDiskIO, CategoryUnknown,
	}
}

// InferEntryCategory assigns a practical baseline category before optional AI
// enrichment runs. This keeps logs usable even when AI is disabled.
func InferEntryCategory(entry LogEntry) string {
	text := categoryText(entry.Source, entry.Message, entry.StackTrace, strings.Join(entry.Tags, " "))

	switch {
	case anyContains(text, "access denied", "using password", "mysql auth", "db auth"):
		return CategoryDBAuth
	case anyContains(text, "failed password", "brute-force", "break-in", "invalid signature", "jwt signature", "signature verification", "authentication failed"):
		return CategorySecurity
	case anyContains(text, "permission denied", "forbidden", "unauthorized"):
		return CategoryPermission
	case anyContains(text, "no space left", "disk space", "disk full", "ext4-fs", "i/o error", "io error", "disk_io"):
		return CategoryDiskIO
	case anyContains(text, "oomkilled", "out of memory", "heap ", "gc pause", "memory"):
		return CategoryMemory
	case anyContains(text, "pool exhausted", "pending threads", "thread pool", "too many connections"):
		return CategoryThreadExhaustion
	case anyContains(text, "slow query"):
		return CategorySlowQuery
	case anyContains(text, "deadlock", "lock wait", "database lock"):
		return CategoryDatabase
	case anyContains(text, "timeout", "timed out", "deadline exceeded"):
		return CategoryTimeout
	case anyContains(text, "dns", "nxdomain", "econnrefused", "connection refused", "connection reset", "no route to host", "upstream", "http 503", " returned 503", "circuit breaker"):
		return CategoryNetwork
	case anyContains(text, "rate limit", "too many requests", " 429 "):
		return CategoryRateLimit
	case anyContains(text, "certificate", "config", "configuration", "feature flag", "launchdarkly"):
		return CategoryConfig
	case anyContains(text, "panic", "nil pointer", "exception", "crashloopbackoff", "crashed", "stacktrace"):
		return CategoryAppBug
	case anyContains(text, "audit", "role.changed", "user.role", "permission.changed"):
		return CategoryAudit
	case isAccessLog(text):
		return CategoryAccessLog
	case entry.Level == "info" || anyContains(text, "checkout completed", "request completed"):
		return CategoryApplication
	default:
		return CategoryUnknown
	}
}

// InferFamilyCategory applies the same baseline category to an existing family.
func InferFamilyCategory(family ErrorFamily) string {
	text := categoryText(family.Source, family.Title, family.Pattern, strings.Join(family.SampleMessages, " "))

	switch {
	case anyContains(text, "access denied", "using password", "mysql auth", "db auth"):
		return CategoryDBAuth
	case anyContains(text, "failed password", "brute-force", "break-in", "invalid signature", "jwt signature", "signature verification", "authentication failed"):
		return CategorySecurity
	case anyContains(text, "permission denied", "forbidden", "unauthorized"):
		return CategoryPermission
	case anyContains(text, "no space left", "disk space", "disk full", "ext4-fs", "i/o error", "io error", "disk_io"):
		return CategoryDiskIO
	case anyContains(text, "oomkilled", "out of memory", "heap ", "gc pause", "memory"):
		return CategoryMemory
	case anyContains(text, "pool exhausted", "pending threads", "thread pool", "too many connections"):
		return CategoryThreadExhaustion
	case anyContains(text, "slow query"):
		return CategorySlowQuery
	case anyContains(text, "deadlock", "lock wait", "database lock"):
		return CategoryDatabase
	case anyContains(text, "timeout", "timed out", "deadline exceeded"):
		return CategoryTimeout
	case anyContains(text, "dns", "nxdomain", "econnrefused", "connection refused", "connection reset", "no route to host", "upstream", "http 503", " returned 503", "circuit breaker"):
		return CategoryNetwork
	case anyContains(text, "rate limit", "too many requests", " 429 "):
		return CategoryRateLimit
	case anyContains(text, "certificate", "config", "configuration", "feature flag", "launchdarkly"):
		return CategoryConfig
	case anyContains(text, "panic", "nil pointer", "exception", "crashloopbackoff", "crashed", "stacktrace"):
		return CategoryAppBug
	case anyContains(text, "audit", "role.changed", "user.role", "permission.changed"):
		return CategoryAudit
	case isAccessLog(text):
		return CategoryAccessLog
	case anyContains(text, "checkout completed", "request completed"):
		return CategoryApplication
	default:
		return CategoryUnknown
	}
}

func effectiveFamilyCategory(family ErrorFamily) string {
	if family.Category != "" && family.Category != CategoryUnknown {
		return family.Category
	}
	return InferFamilyCategory(family)
}

func categoryText(parts ...string) string {
	return strings.ToLower(strings.Join(parts, " "))
}

func anyContains(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func isAccessLog(text string) bool {
	return strings.Contains(text, " get /") ||
		strings.Contains(text, " post /") ||
		strings.Contains(text, " put /") ||
		strings.Contains(text, " delete /") ||
		strings.Contains(text, " 200 ") ||
		strings.Contains(text, " 201 ") ||
		strings.Contains(text, " 204 ")
}

// --- Fingerprinting ---

var (
	reIP      = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?`)
	reUUID    = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reLongNum = regexp.MustCompile(`\d{10,}`)
	reHex     = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	reDurMS   = regexp.MustCompile(`\d+(\.\d+)?ms`)
	reDurS    = regexp.MustCompile(`\d+(\.\d+)?s`)
	rePort    = regexp.MustCompile(`port \d+`)
	rePID     = regexp.MustCompile(`pid[= ]\d+`)
	reThread  = regexp.MustCompile(`thread[- ]\d+`)
	reBracket = regexp.MustCompile(`\[\d+\]`)
	reLine    = regexp.MustCompile(`line \d+`)
	reFileCol = regexp.MustCompile(`:\d+:`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

// normPatterns are applied in order to normalize variable parts.
var normPatterns = []*regexp.Regexp{
	reIP, reUUID, reLongNum, reHex, reDurMS, reDurS,
	rePort, rePID, reThread, reBracket, reLine, reFileCol,
}

// ComputeFingerprint generates a stable fingerprint for a log message.
func ComputeFingerprint(message, stackTrace, source string) string {
	normalized := message

	for _, re := range normPatterns {
		normalized = re.ReplaceAllString(normalized, "<?>")
	}

	normalized = reSpaces.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)
	normalized = strings.ToLower(normalized)

	// Include stack trace top frame if available
	stackTop := ""
	if stackTrace != "" {
		lines := strings.Split(stackTrace, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				stackTop = trimmed
				break
			}
		}
	}

	input := source + "||" + normalized + "||" + stackTop
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:12])
}

// ExtractPattern generates a human-readable pattern from a message.
func ExtractPattern(message string) string {
	pattern := message
	replacements := []struct {
		re   *regexp.Regexp
		repl string
	}{
		{reIP, "<ip>"},
		{reUUID, "<uuid>"},
		{reLongNum, "<id>"},
		{reDurMS, "<duration>"},
		{reDurS, "<duration>"},
		{rePort, "port <N>"},
	}
	for _, r := range replacements {
		pattern = r.re.ReplaceAllString(pattern, r.repl)
	}
	return pattern
}
