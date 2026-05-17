package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"medics-health-check/backend/internal/util/jsonl"
)

// Repository defines log intelligence persistence operations.
type Repository interface {
	IngestEntries(entries []LogEntry) error
	RecentEntries(source string, limit int) ([]LogEntry, error)
	EntriesByFamily(familyID string, limit int) ([]LogEntry, error)

	GetFamily(id string) (*ErrorFamily, error)
	ListFamilies(status string, limit int) ([]ErrorFamily, error)
	UpdateFamily(family ErrorFamily) error
	FamilyStats() LogFamilyStats

	PruneBefore(cutoff time.Time) error
	TotalEntries() int
}

// FileRepository is a legacy file-backed log Repository. Retained for tests; production uses MongoDB.
type FileRepository struct {
	mu        sync.RWMutex
	dataDir   string
	entries   []LogEntry
	families  map[string]*ErrorFamily // keyed by fingerprint
	familyIdx map[string]*ErrorFamily // keyed by family ID
}

// NewFileRepository creates a legacy file-backed log repository (test-only).
func NewFileRepository(dataDir string) (*FileRepository, error) {
	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs data dir: %w", err)
	}

	repo := &FileRepository{
		dataDir:   logsDir,
		families:  make(map[string]*ErrorFamily),
		familyIdx: make(map[string]*ErrorFamily),
	}

	// Load existing data
	var err error
	repo.entries, err = jsonl.Load[LogEntry](repo.entriesPath())
	if err != nil {
		return nil, fmt.Errorf("load log entries: %w", err)
	}

	families, err := jsonl.Load[ErrorFamily](repo.familiesPath())
	if err != nil {
		return nil, fmt.Errorf("load error families: %w", err)
	}
	for i := range families {
		f := &families[i]
		repo.families[f.Fingerprint] = f
		repo.familyIdx[f.ID] = f
	}

	return repo, nil
}

func (r *FileRepository) entriesPath() string {
	return filepath.Join(r.dataDir, "log_entries.jsonl")
}

func (r *FileRepository) familiesPath() string {
	return filepath.Join(r.dataDir, "error_families.jsonl")
}

// IngestEntries processes and stores log entries, updating error families.
func (r *FileRepository) IngestEntries(entries []LogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range entries {
		entry := &entries[i]

		// Compute fingerprint if not set
		if entry.Fingerprint == "" {
			entry.Fingerprint = ComputeFingerprint(entry.Message, entry.StackTrace, entry.Source)
		}
		if entry.Category == "" {
			entry.Category = InferEntryCategory(*entry)
		}

		// Find or create error family
		family, exists := r.families[entry.Fingerprint]
		if !exists {
			family = &ErrorFamily{
				ID:              fmt.Sprintf("fam-%s", entry.Fingerprint[:16]),
				Fingerprint:     entry.Fingerprint,
				Title:           truncate(entry.Message, 120),
				Pattern:         ExtractPattern(entry.Message),
				Source:          entry.Source,
				FirstSeenAt:     entry.Timestamp,
				LastSeenAt:      entry.Timestamp,
				OccurrenceCount: 0,
				Status:          "active",
				Severity:        levelToSeverity(entry.Level),
				Category:        entry.Category,
			}
			r.families[entry.Fingerprint] = family
			r.familyIdx[family.ID] = family
		}
		if (family.Category == "" || family.Category == CategoryUnknown) && entry.Category != "" {
			family.Category = entry.Category
		}

		// Update family stats
		family.OccurrenceCount++
		if entry.Timestamp.After(family.LastSeenAt) {
			family.LastSeenAt = entry.Timestamp
		}
		if entry.Timestamp.Before(family.FirstSeenAt) {
			family.FirstSeenAt = entry.Timestamp
		}

		// Track sample messages (up to 5)
		if len(family.SampleMessages) < 5 {
			family.SampleMessages = append(family.SampleMessages, truncate(entry.Message, 200))
		}

		// Track affected servers
		if entry.Server != "" && !contains(family.Servers, entry.Server) {
			family.Servers = append(family.Servers, entry.Server)
		}

		// Assign family ID to entry
		entry.FamilyID = family.ID

		// Persist entry
		r.entries = append(r.entries, *entry)
		if err := jsonl.Append(r.entriesPath(), *entry); err != nil {
			return fmt.Errorf("append log entry: %w", err)
		}
	}

	// Rewrite families (atomic)
	familySlice := make([]ErrorFamily, 0, len(r.families))
	for _, f := range r.families {
		familySlice = append(familySlice, *f)
	}
	if err := jsonl.Rewrite(r.familiesPath(), familySlice); err != nil {
		return fmt.Errorf("rewrite families: %w", err)
	}

	return nil
}

