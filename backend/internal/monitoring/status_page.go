package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// StatusPageComponentStatus represents component health state on the status page.
type StatusPageComponentStatus string

const (
	ComponentOperational      StatusPageComponentStatus = "operational"
	ComponentDegraded         StatusPageComponentStatus = "degraded_performance"
	ComponentPartialOutage    StatusPageComponentStatus = "partial_outage"
	ComponentMajorOutage      StatusPageComponentStatus = "major_outage"
	ComponentUnderMaintenance StatusPageComponentStatus = "under_maintenance"
)

// StatusPageComponent maps a check or group to a user-facing component.
type StatusPageComponent struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	CheckIDs    []string `json:"checkIds,omitempty"` // mapped health checks
	Tags        []string `json:"tags,omitempty"`     // filter by tags
	Servers     []string `json:"servers,omitempty"`  // filter by servers
	Order       int      `json:"order"`              // display order
	Group       string   `json:"group,omitempty"`    // group name for hierarchy
}

// StatusPageConfig is the configuration for a public status page.
type StatusPageConfig struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"` // "Acme Inc Status"
	Slug            string                `json:"slug"` // URL slug: "acme-status"
	Description     string                `json:"description,omitempty"`
	LogoURL         string                `json:"logoUrl,omitempty"`
	FaviconURL      string                `json:"faviconUrl,omitempty"`
	CustomDomain    string                `json:"customDomain,omitempty"`
	IsPublic        bool                  `json:"isPublic"`      // accessible without auth
	ShowIncidents   bool                  `json:"showIncidents"` // show incident history
	ShowUptime      bool                  `json:"showUptime"`    // show uptime percentages
	UptimeDays      int                   `json:"uptimeDays"`    // days to show (default 90)
	Components      []StatusPageComponent `json:"components"`
	AnnouncementMsg string                `json:"announcement,omitempty"` // pinned banner message
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
}

// StatusPageComponentState is the live state of a component.
type StatusPageComponentState struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Group       string                    `json:"group,omitempty"`
	Status      StatusPageComponentStatus `json:"status"`
	Uptime      float64                   `json:"uptimePercent,omitempty"` // e.g., 99.97
	Latency     int64                     `json:"latencyMs,omitempty"`
	LastChecked string                    `json:"lastChecked,omitempty"`
}

// StatusPageIncident is a simplified incident for the public page.
type StatusPageIncident struct {
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Status     string             `json:"status"` // "investigating", "identified", "monitoring", "resolved"
	Severity   string             `json:"severity"`
	CreatedAt  time.Time          `json:"createdAt"`
	ResolvedAt *time.Time         `json:"resolvedAt,omitempty"`
	Components []string           `json:"affectedComponents,omitempty"`
	Updates    []StatusPageUpdate `json:"updates,omitempty"`
}

// StatusPageUpdate is a timeline entry on an incident.
type StatusPageUpdate struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// StatusPageResponse is the full public status page payload.
type StatusPageResponse struct {
	Page       StatusPageMeta             `json:"page"`
	Status     OverallStatus              `json:"status"`
	Components []StatusPageComponentState `json:"components"`
	Incidents  []StatusPageIncident       `json:"incidents,omitempty"`
	UptimeData []UptimeDayEntry           `json:"uptimeHistory,omitempty"`
}

// StatusPageMeta is the page metadata.
type StatusPageMeta struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	LogoURL      string `json:"logoUrl,omitempty"`
	Announcement string `json:"announcement,omitempty"`
}

// OverallStatus summarizes the page health.
type OverallStatus struct {
	Indicator   string `json:"indicator"`   // "none", "minor", "major", "critical"
	Description string `json:"description"` // "All Systems Operational"
}

// UptimeDayEntry is per-day uptime for the status page graph.
type UptimeDayEntry struct {
	Date   string  `json:"date"` // YYYY-MM-DD
	Uptime float64 `json:"uptimePercent"`
}

// Compile-time check: StatusPageStore implements StatusPageRepository.
var _ StatusPageRepository = (*StatusPageStore)(nil)

// StatusPageStore manages status page configurations.
type StatusPageStore struct {
	mu       sync.RWMutex
	pages    map[string]*StatusPageConfig
	filePath string
	nextID   int
}

// NewStatusPageStore creates or loads the store.
func NewStatusPageStore(filePath string) (*StatusPageStore, error) {
	s := &StatusPageStore{
		pages:    make(map[string]*StatusPageConfig),
		filePath: filePath,
		nextID:   1,
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load status pages: %w", err)
	}
	return s, nil
}

func (s *StatusPageStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var pages []StatusPageConfig
	if err := json.Unmarshal(data, &pages); err != nil {
		return err
	}
	for i := range pages {
		p := pages[i]
		s.pages[p.ID] = &p
		s.nextID++
	}
	return nil
}

