package evidence

import (
	"context"
	"fmt"
	"testing"
	"time"

	"medics-health-check/backend/internal/monitoring"
)

// --- Mocks ---

type mockStore struct {
	state monitoring.State
}

func (m *mockStore) Snapshot() monitoring.State          { return m.state }
func (m *mockStore) DashboardSnapshot() monitoring.DashboardSnapshot {
	return monitoring.DashboardSnapshot{State: m.state}
}
func (m *mockStore) Update(fn func(*monitoring.State) error) error { return fn(&m.state) }
func (m *mockStore) ReplaceChecks(checks []monitoring.CheckConfig) error {
	m.state.Checks = checks
	return nil
}
func (m *mockStore) UpsertCheck(c monitoring.CheckConfig) error {
	m.state.Checks = append(m.state.Checks, c)
	return nil
}
func (m *mockStore) DeleteCheck(id string) error { return nil }
func (m *mockStore) AppendResults(results []monitoring.CheckResult, retentionDays int) error {
	m.state.Results = append(m.state.Results, results...)
	return nil
}
func (m *mockStore) SetLastRun(t time.Time) error {
	m.state.LastRunAt = t
	return nil
}

type mockIncidentRepo struct {
	incidents []monitoring.Incident
}

func (m *mockIncidentRepo) CreateIncident(inc monitoring.Incident) error {
	m.incidents = append(m.incidents, inc)
	return nil
}
func (m *mockIncidentRepo) UpdateIncident(id string, fn func(*monitoring.Incident) error) error {
	for i := range m.incidents {
		if m.incidents[i].ID == id {
			return fn(&m.incidents[i])
		}
	}
	return fmt.Errorf("not found")
}
func (m *mockIncidentRepo) GetIncident(id string) (monitoring.Incident, error) {
	for _, inc := range m.incidents {
		if inc.ID == id {
			return inc, nil
		}
	}
	return monitoring.Incident{}, fmt.Errorf("not found")
}
func (m *mockIncidentRepo) ListIncidents() ([]monitoring.Incident, error) {
	return m.incidents, nil
}
func (m *mockIncidentRepo) FindOpenIncident(checkID string) (monitoring.Incident, error) {
	for _, inc := range m.incidents {
		if inc.CheckID == checkID && inc.Status == "open" {
			return inc, nil
		}
	}
	return monitoring.Incident{}, nil
}

type mockSnapshotRepo struct {
	snapshots map[string][]monitoring.IncidentSnapshot
}

func (m *mockSnapshotRepo) SaveSnapshots(incidentID string, snaps []monitoring.IncidentSnapshot) error {
	m.snapshots[incidentID] = append(m.snapshots[incidentID], snaps...)
	return nil
}
func (m *mockSnapshotRepo) GetSnapshots(incidentID string) ([]monitoring.IncidentSnapshot, error) {
	return m.snapshots[incidentID], nil
}
func (m *mockSnapshotRepo) PruneBefore(cutoff time.Time) error { return nil }

// --- Tests ---

func TestComputeConfidence_Empty(t *testing.T) {
	evidence := &CollectedEvidence{
		Events:              []SignalEvent{},
		ByCategory:          map[string][]SignalEvent{},
		AvailableCategories: []string{},
		MissingCategories:   []string{"checks", "mysql", "server_metrics"},
	}

	score := ComputeConfidence(evidence)

	if score.Score != 0 {
		t.Errorf("expected score 0 for empty evidence, got %f", score.Score)
	}
	if score.Band != "low" {
		t.Errorf("expected band 'low', got %q", score.Band)
	}
}

