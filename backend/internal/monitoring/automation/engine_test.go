package automation

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"health-ops/backend/internal/monitoring"
)

func TestApproveRecordsAuditWithoutExecution(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t, `[{
		"type":"restart",
		"title":"Restart API",
		"description":"Restart the API service",
		"risk":"medium",
		"command":"systemctl restart api",
		"reason":"Service is unhealthy"
	}]`)

	actions, err := engine.Suggest(context.Background(), SuggestRequest{Context: "api unhealthy"})
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions length = %d, want 1", len(actions))
	}

	if err := engine.Approve(actions[0].ID, "operator"); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	approved, ok := engine.GetAction(actions[0].ID)
	if !ok {
		t.Fatal("approved action not found")
	}
	if approved.Status != StatusApproved {
		t.Fatalf("status = %s, want %s", approved.Status, StatusApproved)
	}
	if approved.ApprovedBy != "operator" {
		t.Fatalf("approvedBy = %q, want operator", approved.ApprovedBy)
	}
	if approved.ApprovedAt == nil {
		t.Fatal("approvedAt was not set")
	}
	if approved.ExecutedAt != nil {
		t.Fatalf("executedAt = %v, want nil because approval must not execute commands", approved.ExecutedAt)
	}
	if approved.Result != "approved in audit log; command not executed" {
		t.Fatalf("result = %q", approved.Result)
	}
}

func TestApproveUnknownActionReturnsError(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t, `[]`)
	if err := engine.Approve("act_missing", "operator"); err == nil {
		t.Fatal("expected missing action error")
	}
}

func TestSuggestReusesExistingPendingAction(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t, `[{
		"type":"custom",
		"title":"Inspect process",
		"description":"Verify the process",
		"risk":"low",
		"reason":"Process is missing"
	}]`)

	first, err := engine.Suggest(context.Background(), SuggestRequest{Context: "process missing"})
	if err != nil {
		t.Fatalf("first Suggest failed: %v", err)
	}
	second, err := engine.Suggest(context.Background(), SuggestRequest{Context: "process missing"})
	if err != nil {
		t.Fatalf("second Suggest failed: %v", err)
	}

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("suggestions lengths = %d/%d, want 1/1", len(first), len(second))
	}
	if second[0].ID != first[0].ID {
		t.Fatalf("second suggestion ID = %q, want existing %q", second[0].ID, first[0].ID)
	}
	if got := len(engine.ListActions("")); got != 1 {
		t.Fatalf("ListActions length = %d, want 1", got)
	}
	if got := len(engine.AuditLog()); got != 1 {
		t.Fatalf("AuditLog length = %d, want one suggested audit event", got)
	}
}

func TestSuggestIncludesOpenIncidentsAndUnhealthyChecksWithoutExplicitTarget(t *testing.T) {
	t.Parallel()

	store, err := monitoring.NewFileStore(filepath.Join(t.TempDir(), "state.json"), nil)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	now := time.Now().UTC()
	if err := store.Update(func(state *monitoring.State) error {
		state.Checks = []monitoring.CheckConfig{
			{ID: "check-http", Name: "Python HTTP Demo", Type: "process", Server: "demo-linux-1", Target: "http.server"},
			{ID: "check-api", Name: "Checkout API", Type: "api", Target: "http://demo-api:5000/health"},
		}
		state.Results = []monitoring.CheckResult{
			{
				ID:         "result-http",
				CheckID:    "check-http",
				Name:       "Python HTTP Demo",
				Type:       "process",
				Server:     "demo-linux-1",
				Status:     "critical",
				Healthy:    false,
				Message:    `Process "http.server" not found`,
				DurationMs: 23,
				FinishedAt: now,
			},
			{
				ID:         "result-api",
				CheckID:    "check-api",
				Name:       "Checkout API",
				Type:       "api",
				Status:     "healthy",
				Healthy:    true,
				DurationMs: 12,
				FinishedAt: now,
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	incidentRepo := monitoring.NewMemoryIncidentRepository()
	if err := incidentRepo.CreateIncident(monitoring.Incident{
		ID:        "inc-http",
		CheckID:   "check-http",
		CheckName: "Python HTTP Demo",
		Type:      "process",
		Status:    "open",
		Severity:  "critical",
		Message:   `Rule: Check Down | Details: Process "http.server" not found`,
		StartedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}

	var capturedUser string
	engine := NewEngine(store, incidentRepo, func(_ context.Context, _, userMsg string) (string, error) {
		capturedUser = userMsg
		return `[{"type":"custom","title":"Inspect process","description":"Verify the demo process","risk":"low","reason":"Process is missing"}]`, nil
	}, nil)

	if _, err := engine.Suggest(context.Background(), SuggestRequest{}); err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	for _, want := range []string{
		"Current unhealthy checks:",
		"Python HTTP Demo",
		`Process "http.server" not found`,
		"Open incidents:",
	} {
		if !strings.Contains(capturedUser, want) {
			t.Fatalf("AI context missing %q:\n%s", want, capturedUser)
		}
	}
}

func newTestEngine(t *testing.T, aiResponse string) *Engine {
	t.Helper()

	store, err := monitoring.NewFileStore(filepath.Join(t.TempDir(), "state.json"), nil)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	return NewEngine(store, nil, func(context.Context, string, string) (string, error) {
		return aiResponse, nil
	}, nil)
}
