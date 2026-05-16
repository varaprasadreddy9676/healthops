package automation

import (
	"context"
	"path/filepath"
	"testing"

	"medics-health-check/backend/internal/monitoring"
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
