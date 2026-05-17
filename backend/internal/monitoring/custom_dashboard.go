package monitoring

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// WidgetType identifies the kind of widget on a custom dashboard.
type WidgetType string

const (
	WidgetCheckList    WidgetType = "check_list"
	WidgetStatusGrid   WidgetType = "status_grid"
	WidgetUptimeChart  WidgetType = "uptime_chart"
	WidgetLatencyChart WidgetType = "latency_chart"
	WidgetIncidents    WidgetType = "incidents"
	WidgetSummary      WidgetType = "summary"
	WidgetMetric       WidgetType = "metric"
	WidgetText         WidgetType = "text"
)

// DashboardWidget represents a single widget on a custom dashboard.
type DashboardWidget struct {
	ID       string                 `json:"id"`
	Type     WidgetType             `json:"type"`
	Title    string                 `json:"title"`
	Position WidgetPosition         `json:"position"`
	Config   map[string]interface{} `json:"config,omitempty"` // widget-specific config
}

// WidgetPosition defines grid placement.
type WidgetPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"w"`
	Height int `json:"h"`
}

// CustomDashboard is a user-defined dashboard with selected widgets.
type CustomDashboard struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	IsDefault   bool              `json:"isDefault,omitempty"`
	Visibility  string            `json:"visibility"`         // "private", "team", "public"
	CheckIDs    []string          `json:"checkIds,omitempty"` // filter to specific checks
	Tags        []string          `json:"tags,omitempty"`     // filter by tags
	Servers     []string          `json:"servers,omitempty"`  // filter by servers
	Widgets     []DashboardWidget `json:"widgets"`
	RefreshSec  int               `json:"refreshSeconds,omitempty"` // auto-refresh interval
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// DashboardData is the live data payload for a custom dashboard.
type DashboardData struct {
	Dashboard   CustomDashboard          `json:"dashboard"`
	Checks      []CheckConfig            `json:"checks"`
	Results     map[string][]CheckResult `json:"results"` // checkID -> recent results
	Summary     Summary                  `json:"summary"`
	Incidents   []Incident               `json:"incidents,omitempty"`
	GeneratedAt time.Time                `json:"generatedAt"`
}

// CustomDashboardStore manages persisted custom dashboards.
type CustomDashboardStore struct {
	mu         sync.RWMutex
	dashboards map[string]*CustomDashboard
	filePath   string
	nextID     int
}

// NewCustomDashboardStore creates or loads a dashboard store from disk.
func NewCustomDashboardStore(filePath string) (*CustomDashboardStore, error) {
	s := &CustomDashboardStore{
		dashboards: make(map[string]*CustomDashboard),
		filePath:   filePath,
		nextID:     1,
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load dashboards: %w", err)
	}
	return s, nil
}

func (s *CustomDashboardStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var dashboards []CustomDashboard
	if err := json.Unmarshal(data, &dashboards); err != nil {
		return err
	}
	for i := range dashboards {
		d := dashboards[i]
		s.dashboards[d.ID] = &d
		s.nextID++
	}
	return nil
}

func (s *CustomDashboardStore) save() error {
	dashboards := make([]CustomDashboard, 0, len(s.dashboards))
	for _, d := range s.dashboards {
		dashboards = append(dashboards, *d)
	}
	data, err := json.MarshalIndent(dashboards, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Create adds a new custom dashboard.
func (s *CustomDashboardStore) Create(d CustomDashboard) (*CustomDashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.Name == "" {
		return nil, fmt.Errorf("dashboard name is required")
	}

	if d.ID == "" {
		d.ID = fmt.Sprintf("dash-%d", s.nextID)
		s.nextID++
	}
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now

	if d.Visibility == "" {
		d.Visibility = "private"
	}
	if d.Widgets == nil {
		d.Widgets = []DashboardWidget{}
	}

	s.dashboards[d.ID] = &d
	if err := s.save(); err != nil {
		delete(s.dashboards, d.ID)
		return nil, err
	}
	return &d, nil
}

// Get returns a dashboard by ID.
func (s *CustomDashboardStore) Get(id string) (*CustomDashboard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", id)
	}
	copy := *d
	return &copy, nil
}

// List returns all dashboards, optionally filtered by owner.
func (s *CustomDashboardStore) List(owner string) []CustomDashboard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]CustomDashboard, 0, len(s.dashboards))
	for _, d := range s.dashboards {
		if owner == "" || d.Owner == owner || d.Visibility == "public" || d.Visibility == "team" {
			result = append(result, *d)
		}
	}
	return result
}

// Update modifies an existing dashboard.
func (s *CustomDashboardStore) Update(id string, update CustomDashboard) (*CustomDashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", id)
	}

	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Owner != "" || existing.Owner != "" {
		existing.Owner = update.Owner
	}
	if update.Visibility != "" {
		existing.Visibility = update.Visibility
	}
	if update.CheckIDs != nil {
		existing.CheckIDs = update.CheckIDs
	}
	if update.Tags != nil {
		existing.Tags = update.Tags
	}
	if update.Servers != nil {
		existing.Servers = update.Servers
	}
	if update.Widgets != nil {
		existing.Widgets = update.Widgets
	}
	if update.RefreshSec > 0 {
		existing.RefreshSec = update.RefreshSec
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := s.save(); err != nil {
		return nil, err
	}
	copy := *existing
	return &copy, nil
}

// Delete removes a dashboard.
func (s *CustomDashboardStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dashboards[id]; !ok {
		return fmt.Errorf("dashboard %q not found", id)
	}
	delete(s.dashboards, id)
	return s.save()
}

// Duplicate clones a dashboard with a new name.
func (s *CustomDashboardStore) Duplicate(id, newName string) (*CustomDashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %q not found", id)
	}

	dup := *original
	dup.ID = fmt.Sprintf("dash-%d", s.nextID)
	s.nextID++
	dup.Name = newName
	dup.IsDefault = false
	now := time.Now().UTC()
	dup.CreatedAt = now
	dup.UpdatedAt = now

	s.dashboards[dup.ID] = &dup
	if err := s.save(); err != nil {
		delete(s.dashboards, dup.ID)
		return nil, err
	}
	return &dup, nil
}
