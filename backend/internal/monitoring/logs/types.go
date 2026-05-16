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
	ID          string            `json:"id" bson:"_id"`
	Timestamp   time.Time         `json:"timestamp" bson:"timestamp"`
	Level       string            `json:"level" bson:"level"`
	Message     string            `json:"message" bson:"message"`
	Source      string            `json:"source" bson:"source"`
	Server      string            `json:"server,omitempty" bson:"server,omitempty"`
	Category    string            `json:"category,omitempty" bson:"category,omitempty"`
	StackTrace  string            `json:"stackTrace,omitempty" bson:"stackTrace,omitempty"`
	Fingerprint string            `json:"fingerprint" bson:"fingerprint"`
	FamilyID    string            `json:"familyId" bson:"familyId"`
	Tags        []string          `json:"tags,omitempty" bson:"tags,omitempty"`
	Meta        map[string]string `json:"meta,omitempty" bson:"meta,omitempty"`
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
	Timestamp  string            `json:"timestamp,omitempty"`
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Source     string            `json:"source"`
	Server     string            `json:"server,omitempty"`
	StackTrace string            `json:"stackTrace,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
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
	CategoryNetwork          = "network"
	CategoryAppBug           = "app_bug"
	CategoryMemory           = "memory"
	CategoryConfig           = "config"
	CategoryPermission       = "permission"
	CategoryDiskIO           = "disk_io"
	CategoryUnknown          = "unknown"
)

// AllCategories returns all predefined categories.
func AllCategories() []string {
	return []string{
		CategoryDBAuth, CategoryTimeout, CategoryThreadExhaustion,
		CategorySlowQuery, CategoryNetwork, CategoryAppBug,
		CategoryMemory, CategoryConfig, CategoryPermission,
		CategoryDiskIO, CategoryUnknown,
	}
}

// --- Fingerprinting ---

var (
	reIP       = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?`)
	reUUID     = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reLongNum  = regexp.MustCompile(`\d{10,}`)
	reHex      = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	reDurMS    = regexp.MustCompile(`\d+(\.\d+)?ms`)
	reDurS     = regexp.MustCompile(`\d+(\.\d+)?s`)
	rePort     = regexp.MustCompile(`port \d+`)
	rePID      = regexp.MustCompile(`pid[= ]\d+`)
	reThread   = regexp.MustCompile(`thread[- ]\d+`)
	reBracket  = regexp.MustCompile(`\[\d+\]`)
	reLine     = regexp.MustCompile(`line \d+`)
	reFileCol  = regexp.MustCompile(`:\d+:`)
	reSpaces   = regexp.MustCompile(`\s+`)
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
