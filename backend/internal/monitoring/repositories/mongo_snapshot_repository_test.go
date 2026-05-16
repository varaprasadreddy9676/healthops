package repositories

import (
	"context"
	"testing"
	"time"

	"medics-health-check/backend/internal/monitoring"
	"medics-health-check/backend/internal/util/mongotest"
)

func newTestSnapshotRepo(t *testing.T) *MongoSnapshotRepository {
	t.Helper()
	client := mongotest.Connect(t, 2*time.Second)
	repo, err := NewMongoSnapshotRepository(client, "healthops_test", "test")
	if err != nil {
		t.Fatalf("init snapshot repo: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.collection.Drop(ctx); err != nil {
		t.Fatalf("drop collection: %v", err)
	}
	return repo
}

func TestMongoSnapshotRepository_SaveAndGet(t *testing.T) {
	repo := newTestSnapshotRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	snaps := []monitoring.IncidentSnapshot{
		{
			IncidentID:   "inc-1",
			SnapshotType: "latest_sample",
			Timestamp:    now,
			PayloadJSON:  `{"connections": 42}`,
		},
		{
			IncidentID:   "inc-1",
			SnapshotType: "recent_deltas",
			Timestamp:    now.Add(time.Second),
			PayloadJSON:  `{"slowQueries": 5}`,
		},
	}

	if err := repo.SaveSnapshots("inc-1", snaps); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.GetSnapshots("inc-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(got))
	}
	if got[0].SnapshotType != "latest_sample" {
		t.Fatalf("unexpected first snapshot type: %s", got[0].SnapshotType)
	}
	if got[1].SnapshotType != "recent_deltas" {
		t.Fatalf("unexpected second snapshot type: %s", got[1].SnapshotType)
	}
}

func TestMongoSnapshotRepository_GetEmpty(t *testing.T) {
	repo := newTestSnapshotRepo(t)

	got, err := repo.GetSnapshots("no-such-incident")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestMongoSnapshotRepository_SaveEmpty(t *testing.T) {
	repo := newTestSnapshotRepo(t)

	// Should be a no-op, not an error
	if err := repo.SaveSnapshots("inc-1", nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}
}

func TestMongoSnapshotRepository_PruneBefore(t *testing.T) {
	repo := newTestSnapshotRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	old := now.Add(-48 * time.Hour)

	snaps := []monitoring.IncidentSnapshot{
		{IncidentID: "inc-1", SnapshotType: "old_sample", Timestamp: old, PayloadJSON: `{}`},
		{IncidentID: "inc-1", SnapshotType: "new_sample", Timestamp: now, PayloadJSON: `{}`},
	}
	if err := repo.SaveSnapshots("inc-1", snaps); err != nil {
		t.Fatalf("save: %v", err)
	}

	cutoff := now.Add(-24 * time.Hour)
	if err := repo.PruneBefore(cutoff); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// Only the new snapshot should remain
	got, err := repo.GetSnapshots("inc-1")
	if err != nil {
		t.Fatalf("get after prune: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 snapshot after prune, got %d", len(got))
	}
	if got[0].SnapshotType != "new_sample" {
		t.Fatalf("wrong snapshot survived: %+v", got[0])
	}
}

func TestMongoSnapshotRepository_MultipleIncidents(t *testing.T) {
	repo := newTestSnapshotRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)

	if err := repo.SaveSnapshots("inc-a", []monitoring.IncidentSnapshot{
		{IncidentID: "inc-a", SnapshotType: "sample", Timestamp: now, PayloadJSON: `{"a":1}`},
	}); err != nil {
		t.Fatalf("save inc-a: %v", err)
	}
	if err := repo.SaveSnapshots("inc-b", []monitoring.IncidentSnapshot{
		{IncidentID: "inc-b", SnapshotType: "delta", Timestamp: now, PayloadJSON: `{"b":2}`},
	}); err != nil {
		t.Fatalf("save inc-b: %v", err)
	}

	gotA, _ := repo.GetSnapshots("inc-a")
	gotB, _ := repo.GetSnapshots("inc-b")
	if len(gotA) != 1 || len(gotB) != 1 {
		t.Fatalf("isolation broken: inc-a=%d, inc-b=%d", len(gotA), len(gotB))
	}
}
