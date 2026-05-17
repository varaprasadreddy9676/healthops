package rca

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockSignalSource implements SignalSource for testing.
type mockSignalSource struct {
	results []CheckResultRef
}

func (m *mockSignalSource) RecentResults(checkID string, limit int) []CheckResultRef {
	var filtered []CheckResultRef
	for _, r := range m.results {
		if r.CheckID == checkID {
			filtered = append(filtered, r)
			if len(filtered) >= limit {
				break
			}
		}
	}
	return filtered
}

func (m *mockSignalSource) AllRecentResults(since time.Time, limit int) []CheckResultRef {
	var filtered []CheckResultRef
	for _, r := range m.results {
		if !r.Timestamp.Before(since) {
			filtered = append(filtered, r)
			if len(filtered) >= limit {
				break
			}
		}
	}
	return filtered
}

func TestCollector_CollectContext(t *testing.T) {
	now := time.Now().UTC()

	source := &mockSignalSource{
		results: []CheckResultRef{
			{CheckID: "inc-1", Name: "api-health", Type: "api", Status: "healthy", DurationMs: 45, Timestamp: now.Add(-10 * time.Minute), Metrics: map[string]float64{"statusCode": 200}},
			{CheckID: "inc-1", Name: "api-health", Type: "api", Status: "healthy", DurationMs: 52, Timestamp: now.Add(-8 * time.Minute), Metrics: map[string]float64{"statusCode": 200}},
			{CheckID: "inc-1", Name: "api-health", Type: "api", Status: "warning", DurationMs: 1200, Timestamp: now.Add(-5 * time.Minute), Metrics: map[string]float64{"statusCode": 200}},
			{CheckID: "inc-1", Name: "api-health", Type: "api", Status: "critical", DurationMs: 5000, Timestamp: now.Add(-3 * time.Minute), Metrics: map[string]float64{"statusCode": 503}},
			{CheckID: "inc-1", Name: "api-health", Type: "api", Status: "critical", DurationMs: 5000, Timestamp: now.Add(-1 * time.Minute), Metrics: map[string]float64{"statusCode": 503}},
		},
	}

	collector := NewCollector(source)

	incident := IncidentRef{
		ID:        "inc-1",
		CheckName: "api-health",
		Severity:  "critical",
		Status:    "open",
		StartedAt: now.Add(-5 * time.Minute),
		Message:   "API health check failed with status 503",
	}

	ctx := collector.CollectContext(incident, 15*time.Minute)

	if ctx.IncidentID != "inc-1" {
		t.Errorf("expected incidentID 'inc-1', got %q", ctx.IncidentID)
	}
	if ctx.CheckName != "api-health" {
		t.Errorf("expected checkName 'api-health', got %q", ctx.CheckName)
	}
	if len(ctx.Signals) == 0 {
		t.Error("expected at least one signal series")
	}
	if len(ctx.RecentEvents) == 0 {
		t.Error("expected at least one timeline event")
	}

	// Check that latency signal is detected
	var foundLatency bool
	for _, s := range ctx.Signals {
		if s.Name == "latencyMs" {
			foundLatency = true
			if s.Trend != "rising" && s.Trend != "spike" {
				t.Errorf("expected latency trend 'rising' or 'spike', got %q", s.Trend)
			}
			break
		}
	}
	if !foundLatency {
		t.Error("expected latencyMs signal series")
	}
}