func TestComputeConfidence_WithMetricAnomaly(t *testing.T) {
	evidence := &CollectedEvidence{
		Events: []SignalEvent{
			{Type: SignalTypeServer, Severity: "warning", Message: "high CPU"},
			{Type: SignalTypeCheck, Severity: "info", Message: "check ok"},
		},
		ByCategory:          map[string][]SignalEvent{},
		AvailableCategories: []string{"server_metrics", "checks"},
		MissingCategories:   []string{"mysql"},
	}

	score := ComputeConfidence(evidence)

	// Should have metric anomaly (0.2) + evidence count (2/10 * 0.2 = 0.04) = 0.24
	if score.Score < 0.23 || score.Score > 0.25 {
		t.Errorf("expected score ~0.24, got %f", score.Score)
	}
	if !score.Breakdown.HasMetricAnomaly {
		t.Error("expected HasMetricAnomaly to be true")
	}
	if score.Band != "low" {
		t.Errorf("expected band 'low', got %q", score.Band)
	}
}

func TestComputeConfidence_HighEvidence(t *testing.T) {
	events := make([]SignalEvent, 15)
	for i := range events {
		events[i] = SignalEvent{
			Type:     SignalTypeServer,
			Severity: "warning",
			Message:  fmt.Sprintf("event %d", i),
		}
	}
	// Add a past incident
	events = append(events, SignalEvent{
		Type:   SignalTypeCheck,
		Source: "incident_manager",
	})

	evidence := &CollectedEvidence{
		Events:              events,
		ByCategory:          map[string][]SignalEvent{},
		AvailableCategories: []string{"server_metrics", "incident_history"},
		MissingCategories:   []string{},
	}

	score := ComputeConfidence(evidence)

	// metric anomaly (0.2) + similar past (0.2) + evidence count (1.0 * 0.2 = 0.2) = 0.6
	if score.Score < 0.59 || score.Score > 0.61 {
		t.Errorf("expected score ~0.6, got %f", score.Score)
	}
	if score.Band != "medium" {
		t.Errorf("expected band 'medium', got %q", score.Band)
	}
}

func TestRegistry_RegisterAndProvide(t *testing.T) {
	reg := NewRegistry()

	store := &mockStore{
		state: monitoring.State{
			Checks: []monitoring.CheckConfig{
				{ID: "chk1", Name: "test", Type: "api"},
			},
		},
	}

	reg.Register(NewCheckProvider(store))
	reg.Register(NewIncidentHistoryProvider(&mockIncidentRepo{}))

	categories := reg.Categories()
	if len(categories) < 2 {
		t.Errorf("expected at least 2 categories, got %d", len(categories))
	}

	providers := reg.Providers()
	if len(providers) < 2 {
		t.Errorf("expected at least 2 providers, got %d", len(providers))
	}
}

func TestCheckProvider_Collect(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		state: monitoring.State{
			Checks: []monitoring.CheckConfig{
				{ID: "chk1", Name: "API Health", Type: "api", Server: "web-1", Application: "myapp"},
			},
			Results: []monitoring.CheckResult{
				{
					CheckID:    "chk1",
					Status:     "critical",
					Message:    "connection refused",
					FinishedAt: now.Add(-10 * time.Minute),
					DurationMs: 1500,
				},
				{
					CheckID:    "chk1",
					Status:     "healthy",
					Message:    "ok",
					FinishedAt: now.Add(-5 * time.Minute),
					DurationMs: 42,
				},
			},
		},
	}

	provider := NewCheckProvider(store)

	if provider.Category() != "checks" {
		t.Errorf("expected category 'checks', got %q", provider.Category())
	}

	window := TimeWindow{Start: now.Add(-1 * time.Hour), End: now}
	events, err := provider.Collect(context.Background(), "inc-1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// First event should be critical
	if events[0].Severity != "critical" {
		t.Errorf("expected first event severity 'critical', got %q", events[0].Severity)
	}
	if events[0].Host != "web-1" {
		t.Errorf("expected host 'web-1', got %q", events[0].Host)
	}
	if events[0].Service != "myapp" {
		t.Errorf("expected service 'myapp', got %q", events[0].Service)
	}
}

