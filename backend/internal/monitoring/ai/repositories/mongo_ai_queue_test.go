package repositories

import (
	"context"
	"testing"
	"time"

	"health-ops/backend/internal/monitoring"
	"health-ops/backend/internal/util/mongotest"
)

func newTestAIQueue(t *testing.T) *MongoAIQueue {
	t.Helper()
	client := mongotest.Connect(t, 2*time.Second)
	repo, err := NewMongoAIQueue(client, "healthops_test", "test")
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = repo.queueCollection.Drop(ctx)
	_ = repo.resultsCollection.Drop(ctx)
	return repo
}

func TestMongoAIQueue_EnqueueIdempotent(t *testing.T) {
	repo := newTestAIQueue(t)

	if err := repo.Enqueue("inc-1", "v1"); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if err := repo.Enqueue("inc-1", "v1"); err != nil {
		t.Fatalf("duplicate enqueue should be no-op: %v", err)
	}

	pending, _ := repo.ListPendingItems(10)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending item, got %d", len(pending))
	}
}

func TestMongoAIQueue_ClaimPendingTransitions(t *testing.T) {
	repo := newTestAIQueue(t)

	_ = repo.Enqueue("inc-1", "v1")
	time.Sleep(2 * time.Millisecond)
	_ = repo.Enqueue("inc-2", "v1")

	claimed, err := repo.ClaimPending(10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed, got %d", len(claimed))
	}
	for _, c := range claimed {
		if c.Status != "processing" {
			t.Fatalf("expected processing, got %s for %s", c.Status, c.IncidentID)
		}
		if c.ClaimedAt == nil {
			t.Fatalf("ClaimedAt nil for %s", c.IncidentID)
		}
	}

	// pending list should be empty after claim
	pending, _ := repo.ListPendingItems(10)
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after claim, got %d", len(pending))
	}

	// second claim returns nothing
	again, _ := repo.ClaimPending(10)
	if len(again) != 0 {
		t.Fatalf("expected 0 on second claim, got %d", len(again))
	}
}

func TestMongoAIQueue_ClaimPendingRespectsLimit(t *testing.T) {
	repo := newTestAIQueue(t)
	for i := 0; i < 5; i++ {
		_ = repo.Enqueue("inc-"+string(rune('a'+i)), "v1")
		time.Sleep(time.Millisecond)
	}

	claimed, err := repo.ClaimPending(3)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 3 {
		t.Fatalf("expected 3 claimed, got %d", len(claimed))
	}
}

func TestMongoAIQueue_CompleteRecordsResult(t *testing.T) {
	repo := newTestAIQueue(t)
	_ = repo.Enqueue("inc-1", "v1")
	_, _ = repo.ClaimPending(10)

	result := monitoring.AIAnalysisResult{
		Analysis:    "looks like a connection storm",
		Suggestions: []string{"raise max_connections", "scale read replicas"},
		Severity:    "high",
	}
	if err := repo.Complete("inc-1", result); err != nil {
		t.Fatalf("complete: %v", err)
	}

	results := repo.GetResults("inc-1")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Analysis != result.Analysis {
		t.Fatalf("analysis mismatch: %s", results[0].Analysis)
	}
	if results[0].IncidentID != "inc-1" {
		t.Fatalf("incident id not set on result: %s", results[0].IncidentID)
	}
	if results[0].CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be defaulted")
	}

	// queue item now in completed state
	all := repo.AllItems()
	if len(all) != 1 || all[0].Status != "completed" {
		t.Fatalf("expected completed item, got %+v", all)
	}
}

func TestMongoAIQueue_CompleteMissingErrors(t *testing.T) {
	repo := newTestAIQueue(t)
	err := repo.Complete("missing", monitoring.AIAnalysisResult{Analysis: "x"})
	if err == nil {
		t.Fatal("expected error completing missing item")
	}
}

func TestMongoAIQueue_FailTransitions(t *testing.T) {
	repo := newTestAIQueue(t)
	_ = repo.Enqueue("inc-fail", "v1")
	_, _ = repo.ClaimPending(10)

	if err := repo.Fail("inc-fail", "provider rate-limited"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	all := repo.AllItems()
	if len(all) != 1 {
		t.Fatalf("expected 1 item, got %d", len(all))
	}
	if all[0].Status != "failed" {
		t.Fatalf("expected failed, got %s", all[0].Status)
	}
	if all[0].LastError != "provider rate-limited" {
		t.Fatalf("unexpected error: %s", all[0].LastError)
	}
}

func TestMongoAIQueue_EnqueueReplacesTerminalItem(t *testing.T) {
	repo := newTestAIQueue(t)
	_ = repo.Enqueue("inc-redo", "v1")
	_, _ = repo.ClaimPending(10)
	_ = repo.Fail("inc-redo", "transient")

	// re-enqueue should succeed and create a fresh pending item
	if err := repo.Enqueue("inc-redo", "v2"); err != nil {
		t.Fatalf("re-enqueue after fail: %v", err)
	}

	pending, _ := repo.ListPendingItems(10)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].PromptVersion != "v2" {
		t.Fatalf("expected v2, got %s", pending[0].PromptVersion)
	}
}

func TestMongoAIQueue_AllResultsOrdering(t *testing.T) {
	repo := newTestAIQueue(t)

	for _, id := range []string{"i1", "i2", "i3"} {
		_ = repo.Enqueue(id, "v1")
		_, _ = repo.ClaimPending(10)
		_ = repo.Complete(id, monitoring.AIAnalysisResult{Analysis: id})
		time.Sleep(2 * time.Millisecond)
	}

	results := repo.AllResults(10)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Newest first
	if results[0].IncidentID != "i3" {
		t.Fatalf("expected newest first (i3), got %s", results[0].IncidentID)
	}

	limited := repo.AllResults(2)
	if len(limited) != 2 {
		t.Fatalf("expected limit 2, got %d", len(limited))
	}
}

func TestMongoAIQueue_PruneBefore(t *testing.T) {
	repo := newTestAIQueue(t)

	// Manually insert old + new items
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oldItem := aiQueueDoc{
		ID: "inc-old",
		Item: monitoring.AIQueueItem{
			IncidentID:    "inc-old",
			PromptVersion: "v1",
			Status:        "completed",
			CreatedAt:     time.Now().UTC().Add(-10 * 24 * time.Hour),
		},
	}
	if _, err := repo.queueCollection.InsertOne(ctx, oldItem); err != nil {
		t.Fatalf("insert old: %v", err)
	}

	_ = repo.Enqueue("inc-new", "v1")

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	if err := repo.PruneBefore(cutoff); err != nil {
		t.Fatalf("prune: %v", err)
	}

	all := repo.AllItems()
	if len(all) != 1 || all[0].IncidentID != "inc-new" {
		t.Fatalf("expected only inc-new to survive, got %+v", all)
	}
}
