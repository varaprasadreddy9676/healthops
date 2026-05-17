package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// MaintenanceWindow defines a planned downtime suppression window.
type MaintenanceWindow struct {
	ID          string    `json:"id" bson:"id"`
	Name        string    `json:"name" bson:"name"`
	Description string    `json:"description,omitempty" bson:"description,omitempty"`
	StartTime   time.Time `json:"startTime" bson:"startTime"`
	EndTime     time.Time `json:"endTime" bson:"endTime"`

	// Scope: which checks are suppressed. All empty = all checks.
	CheckIDs []string `json:"checkIds,omitempty" bson:"checkIds,omitempty"`
	Tags     []string `json:"tags,omitempty" bson:"tags,omitempty"`
	Servers  []string `json:"servers,omitempty" bson:"servers,omitempty"`

	// Recurring schedule (optional). If set, the window repeats.
	Recurring      bool   `json:"recurring,omitempty" bson:"recurring,omitempty"`
	RecurrenceRule string `json:"recurrenceRule,omitempty" bson:"recurrenceRule,omitempty"` // "daily", "weekly", "monthly"

	// State
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	CreatedBy string    `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
	Enabled   bool      `json:"enabled" bson:"enabled"`
}

// IsActive returns whether the window is currently active.
// For one-shot windows, checks if now is between StartTime and EndTime.
// For recurring windows, projects the original window's time-of-day onto the
// current period (daily/weekly/monthly) and checks if now falls inside.
func (mw *MaintenanceWindow) IsActive(now time.Time) bool {
	if !mw.Enabled {
		return false
	}
	if mw.StartTime.IsZero() || mw.EndTime.IsZero() {
		return false
	}
	// One-shot
	if !mw.Recurring {
		return !now.Before(mw.StartTime) && now.Before(mw.EndTime)
	}
	// Recurring — project original time-of-day onto current period
	occStart, occEnd := mw.currentOccurrence(now)
	if occStart.IsZero() {
		return false
	}
	return !now.Before(occStart) && now.Before(occEnd)
}

// currentOccurrence returns the start and end times of the occurrence of this
// recurring window that contains or most recently ended before `now`. Returns
// zero times if the recurrence rule is unrecognized or the window has not yet
// started.
func (mw *MaintenanceWindow) currentOccurrence(now time.Time) (time.Time, time.Time) {
	if now.Before(mw.StartTime) {
		return time.Time{}, time.Time{}
	}
	duration := mw.EndTime.Sub(mw.StartTime)
	if duration <= 0 {
		return time.Time{}, time.Time{}
	}
	loc := mw.StartTime.Location()
	if loc == nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	localStart := mw.StartTime.In(loc)

	switch strings.ToLower(mw.RecurrenceRule) {
	case "daily":
		occStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(),
			localStart.Hour(), localStart.Minute(), localStart.Second(), localStart.Nanosecond(), loc)
		// If occStart is in the future, use yesterday's occurrence
		if occStart.After(localNow) {
			occStart = occStart.AddDate(0, 0, -1)
		}
		return occStart, occStart.Add(duration)
	case "weekly":
		// Match the same weekday as the original StartTime
		daysDiff := int(localNow.Weekday() - localStart.Weekday())
		if daysDiff < 0 {
			daysDiff += 7
		}
		occStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day()-daysDiff,
			localStart.Hour(), localStart.Minute(), localStart.Second(), localStart.Nanosecond(), loc)
		if occStart.After(localNow) {
			occStart = occStart.AddDate(0, 0, -7)
		}
		return occStart, occStart.Add(duration)
	case "monthly":
		// Match same day-of-month as original StartTime. The original day may
		// not exist in every month (e.g. day 31 in Feb), so we re-clamp per
		// candidate month rather than relying on AddDate normalization, which
		// rolls overflow into the next month and produces wrong dates.
		occStart := buildMonthlyOccurrence(localNow.Year(), int(localNow.Month()), localStart, loc)
		if occStart.After(localNow) {
			year, month := localNow.Year(), int(localNow.Month())-1
			if month == 0 {
				year, month = year-1, 12
			}
			occStart = buildMonthlyOccurrence(year, month, localStart, loc)
		}
		return occStart, occStart.Add(duration)
	}
	return time.Time{}, time.Time{}
}

// buildMonthlyOccurrence returns the start time of the monthly occurrence
// within (year, month) projected from `localStart`'s day-of-month and
// time-of-day. The day is clamped to that month's last day so an original
// day-of-31 produces Feb 28/29 in February, etc.
func buildMonthlyOccurrence(year, month int, localStart time.Time, loc *time.Location) time.Time {
	lastDay := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, loc).Day()
	day := localStart.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, time.Month(month), day,
		localStart.Hour(), localStart.Minute(), localStart.Second(), localStart.Nanosecond(), loc)
}

// CoversCheck returns whether this window suppresses the given check.
func (mw *MaintenanceWindow) CoversCheck(check CheckConfig) bool {
	// If no scope is specified, covers all checks
	if len(mw.CheckIDs) == 0 && len(mw.Tags) == 0 && len(mw.Servers) == 0 {
		return true
	}

	// Check by ID
	for _, id := range mw.CheckIDs {
		if id == check.ID {
			return true
		}
	}

	// Check by tag
	if len(mw.Tags) > 0 && len(check.Tags) > 0 {
		tagSet := make(map[string]bool, len(check.Tags))
		for _, t := range check.Tags {
			tagSet[t] = true
		}
		for _, t := range mw.Tags {
			if tagSet[t] {
				return true
			}
		}
	}

	// Check by server
	for _, s := range mw.Servers {
		if s == check.Server {
			return true
		}
	}

	return false
}

// MaintenanceStore persists and evaluates maintenance windows.
type MaintenanceStore struct {
	mu       sync.RWMutex
	windows  []MaintenanceWindow
	filePath string
}

// NewMaintenanceStore creates a file-backed maintenance store.
func NewMaintenanceStore(filePath string) (*MaintenanceStore, error) {
	store := &MaintenanceStore{
		filePath: filePath,
		windows:  []MaintenanceWindow{},
	}
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load maintenance windows: %w", err)
	}
	return store, nil
}

func (s *MaintenanceStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.windows)
}

func (s *MaintenanceStore) persist() error {
	data, err := json.MarshalIndent(s.windows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Create adds a new maintenance window.
func (s *MaintenanceStore) Create(mw MaintenanceWindow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mw.ID == "" {
		mw.ID = fmt.Sprintf("mw-%d", time.Now().UnixNano())
	}
	if mw.CreatedAt.IsZero() {
		mw.CreatedAt = time.Now().UTC()
	}

	s.windows = append(s.windows, mw)
	return s.persist()
}

// Update modifies an existing maintenance window.
func (s *MaintenanceStore) Update(id string, mutator func(*MaintenanceWindow) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.windows {
		if s.windows[i].ID == id {
			updated := s.windows[i]
			if err := mutator(&updated); err != nil {
				return err
			}
			s.windows[i] = updated
			return s.persist()
		}
	}
	return fmt.Errorf("maintenance window %q not found", id)
}

// Delete removes a maintenance window by ID.
func (s *MaintenanceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.windows {
		if s.windows[i].ID == id {
			s.windows = append(s.windows[:i], s.windows[i+1:]...)
			return s.persist()
		}
	}
	return fmt.Errorf("maintenance window %q not found", id)
}

// Get returns a maintenance window by ID.
func (s *MaintenanceStore) Get(id string) (MaintenanceWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, mw := range s.windows {
		if mw.ID == id {
			return mw, nil
		}
	}
	return MaintenanceWindow{}, fmt.Errorf("maintenance window %q not found", id)
}

// List returns all maintenance windows.
func (s *MaintenanceStore) List() []MaintenanceWindow {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]MaintenanceWindow, len(s.windows))
	copy(out, s.windows)
	return out
}

// ListActive returns only currently active windows.
func (s *MaintenanceStore) ListActive() []MaintenanceWindow {
	now := time.Now().UTC()
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []MaintenanceWindow
	for _, mw := range s.windows {
		if mw.IsActive(now) {
			out = append(out, mw)
		}
	}
	return out
}

// IsCheckInMaintenance returns true if the given check is suppressed by any
// active maintenance window.
func (s *MaintenanceStore) IsCheckInMaintenance(check CheckConfig) bool {
	now := time.Now().UTC()
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, mw := range s.windows {
		if mw.IsActive(now) && mw.CoversCheck(check) {
			return true
		}
	}
	return false
}

// PruneExpired removes non-recurring windows that ended before the cutoff.
func (s *MaintenanceStore) PruneExpired(cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []MaintenanceWindow
	removed := 0
	for _, mw := range s.windows {
		if !mw.Recurring && mw.EndTime.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, mw)
	}

	if removed > 0 {
		s.windows = kept
		if err := s.persist(); err != nil {
			return 0, err
		}
	}
	return removed, nil
}