func TestIncidentHistoryProvider_ExcludesCurrentIncident(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockIncidentRepo{
		incidents: []monitoring.Incident{
			{ID: "inc-current", CheckID: "chk1", CheckName: "API", Status: "open", Severity: "critical", StartedAt: now.Add(-30 * time.Minute), Message: "down"},
			{ID: "inc-past", CheckID: "chk1", CheckName: "API", Status: "resolved", Severity: "warning", StartedAt: now.Add(-2 * time.Hour), Message: "was slow"},
		},
	}

	provider := NewIncidentHistoryProvider(repo)
	window := TimeWindow{Start: now.Add(-24 * time.Hour), End: now}
	events, err := provider.Collect(context.Background(), "inc-current", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event (current excluded), got %d", len(events))
	}
	if events[0].Attributes["incidentId"] != "inc-past" {
		t.Errorf("expected past incident, got %s", events[0].Attributes["incidentId"])
	}
}

func TestContextBuilder_CollectAndFormat(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		state: monitoring.State{
			Checks: []monitoring.CheckConfig{
				{ID: "chk1", Name: "API", Type: "api", Server: "web-1"},
			},
			Results: []monitoring.CheckResult{
				{CheckID: "chk1", Status: "critical", Message: "timeout", FinishedAt: now.Add(-5 * time.Minute), DurationMs: 5000},
			},
		},
	}

	registry := NewRegistry()
	registry.Register(NewCheckProvider(store))

	builder := NewContextBuilder(registry, nil)

	window := TimeWindow{Start: now.Add(-1 * time.Hour), End: now}
	evidence, err := builder.Collect(context.Background(), "inc-1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(evidence.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evidence.Events))
	}
	if len(evidence.AvailableCategories) != 1 {
		t.Errorf("expected 1 available category, got %d", len(evidence.AvailableCategories))
	}

	// Test FormatForPrompt
	prompt := builder.FormatForPrompt(evidence)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	if len(prompt) < 50 {
		t.Errorf("expected meaningful prompt content, got %d chars", len(prompt))
	}
}

func TestContextBuilder_CapEvents(t *testing.T) {
	now := time.Now().UTC()

	// Create a store with 60 results
	results := make([]monitoring.CheckResult, 60)
	for i := range results {
		results[i] = monitoring.CheckResult{
			CheckID:    "chk1",
			Status:     "healthy",
			Message:    "ok",
			FinishedAt: now.Add(-time.Duration(i) * time.Minute),
			DurationMs: 42,
		}
	}

	store := &mockStore{
		state: monitoring.State{
			Checks:  []monitoring.CheckConfig{{ID: "chk1", Name: "API", Type: "api"}},
			Results: results,
		},
	}

	registry := NewRegistry()
	registry.Register(NewCheckProvider(store))

	builder := NewContextBuilder(registry, nil)
	builder.SetEvidenceCap(20)

	window := TimeWindow{Start: now.Add(-2 * time.Hour), End: now}
	evidence, err := builder.Collect(context.Background(), "inc-1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(evidence.Events) > 20 {
		t.Errorf("expected at most 20 events after cap, got %d", len(evidence.Events))
	}
	if !evidence.WasCapped {
		t.Error("expected WasCapped to be true")
	}
	if evidence.TotalBeforeCap != 60 {
		t.Errorf("expected TotalBeforeCap=60, got %d", evidence.TotalBeforeCap)
	}
}

