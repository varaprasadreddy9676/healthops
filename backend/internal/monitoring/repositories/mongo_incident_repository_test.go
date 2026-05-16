package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"medics-health-check/backend/internal/monitoring"
	"medics-health-check/backend/internal/util/mongotest"
)

func newTestIncidentRepo(t *testing.T) *MongoIncidentRepository {
	t.Helper()
	client := mongotest.Connect(t, 2*time.Second)
	repo, err := NewMongoIncidentRepository(client, "healthops_test", "test")
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.collection.Drop(ctx); err != nil {
		t.Fatalf("drop collection: %v", err)
	}
	return repo
}

func sampleIncident(id, checkID, status string) monitoring.Incident {
	now := time.Now().UTC().Truncate(time.Millisecond)
	return monitoring.Incident{
		ID:        id,
		CheckID:   checkID,
		CheckName: "test-check",
		Type:      "api",
		Status:    status,
		Severity:  "warning",
		Message:   "test incident",
		StartedAt: now,
		UpdatedAt: now,
		Metadata:  map[string]string{"env": "test"},
	}
}

func TestMongoIncidentRepository_CreateAndGet(t *testing.T) {
	repo := newTestIncidentRepo(t)

	inc := sampleIncident("inc-1", "check-1", "open")
	if err := repo.CreateIncident(inc); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetIncident("inc-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "inc-1" || got.CheckID != "check-1" || got.Status != "open" {
		t.Fatalf("unexpected incident: %+v", got)
	}
	if got.Metadata["env"] != "test" {
		t.Fatalf("metadata not preserved: %+v", got.Metadata)
	}
}

func TestMongoIncidentRepository_CreateDuplicate(t *testing.T) {
	repo := newTestIncidentRepo(t)
	inc := sampleIncident("inc-dup", "check-1", "open")
	if err := repo.CreateIncident(inc); err != nil {
		t.Fatalf("create: %v", err)
	}
	err := repo.CreateIncident(inc)
	if !errors.Is(err, ErrIncidentExists) {
		t.Fatalf("expected ErrIncidentExists, got %v", err)
	}
}

func TestMongoIncidentRepository_GetMissingReturnsZero(t *testing.T) {
	repo := newTestIncidentRepo(t)
	got, err := repo.GetIncident("does-not-exist")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if got.ID != "" {
		t.Fatalf("expected zero incident, got %+v", got)
	}
}

func TestMongoIncidentRepository_UpdateMutator(t *testing.T) {
	repo := newTestIncidentRepo(t)
	inc := sampleIncident("inc-2", "check-1", "open")
	if err := repo.CreateIncident(inc); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := repo.UpdateIncident("inc-2", func(i *monitoring.Incident) error {
		i.Status = "acknowledged"
		i.AcknowledgedBy = "alice"
		now := time.Now().UTC()
		i.AcknowledgedAt = &now
		i.UpdatedAt = now
		return nil
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := repo.GetIncident("inc-2")
	if got.Status != "acknowledged" {
		t.Fatalf("expected acknowledged, got %s", got.Status)
	}
	if got.AcknowledgedBy != "alice" {
		t.Fatalf("expected alice, got %s", got.AcknowledgedBy)
	}
	if got.AcknowledgedAt == nil {
		t.Fatal("AcknowledgedAt not persisted")
	}
}

func TestMongoIncidentRepository_UpdateMissing(t *testing.T) {
	repo := newTestIncidentRepo(t)
	err := repo.UpdateIncident("missing", func(i *monitoring.Incident) error { return nil })
	if !errors.Is(err, ErrIncidentNotFound) {
		t.Fatalf("expected ErrIncidentNotFound, got %v", err)
	}
}

func TestMongoIncidentRepository_UpdateMutatorError(t *testing.T) {
	repo := newTestIncidentRepo(t)
	inc := sampleIncident("inc-err", "check-1", "open")
	_ = repo.CreateIncident(inc)

	mutErr := errors.New("mutator says no")
	err := repo.UpdateIncident("inc-err", func(i *monitoring.Incident) error {
		i.Status = "acknowledged"
		return mutErr
	})
	if !errors.Is(err, mutErr) {
		t.Fatalf("expected mutator error, got %v", err)
	}
	got, _ := repo.GetIncident("inc-err")
	if got.Status != "open" {
		t.Fatalf("mutator error should not persist changes; got status %s", got.Status)
	}
}

func TestMongoIncidentRepository_ListAndFindOpen(t *testing.T) {
	repo := newTestIncidentRepo(t)
	_ = repo.CreateIncident(sampleIncident("inc-a", "check-1", "open"))
	time.Sleep(5 * time.Millisecond)
	_ = repo.CreateIncident(sampleIncident("inc-b", "check-1", "resolved"))
	time.Sleep(5 * time.Millisecond)
	_ = repo.CreateIncident(sampleIncident("inc-c", "check-2", "acknowledged"))

	all, err := repo.ListIncidents()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 incidents, got %d", len(all))
	}

	open, err := repo.FindOpenIncident("check-1")
	if err != nil {
		t.Fatalf("find open: %v", err)
	}
	if open.ID != "inc-a" {
		t.Fatalf("expected inc-a, got %q", open.ID)
	}

	// check-2 has acknowledged (non-resolved) incident
	open2, _ := repo.FindOpenIncident("check-2")
	if open2.ID != "inc-c" {
		t.Fatalf("expected inc-c, got %q", open2.ID)
	}

	// non-existent check
	none, _ := repo.FindOpenIncident("nope")
	if none.ID != "" {
		t.Fatalf("expected zero incident, got %+v", none)
	}
}

func TestMongoIncidentRepository_PruneBefore(t *testing.T) {
	repo := newTestIncidentRepo(t)

	old := sampleIncident("inc-old", "check-1", "resolved")
	old.UpdatedAt = time.Now().UTC().Add(-10 * 24 * time.Hour)
	_ = repo.CreateIncident(old)

	recent := sampleIncident("inc-recent", "check-1", "resolved")
	_ = repo.CreateIncident(recent)

	openOld := sampleIncident("inc-open-old", "check-2", "open")
	openOld.UpdatedAt = time.Now().UTC().Add(-30 * 24 * time.Hour)
	_ = repo.CreateIncident(openOld)

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	if err := repo.PruneBefore(cutoff); err != nil {
		t.Fatalf("prune: %v", err)
	}

	all, _ := repo.ListIncidents()
	ids := make(map[string]bool, len(all))
	for _, i := range all {
		ids[i.ID] = true
	}

	if ids["inc-old"] {
		t.Fatal("expected old resolved incident to be pruned")
	}
	if !ids["inc-recent"] {
		t.Fatal("expected recent resolved incident to be kept")
	}
	if !ids["inc-open-old"] {
		t.Fatal("expected open incident to be kept regardless of age")
	}
}
