package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"health-ops/backend/internal/monitoring"
	"health-ops/backend/internal/util/mongotest"
)

func newTestMySQLRepo(t *testing.T) *MongoMySQLRepository {
	t.Helper()
	client := mongotest.Connect(t, 2*time.Second)
	repo, err := NewMongoMySQLRepository(client, "healthops_test", "test")
	if err != nil {
		t.Fatalf("init mysql repo: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.samplesCollection.Drop(ctx); err != nil {
		t.Fatalf("drop samples: %v", err)
	}
	if err := repo.deltasCollection.Drop(ctx); err != nil {
		t.Fatalf("drop deltas: %v", err)
	}
	// Re-create indexes after drop
	if err := repo.ensureIndexes(); err != nil {
		t.Fatalf("re-create indexes: %v", err)
	}
	return repo
}

func testSample(checkID, sampleID string, ts time.Time) monitoring.MySQLSample {
	return monitoring.MySQLSample{
		SampleID:           sampleID,
		CheckID:            checkID,
		Timestamp:          ts,
		Connections:        100,
		MaxConnections:     200,
		MaxUsedConnections: 150,
		ThreadsRunning:     5,
		SlowQueries:        10,
		Questions:          5000,
		UptimeSeconds:      3600,
	}
}

func TestMongoMySQLRepository_AppendSample(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	sample := testSample("mysql-1", "sample-1", now)

	id, err := repo.AppendSample(sample)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if id != "sample-1" {
		t.Fatalf("expected sample-1, got %s", id)
	}

	// Duplicate insert is idempotent
	id2, err := repo.AppendSample(sample)
	if err != nil {
		t.Fatalf("duplicate append: %v", err)
	}
	if id2 != "sample-1" {
		t.Fatalf("expected sample-1 on dup, got %s", id2)
	}
}

func TestMongoMySQLRepository_AppendSampleGeneratesID(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	sample := monitoring.MySQLSample{
		CheckID:   "mysql-auto",
		Timestamp: now,
	}

	id, err := repo.AppendSample(sample)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty generated ID")
	}
}

func TestMongoMySQLRepository_LatestSample(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	s1 := testSample("mysql-1", "s1", now.Add(-time.Minute))
	s2 := testSample("mysql-1", "s2", now)

	if _, err := repo.AppendSample(s1); err != nil {
		t.Fatalf("append s1: %v", err)
	}
	if _, err := repo.AppendSample(s2); err != nil {
		t.Fatalf("append s2: %v", err)
	}

	latest, err := repo.LatestSample("mysql-1")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest.SampleID != "s2" {
		t.Fatalf("expected s2, got %s", latest.SampleID)
	}
}

func TestMongoMySQLRepository_RecentSamples(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 5; i++ {
		s := testSample("mysql-1", fmt.Sprintf("s%d", i), now.Add(time.Duration(i)*time.Minute))
		if _, err := repo.AppendSample(s); err != nil {
			t.Fatalf("append s%d: %v", i, err)
		}
	}

	results, err := repo.RecentSamples("mysql-1", 3)
	if err != nil {
		t.Fatalf("recent samples: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
	// Should be newest first
	if results[0].SampleID != "s4" {
		t.Fatalf("expected s4 first, got %s", results[0].SampleID)
	}
}

func TestMongoMySQLRepository_ComputeAndAppendDelta(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	s1 := testSample("mysql-1", "s1", now.Add(-time.Minute))
	s1.SlowQueries = 5
	s1.Questions = 1000

	s2 := testSample("mysql-1", "s2", now)
	s2.SlowQueries = 10
	s2.Questions = 2000

	if _, err := repo.AppendSample(s1); err != nil {
		t.Fatalf("append s1: %v", err)
	}
	if _, err := repo.AppendSample(s2); err != nil {
		t.Fatalf("append s2: %v", err)
	}

	delta, err := repo.ComputeAndAppendDelta("s2")
	if err != nil {
		t.Fatalf("compute delta: %v", err)
	}
	if delta.SampleID != "s2" {
		t.Fatalf("expected delta sampleId=s2, got %s", delta.SampleID)
	}
	if delta.SlowQueriesDelta != 5 {
		t.Fatalf("expected slowQueriesDelta=5, got %d", delta.SlowQueriesDelta)
	}
	if delta.QuestionsDelta != 1000 {
		t.Fatalf("expected questionsDelta=1000, got %d", delta.QuestionsDelta)
	}

	// Verify delta was persisted
	deltas, err := repo.RecentDeltas("mysql-1", 10)
	if err != nil {
		t.Fatalf("recent deltas: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}
}

func TestMongoMySQLRepository_ComputeAndAppendDelta_NoPrevious(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	s1 := testSample("mysql-1", "s1", now)
	if _, err := repo.AppendSample(s1); err != nil {
		t.Fatalf("append: %v", err)
	}

	_, err := repo.ComputeAndAppendDelta("s1")
	if err == nil {
		t.Fatal("expected error for no previous sample")
	}
}

func TestMongoMySQLRepository_PruneBefore(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	old := now.Add(-48 * time.Hour)

	s1 := testSample("mysql-1", "old-sample", old)
	s2 := testSample("mysql-1", "new-sample", now)

	if _, err := repo.AppendSample(s1); err != nil {
		t.Fatalf("append old: %v", err)
	}
	if _, err := repo.AppendSample(s2); err != nil {
		t.Fatalf("append new: %v", err)
	}

	cutoff := now.Add(-24 * time.Hour)
	if err := repo.PruneBefore(cutoff); err != nil {
		t.Fatalf("prune: %v", err)
	}

	samples, err := repo.RecentSamples("mysql-1", 10)
	if err != nil {
		t.Fatalf("recent after prune: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample after prune, got %d", len(samples))
	}
	if samples[0].SampleID != "new-sample" {
		t.Fatalf("wrong sample survived: %s", samples[0].SampleID)
	}
}

func TestMongoMySQLRepository_RecentDeltas(t *testing.T) {
	repo := newTestMySQLRepo(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	// Create 3 samples for the same check to generate 2 deltas
	for i := 0; i < 3; i++ {
		s := testSample("mysql-1", fmt.Sprintf("sd%d", i), now.Add(time.Duration(i)*time.Minute))
		s.SlowQueries = int64(i * 5)
		s.Questions = int64(i * 1000)
		if _, err := repo.AppendSample(s); err != nil {
			t.Fatalf("append sd%d: %v", i, err)
		}
	}

	// Compute deltas
	if _, err := repo.ComputeAndAppendDelta("sd1"); err != nil {
		t.Fatalf("delta sd1: %v", err)
	}
	if _, err := repo.ComputeAndAppendDelta("sd2"); err != nil {
		t.Fatalf("delta sd2: %v", err)
	}

	deltas, err := repo.RecentDeltas("mysql-1", 10)
	if err != nil {
		t.Fatalf("recent deltas: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d", len(deltas))
	}
	// Newest first
	if deltas[0].SampleID != "sd2" {
		t.Fatalf("expected sd2 first, got %s", deltas[0].SampleID)
	}
}