func TestCollector_DetectTrend(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected string
	}{
		{
			name:     "rising values",
			values:   []float64{10, 12, 14, 16, 18, 20, 22, 24, 26},
			expected: "rising",
		},
		{
			name:     "stable values",
			values:   []float64{50, 52, 48, 51, 49, 50, 51, 49, 50},
			expected: "stable",
		},
		{
			name:     "spike pattern",
			values:   []float64{10, 10, 10, 10, 10, 10, 10, 100, 10},
			expected: "spike",
		},
		{
			name:     "falling values",
			values:   []float64{100, 90, 80, 70, 60, 50, 40, 30, 20},
			expected: "falling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			points := make([]SignalPoint, len(tt.values))
			for i, v := range tt.values {
				points[i] = SignalPoint{
					Timestamp: now.Add(time.Duration(i) * time.Minute),
					Value:     v,
				}
			}
			got := detectTrend(points)
			if got != tt.expected {
				t.Errorf("detectTrend() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAnalyzer_Persistence(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "rca_reports.jsonl")

	source := &mockSignalSource{}
	collector := NewCollector(source)

	repo, err := NewFileReportRepository(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	analyzer, err := NewAnalyzer(collector, nil, repo, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Manually persist a report
	report := RCAReport{
		ID:         "rca-test-1",
		IncidentID: "inc-1",
		CreatedAt:  time.Now().UTC(),
		Status:     "complete",
		Summary:    "Test analysis",
		Hypotheses: []RCAHypothesis{
			{Rank: 1, Title: "Memory leak", Confidence: 0.85, Category: "resource"},
		},
	}
	analyzer.persist(report)

	// Reload from disk
	repo2, err := NewFileReportRepository(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	analyzer2, err := NewAnalyzer(collector, nil, repo2, nil)
	if err != nil {
		t.Fatal(err)
	}

	loaded := analyzer2.GetReport("rca-test-1")
	if loaded == nil {
		t.Fatal("expected report to be persisted and loaded")
	}
	if loaded.Summary != "Test analysis" {
		t.Errorf("expected summary 'Test analysis', got %q", loaded.Summary)
	}
	if len(loaded.Hypotheses) != 1 {
		t.Fatalf("expected 1 hypothesis, got %d", len(loaded.Hypotheses))
	}
	if loaded.Hypotheses[0].Title != "Memory leak" {
		t.Errorf("expected hypothesis title 'Memory leak', got %q", loaded.Hypotheses[0].Title)
	}
}

func TestParseRCAResponse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantSummary    string
		wantHypotheses int
		wantErr        bool
	}{
		{
			name:           "valid JSON",
			input:          `{"summary":"DB connection pool exhausted","hypotheses":[{"rank":1,"title":"Connection leak","description":"App not closing connections","confidence":0.9,"category":"database","evidence":["connection count rising"],"suggestion":"Fix connection pooling"}]}`,
			wantSummary:    "DB connection pool exhausted",
			wantHypotheses: 1,
		},
		{
			name:           "JSON in code block",
			input:          "Here's my analysis:\n```json\n{\"summary\":\"Network timeout\",\"hypotheses\":[{\"rank\":1,\"title\":\"DNS failure\",\"description\":\"DNS not resolving\",\"confidence\":0.7,\"category\":\"network\",\"evidence\":[\"timeout spike\"],\"suggestion\":\"Check DNS\"}]}\n```",
			wantSummary:    "Network timeout",
			wantHypotheses: 1,
		},
		{
			name:    "invalid JSON",
			input:   "I think the problem is...",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hypotheses, summary, err := parseRCAResponse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if summary != tt.wantSummary {
				t.Errorf("summary = %q, want %q", summary, tt.wantSummary)
			}
			if len(hypotheses) != tt.wantHypotheses {
				t.Errorf("hypotheses count = %d, want %d", len(hypotheses), tt.wantHypotheses)
			}
		})
	}
}

func TestComputeStats(t *testing.T) {
	points := []SignalPoint{
		{Value: 10}, {Value: 20}, {Value: 30}, {Value: 40}, {Value: 50},
	}
	min, max, avg := computeStats(points)
	if min != 10 {
		t.Errorf("min = %f, want 10", min)
	}
	if max != 50 {
		t.Errorf("max = %f, want 50", max)
	}
	if avg != 30 {
		t.Errorf("avg = %f, want 30", avg)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