func (s *StatusPageStore) save() error {
	pages := make([]StatusPageConfig, 0, len(s.pages))
	for _, p := range s.pages {
		pages = append(pages, *p)
	}
	data, err := json.MarshalIndent(pages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Create adds a new status page.
func (s *StatusPageStore) Create(cfg StatusPageConfig) (*StatusPageConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.Name == "" {
		return nil, fmt.Errorf("status page name is required")
	}
	if cfg.Slug == "" {
		return nil, fmt.Errorf("status page slug is required")
	}

	// Verify slug uniqueness
	for _, existing := range s.pages {
		if existing.Slug == cfg.Slug {
			return nil, fmt.Errorf("slug %q already in use", cfg.Slug)
		}
	}

	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("sp-%d", s.nextID)
		s.nextID++
	}
	now := time.Now().UTC()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	if cfg.UptimeDays == 0 {
		cfg.UptimeDays = 90
	}
	if cfg.Components == nil {
		cfg.Components = []StatusPageComponent{}
	}

	s.pages[cfg.ID] = &cfg
	if err := s.save(); err != nil {
		delete(s.pages, cfg.ID)
		return nil, err
	}
	return &cfg, nil
}

// Get retrieves a status page by ID.
func (s *StatusPageStore) Get(id string) (*StatusPageConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pages[id]
	if !ok {
		return nil, fmt.Errorf("status page %q not found", id)
	}
	copy := *p
	return &copy, nil
}

// GetBySlug retrieves a status page by its URL slug.
func (s *StatusPageStore) GetBySlug(slug string) (*StatusPageConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.pages {
		if p.Slug == slug {
			copy := *p
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("status page with slug %q not found", slug)
}

// List returns all status pages.
func (s *StatusPageStore) List() []StatusPageConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]StatusPageConfig, 0, len(s.pages))
	for _, p := range s.pages {
		result = append(result, *p)
	}
	return result
}

// Update performs a FULL replacement of the status page's mutable fields with
// the values in `update`. All fields are overwritten unconditionally — empty
// strings clear, `false` bools clear, nil component slices become empty.
//
// IMPORTANT: For HTTP PUT handlers exposing partial-update semantics, do NOT
// call this method directly with a struct decoded from a partial JSON body —
// fields the client omitted will be zero-valued and silently overwrite the
// stored values. Use UpdatePartial(id, StatusPageConfigUpdate{...}) instead,
// which uses pointer fields to distinguish "field omitted" from "field set to
// the zero value".
//
// This method is retained for tests and for callers that intentionally want a
// full replacement (e.g. bulk import).
func (s *StatusPageStore) Update(id string, update StatusPageConfig) (*StatusPageConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.pages[id]
	if !ok {
		return nil, fmt.Errorf("status page %q not found", id)
	}

	if update.Slug != "" && update.Slug != existing.Slug {
		// Check uniqueness
		for _, p := range s.pages {
			if p.ID != id && p.Slug == update.Slug {
				return nil, fmt.Errorf("slug %q already in use", update.Slug)
			}
		}
	}

	// Full replacement of mutable fields. Preserve identity (ID, Slug fallback,
	// CreatedAt) and timestamps; everything else comes from `update`.
	existing.Name = update.Name
	if update.Slug != "" {
		existing.Slug = update.Slug
	}
	existing.Description = update.Description
	existing.LogoURL = update.LogoURL
	existing.Components = update.Components
	if update.UptimeDays > 0 {
		existing.UptimeDays = update.UptimeDays
	}
	existing.IsPublic = update.IsPublic
	existing.ShowIncidents = update.ShowIncidents
	existing.ShowUptime = update.ShowUptime
	existing.AnnouncementMsg = update.AnnouncementMsg
	existing.UpdatedAt = time.Now().UTC()

	if err := s.save(); err != nil {
		return nil, err
	}
	copy := *existing
	return &copy, nil
}

// StatusPageConfigUpdate is a partial-update DTO for status page config.
// Pointer bools let us tell "field omitted" apart from "field explicitly
// set to false" so that a PUT without isPublic/showIncidents/showUptime
// preserves the existing values instead of clearing them.
type StatusPageConfigUpdate struct {
	Name            *string               `json:"name,omitempty"`
	Slug            *string               `json:"slug,omitempty"`
	Description     *string               `json:"description,omitempty"`
	LogoURL         *string               `json:"logoUrl,omitempty"`
	Components      []StatusPageComponent `json:"components,omitempty"`
	UptimeDays      *int                  `json:"uptimeDays,omitempty"`
	IsPublic        *bool                 `json:"isPublic,omitempty"`
	ShowIncidents   *bool                 `json:"showIncidents,omitempty"`
	ShowUptime      *bool                 `json:"showUptime,omitempty"`
	AnnouncementMsg *string               `json:"announcementMsg,omitempty"`
}

// UpdatePartial applies only the fields present in the update DTO,
// preserving everything else.
func (s *StatusPageStore) UpdatePartial(id string, update StatusPageConfigUpdate) (*StatusPageConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.pages[id]
	if !ok {
		return nil, fmt.Errorf("status page %q not found", id)
	}

	if update.Name != nil && *update.Name != "" {
		existing.Name = *update.Name
	}
	if update.Slug != nil && *update.Slug != "" && *update.Slug != existing.Slug {
		for _, p := range s.pages {
			if p.ID != id && p.Slug == *update.Slug {
				return nil, fmt.Errorf("slug %q already in use", *update.Slug)
			}
		}
		existing.Slug = *update.Slug
	}
	if update.Description != nil {
		existing.Description = *update.Description
	}
	if update.LogoURL != nil {
		existing.LogoURL = *update.LogoURL
	}
	if update.Components != nil {
		existing.Components = update.Components
	}
	if update.UptimeDays != nil && *update.UptimeDays > 0 {
		existing.UptimeDays = *update.UptimeDays
	}
	if update.IsPublic != nil {
		existing.IsPublic = *update.IsPublic
	}
	if update.ShowIncidents != nil {
		existing.ShowIncidents = *update.ShowIncidents
	}
	if update.ShowUptime != nil {
		existing.ShowUptime = *update.ShowUptime
	}
	if update.AnnouncementMsg != nil {
		existing.AnnouncementMsg = *update.AnnouncementMsg
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := s.save(); err != nil {
		return nil, err
	}
	copy := *existing
	return &copy, nil
}

// Delete removes a status page.
func (s *StatusPageStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pages[id]; !ok {
		return fmt.Errorf("status page %q not found", id)
	}
	delete(s.pages, id)
	return s.save()
}