// RecentEntries returns recent entries, optionally filtered by source.
func (r *FileRepository) RecentEntries(source string, limit int) ([]LogEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []LogEntry
	for i := len(r.entries) - 1; i >= 0 && len(result) < limit; i-- {
		e := r.entries[i]
		if source == "" || e.Source == source {
			result = append(result, e)
		}
	}
	return result, nil
}

// EntriesByFamily returns entries belonging to a specific family.
func (r *FileRepository) EntriesByFamily(familyID string, limit int) ([]LogEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []LogEntry
	for i := len(r.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if r.entries[i].FamilyID == familyID {
			result = append(result, r.entries[i])
		}
	}
	return result, nil
}

// GetFamily returns a single error family by ID.
func (r *FileRepository) GetFamily(id string) (*ErrorFamily, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, ok := r.familyIdx[id]
	if !ok {
		return nil, fmt.Errorf("family not found: %s", id)
	}
	return f, nil
}

// ListFamilies returns families sorted by last seen (newest first), optionally filtered by status.
func (r *FileRepository) ListFamilies(status string, limit int) ([]ErrorFamily, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ErrorFamily
	for _, f := range r.families {
		if status != "" && f.Status != status {
			continue
		}
		copy := *f
		copy.Category = effectiveFamilyCategory(copy)
		result = append(result, copy)
	}

	// Sort by last seen (newest first)
	sortFamilies(result)

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// UpdateFamily updates a family (used by AI categorization).
func (r *FileRepository) UpdateFamily(family ErrorFamily) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.familyIdx[family.ID]
	if !ok {
		return fmt.Errorf("family not found: %s", family.ID)
	}

	// Update in-place
	*existing = family
	r.families[family.Fingerprint] = existing

	// Rewrite
	familySlice := make([]ErrorFamily, 0, len(r.families))
	for _, f := range r.families {
		familySlice = append(familySlice, *f)
	}
	return jsonl.Rewrite(r.familiesPath(), familySlice)
}

// FamilyStats returns aggregated stats about error families.
func (r *FileRepository) FamilyStats() LogFamilyStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := LogFamilyStats{
		CategoryCounts: make(map[string]int),
		SeverityCounts: make(map[string]int),
	}

	stats.TotalEntries = len(r.entries)

	for _, f := range r.families {
		stats.TotalFamilies++
		if f.Status == "active" {
			stats.ActiveFamilies++
		}
		stats.CategoryCounts[effectiveFamilyCategory(*f)]++
		stats.SeverityCounts[f.Severity]++
	}

	// Top 10 families by occurrence
	families, _ := r.listFamiliesUnsafe("active", 10)
	stats.TopFamilies = families

	return stats
}

func (r *FileRepository) listFamiliesUnsafe(status string, limit int) ([]ErrorFamily, error) {
	var result []ErrorFamily
	for _, f := range r.families {
		if status != "" && f.Status != status {
			continue
		}
		copy := *f
		copy.Category = effectiveFamilyCategory(copy)
		result = append(result, copy)
	}
	sortFamilies(result)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// PruneBefore removes entries older than cutoff.
func (r *FileRepository) PruneBefore(cutoff time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var kept []LogEntry
	for _, e := range r.entries {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
		}
	}
	r.entries = kept

	if err := jsonl.Rewrite(r.entriesPath(), kept); err != nil {
		return fmt.Errorf("rewrite entries after prune: %w", err)
	}

	// Recalculate family counts
	countMap := make(map[string]int)
	for _, e := range r.entries {
		countMap[e.FamilyID]++
	}

	// Remove families with zero entries
	for fp, f := range r.families {
		if countMap[f.ID] == 0 {
			delete(r.families, fp)
			delete(r.familyIdx, f.ID)
		} else {
			f.OccurrenceCount = countMap[f.ID]
		}
	}

	familySlice := make([]ErrorFamily, 0, len(r.families))
	for _, f := range r.families {
		familySlice = append(familySlice, *f)
	}
	return jsonl.Rewrite(r.familiesPath(), familySlice)
}

// TotalEntries returns total ingested log entry count.
func (r *FileRepository) TotalEntries() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// --- helpers ---

func sortFamilies(families []ErrorFamily) {
	// Simple insertion sort (usually < 100 families)
	for i := 1; i < len(families); i++ {
		for j := i; j > 0 && families[j].LastSeenAt.After(families[j-1].LastSeenAt); j-- {
			families[j], families[j-1] = families[j-1], families[j]
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func levelToSeverity(level string) string {
	switch level {
	case "error", "fatal", "panic":
		return "critical"
	case "warn", "warning":
		return "warning"
	default:
		return "info"
	}
}