func TestBriefGenerator_WithoutAIProvider(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		state: monitoring.State{
			Checks: []monitoring.CheckConfig{
				{ID: "chk1", Name: "API Health", Type: "api"},
			},
			Results: []monitoring.CheckResult{
				{CheckID: "chk1", Status: "critical", Message: "timeout", FinishedAt: now.Add(-5 * time.Minute)},
			},
		},
	}

	incidentRepo := &mockIncidentRepo{
		incidents: []monitoring.Incident{
			{ID: "inc-1", CheckID: "chk1", CheckName: "API Health", Type: "api",
				Status: "open", Severity: "critical", Message: "timeout",
				StartedAt: now.Add(-10 * time.Minute)},
		},
	}

	registry := NewRegistry()
	registry.Register(NewCheckProvider(store))

	builder := NewContextBuilder(registry, nil)
	generator := NewBriefGenerator(builder, incidentRepo, nil)

	brief, err := generator.GenerateBrief(context.Background(), "inc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if brief.IncidentID != "inc-1" {
		t.Errorf("expected incidentID 'inc-1', got %q", brief.IncidentID)
	}
	if brief.LikelyCause == "" {
		t.Error("expected non-empty likely cause")
	}
	if len(brief.NextActions) == 0 {
		t.Error("expected at least one next action")
	}
	if brief.Metadata.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestBriefGenerator_WithMockAI(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		state: monitoring.State{
			Checks: []monitoring.CheckConfig{
				{ID: "chk1", Name: "API Health", Type: "api"},
			},
			Results: []monitoring.CheckResult{
				{CheckID: "chk1", Status: "critical", Message: "connection refused", FinishedAt: now.Add(-5 * time.Minute)},
			},
		},
	}

	incidentRepo := &mockIncidentRepo{
		incidents: []monitoring.Incident{
			{ID: "inc-1", CheckID: "chk1", CheckName: "API Health", Type: "api",
				Status: "open", Severity: "critical", Message: "connection refused",
				StartedAt: now.Add(-10 * time.Minute)},
		},
	}

	registry := NewRegistry()
	registry.Register(NewCheckProvider(store))

	builder := NewContextBuilder(registry, nil)
	generator := NewBriefGenerator(builder, incidentRepo, nil)

	// Mock AI response
	generator.SetAICall(func(ctx context.Context, systemMsg, userMsg string) (string, error) {
		return `{"likelyCause":"API server process crashed due to OOM","impactSummary":"All API consumers experiencing 503 errors","nextActions":["Restart the API server","Increase memory limit","Add OOM monitoring"]}`, nil
	})

	brief, err := generator.GenerateBrief(context.Background(), "inc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if brief.LikelyCause != "API server process crashed due to OOM" {
		t.Errorf("expected parsed likely cause, got %q", brief.LikelyCause)
	}
	if brief.ImpactSummary != "All API consumers experiencing 503 errors" {
		t.Errorf("expected parsed impact summary, got %q", brief.ImpactSummary)
	}
	if len(brief.NextActions) != 3 {
		t.Errorf("expected 3 next actions, got %d", len(brief.NextActions))
	}
	if brief.RawAIResponse == "" {
		t.Error("expected raw AI response to be preserved")
	}
}

func TestBriefGenerator_AIFailureGraceful(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		state: monitoring.State{
			Checks:  []monitoring.CheckConfig{{ID: "chk1", Name: "API", Type: "api"}},
			Results: []monitoring.CheckResult{{CheckID: "chk1", Status: "critical", FinishedAt: now}},
		},
	}

	incidentRepo := &mockIncidentRepo{
		incidents: []monitoring.Incident{
			{ID: "inc-1", CheckID: "chk1", CheckName: "API", Type: "api",
				Status: "open", Severity: "critical", StartedAt: now},
		},
	}

	registry := NewRegistry()
	registry.Register(NewCheckProvider(store))

	builder := NewContextBuilder(registry, nil)
	generator := NewBriefGenerator(builder, incidentRepo, nil)

	// AI call fails
	generator.SetAICall(func(ctx context.Context, systemMsg, userMsg string) (string, error) {
		return "", fmt.Errorf("provider unavailable")
	})

	brief, err := generator.GenerateBrief(context.Background(), "inc-1")
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}

	if brief.LikelyCause == "" {
		t.Error("expected a fallback likely cause")
	}
	if len(brief.NextActions) == 0 {
		t.Error("expected fallback next actions")
	}
}

func TestParseAIBriefResponse_ValidJSON(t *testing.T) {
	response := `{"likelyCause":"disk full","impactSummary":"writes failing","nextActions":["expand disk"]}`
	parsed := parseAIBriefResponse(response)

	if parsed.LikelyCause != "disk full" {
		t.Errorf("expected 'disk full', got %q", parsed.LikelyCause)
	}
	if len(parsed.NextActions) != 1 {
		t.Errorf("expected 1 action, got %d", len(parsed.NextActions))
	}
}

func TestParseAIBriefResponse_MarkdownWrapped(t *testing.T) {
	response := "Here is my analysis:\n```json\n{\"likelyCause\":\"OOM kill\",\"nextActions\":[\"increase memory\"]}\n```\nLet me know if you need more details."
	parsed := parseAIBriefResponse(response)

	if parsed.LikelyCause != "OOM kill" {
		t.Errorf("expected 'OOM kill', got %q", parsed.LikelyCause)
	}
}

func TestParseAIBriefResponse_InvalidJSON(t *testing.T) {
	response := "The server is probably overloaded due to traffic spike"
	parsed := parseAIBriefResponse(response)

	if parsed.LikelyCause == "" {
		t.Error("expected fallback to raw response")
	}
}

func TestMySQLSnapshotProvider_Collect(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockSnapshotRepo{
		snapshots: map[string][]monitoring.IncidentSnapshot{
			"inc-1": {
				{
					IncidentID:   "inc-1",
					SnapshotType: "latest_sample",
					Timestamp:    now.Add(-5 * time.Minute),
					PayloadJSON:  `{"connections":50,"maxConnections":100,"threadsRunning":5,"threadsConnected":50,"slowQueries":2,"connectionUtilization":0.5}`,
				},
				{
					IncidentID:   "inc-1",
					SnapshotType: "processlist",
					Timestamp:    now.Add(-4 * time.Minute),
					PayloadJSON:  `[{"id":1,"user":"root","host":"localhost","db":"mydb","command":"Query","time":5}]`,
				},
			},
		},
	}

	provider := NewMySQLSnapshotProvider(repo)
	window := TimeWindow{Start: now.Add(-1 * time.Hour), End: now}
	events, err := provider.Collect(context.Background(), "inc-1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Check that the first event has a meaningful summary
	if events[0].Message == "" {
		t.Error("expected non-empty message for MySQL sample")
	}
}

func TestBuildTimeline(t *testing.T) {
	evidence := &CollectedEvidence{
		Events: []SignalEvent{
			{Timestamp: time.Now(), Severity: "critical", Message: "connection refused"},
			{Timestamp: time.Now(), Severity: "info", Message: "check passed"},
			{Timestamp: time.Now(), Severity: "warning", Message: "high latency"},
		},
	}

	timeline := buildTimeline(evidence)

	// Should only include critical and warning events
	if len(timeline) != 2 {
		t.Errorf("expected 2 timeline entries (critical + warning), got %d", len(timeline))
	}
}

func TestBuildEvidenceCitations(t *testing.T) {
	now := time.Now().UTC()
	evidence := &CollectedEvidence{
		AvailableCategories: []string{"checks"},
		ByCategory: map[string][]SignalEvent{
			"checks": {
				{ID: "sig_abc", Timestamp: now, Severity: "critical", Message: "connection refused"},
				{ID: "sig_def", Timestamp: now, Severity: "info", Message: "check ok"},
			},
		},
	}

	citations := buildEvidenceCitations(evidence)

	if len(citations) == 0 {
		t.Error("expected at least one citation")
	}

	// Should have a specific citation + a summary citation
	foundSpecific := false
	for _, c := range citations {
		if c.SignalID != "" {
			foundSpecific = true
			if c.Category != "checks" {
				t.Errorf("expected category 'checks', got %q", c.Category)
			}
		}
	}
	if !foundSpecific {
		t.Error("expected at least one specific citation with SignalID")
	}
}
