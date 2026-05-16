package monitoring

import (
	"context"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// testMongoClient returns a connected Mongo client, skipping the test if
// MONGODB_URI is not set or unreachable.
func testMongoClient(t *testing.T) *mongo.Client {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set — skipping MongoStore integration test")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetConnectTimeout(5 * time.Second))
	if err != nil {
		t.Skipf("cannot connect to MongoDB: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB ping failed: %v", err)
	}
	return client
}

// newTestMongoStore creates a MongoStore with a unique test database that is
// automatically dropped at the end of the test.
func newTestMongoStore(t *testing.T, seedChecks []CheckConfig) *MongoStore {
	t.Helper()
	client := testMongoClient(t)
	dbName := "healthops_test_" + t.Name()
	logger := log.New(io.Discard, "", 0)

	s, err := NewMongoStore(client, dbName, "test", 7, seedChecks, logger)
	if err != nil {
		t.Fatalf("NewMongoStore: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Database(dbName).Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})
	return s
}

func TestMongoStoreSnapshotReturnsSeededChecks(t *testing.T) {
	seed := []CheckConfig{
		{ID: "c1", Name: "API Health", Type: "api", Target: "https://example.com/healthz"},
		{ID: "c2", Name: "DB Port", Type: "tcp", Target: "db.example.com", Port: 5432},
	}
	s := newTestMongoStore(t, seed)

	state := s.Snapshot()
	if len(state.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(state.Checks))
	}
	ids := map[string]bool{}
	for _, c := range state.Checks {
		ids[c.ID] = true
	}
	if !ids["c1"] || !ids["c2"] {
		t.Fatalf("expected c1 and c2, got %v", ids)
	}
}

func TestMongoStoreSnapshotIsClone(t *testing.T) {
	s := newTestMongoStore(t, []CheckConfig{{ID: "c1", Name: "A", Type: "api", Target: "https://a.com"}})

	snap1 := s.Snapshot()
	snap1.Checks[0].Name = "MUTATED"

	snap2 := s.Snapshot()
	if snap2.Checks[0].Name == "MUTATED" {
		t.Fatal("Snapshot must return a clone, not a reference")
	}
}

func TestMongoStoreUpsertCheck(t *testing.T) {
	s := newTestMongoStore(t, nil)

	c := CheckConfig{ID: "upsert-1", Name: "New Check", Type: "tcp", Target: "host.example.com", Port: 443}
	if err := s.UpsertCheck(c); err != nil {
		t.Fatalf("UpsertCheck: %v", err)
	}

	state := s.Snapshot()
	if len(state.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(state.Checks))
	}
	if state.Checks[0].Name != "New Check" {
		t.Fatalf("expected 'New Check', got %q", state.Checks[0].Name)
	}

	// Upsert again with updated name
	c.Name = "Updated Check"
	if err := s.UpsertCheck(c); err != nil {
		t.Fatalf("UpsertCheck update: %v", err)
	}
	state = s.Snapshot()
	if len(state.Checks) != 1 {
		t.Fatalf("expected 1 check after update, got %d", len(state.Checks))
	}
	if state.Checks[0].Name != "Updated Check" {
		t.Fatalf("expected 'Updated Check', got %q", state.Checks[0].Name)
	}
}

func TestMongoStoreDeleteCheck(t *testing.T) {
	seed := []CheckConfig{
		{ID: "del-1", Name: "Check 1", Type: "api", Target: "https://a.com"},
		{ID: "del-2", Name: "Check 2", Type: "api", Target: "https://b.com"},
	}
	s := newTestMongoStore(t, seed)

	if err := s.DeleteCheck("del-1"); err != nil {
		t.Fatalf("DeleteCheck: %v", err)
	}

	state := s.Snapshot()
	if len(state.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(state.Checks))
	}
	if state.Checks[0].ID != "del-2" {
		t.Fatalf("expected del-2 to remain, got %q", state.Checks[0].ID)
	}
}

func TestMongoStoreReplaceChecks(t *testing.T) {
	seed := []CheckConfig{
		{ID: "old-1", Name: "Old", Type: "api", Target: "https://old.com"},
	}
	s := newTestMongoStore(t, seed)

	newChecks := []CheckConfig{
		{ID: "new-1", Name: "New A", Type: "tcp", Target: "a.com", Port: 80},
		{ID: "new-2", Name: "New B", Type: "tcp", Target: "b.com", Port: 443},
	}
	if err := s.ReplaceChecks(newChecks); err != nil {
		t.Fatalf("ReplaceChecks: %v", err)
	}

	state := s.Snapshot()
	if len(state.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(state.Checks))
	}
	ids := map[string]bool{}
	for _, c := range state.Checks {
		ids[c.ID] = true
	}
	if ids["old-1"] {
		t.Fatal("old check should have been replaced")
	}
	if !ids["new-1"] || !ids["new-2"] {
		t.Fatalf("expected new-1 and new-2, got %v", ids)
	}
}

func TestMongoStoreAppendResults(t *testing.T) {
	s := newTestMongoStore(t, []CheckConfig{{ID: "r1", Name: "R1", Type: "api", Target: "https://r.com"}})

	results := []CheckResult{
		{CheckID: "r1", Status: "ok", FinishedAt: time.Now().UTC()},
		{CheckID: "r1", Status: "ok", FinishedAt: time.Now().UTC().Add(time.Second)},
	}
	if err := s.AppendResults(results, 7); err != nil {
		t.Fatalf("AppendResults: %v", err)
	}

	state := s.Snapshot()
	if len(state.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(state.Results))
	}
}

func TestMongoStoreSetLastRun(t *testing.T) {
	s := newTestMongoStore(t, nil)

	now := time.Now().UTC().Truncate(time.Second)
	if err := s.SetLastRun(now); err != nil {
		t.Fatalf("SetLastRun: %v", err)
	}

	state := s.Snapshot()
	if !state.LastRunAt.Equal(now) {
		t.Fatalf("expected LastRunAt %v, got %v", now, state.LastRunAt)
	}
}

func TestMongoStoreUpdate(t *testing.T) {
	s := newTestMongoStore(t, []CheckConfig{{ID: "upd-1", Name: "Original", Type: "api", Target: "https://a.com"}})

	err := s.Update(func(state *State) error {
		for i := range state.Checks {
			if state.Checks[i].ID == "upd-1" {
				state.Checks[i].Name = "Modified"
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	state := s.Snapshot()
	if state.Checks[0].Name != "Modified" {
		t.Fatalf("expected 'Modified', got %q", state.Checks[0].Name)
	}
}

func TestMongoStoreDashboardSnapshot(t *testing.T) {
	seed := []CheckConfig{
		{ID: "dash-1", Name: "Dash Check", Type: "api", Target: "https://dash.com"},
	}
	s := newTestMongoStore(t, seed)

	snap := s.DashboardSnapshot()
	if len(snap.State.Checks) != 1 {
		t.Fatalf("expected 1 check in dashboard, got %d", len(snap.State.Checks))
	}
	if snap.Summary.TotalChecks != 1 {
		t.Fatalf("expected summary total 1, got %d", snap.Summary.TotalChecks)
	}
}

func TestMongoStoreStatePersistsAcrossRestart(t *testing.T) {
	client := testMongoClient(t)
	dbName := "healthops_test_persist_" + t.Name()
	logger := log.New(io.Discard, "", 0)
	t.Cleanup(func() {
		_ = client.Database(dbName).Drop(context.Background())
	})

	// Create store and add some state
	s1, err := NewMongoStore(client, dbName, "test", 7, []CheckConfig{
		{ID: "p1", Name: "Persist", Type: "api", Target: "https://p.com"},
	}, logger)
	if err != nil {
		t.Fatalf("NewMongoStore s1: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	_ = s1.SetLastRun(now)
	_ = s1.AppendResults([]CheckResult{{CheckID: "p1", Status: "ok", FinishedAt: now}}, 7)

	// Create a new MongoStore against the same database — simulates restart
	s2, err := NewMongoStore(client, dbName, "test", 7, nil, logger)
	if err != nil {
		t.Fatalf("NewMongoStore s2: %v", err)
	}

	state2 := s2.Snapshot()
	if len(state2.Checks) != 1 || state2.Checks[0].ID != "p1" {
		t.Fatalf("expected persisted check p1, got %+v", state2.Checks)
	}
	if len(state2.Results) != 1 {
		t.Fatalf("expected 1 persisted result, got %d", len(state2.Results))
	}
	if !state2.LastRunAt.Equal(now) {
		t.Fatalf("expected persisted LastRunAt %v, got %v", now, state2.LastRunAt)
	}
}

func TestMongoStoreDoesNotReseedWhenChecksExist(t *testing.T) {
	client := testMongoClient(t)
	dbName := "healthops_test_noseed_" + t.Name()
	logger := log.New(io.Discard, "", 0)
	t.Cleanup(func() {
		_ = client.Database(dbName).Drop(context.Background())
	})

	// First run: seed 2 checks
	s1, err := NewMongoStore(client, dbName, "test", 7, []CheckConfig{
		{ID: "s1", Name: "Seed 1", Type: "api", Target: "https://1.com"},
		{ID: "s2", Name: "Seed 2", Type: "api", Target: "https://2.com"},
	}, logger)
	if err != nil {
		t.Fatalf("NewMongoStore s1: %v", err)
	}
	// Delete one via API
	_ = s1.DeleteCheck("s2")

	// Second run: same seeds passed but should NOT re-seed because checks exist
	s2, err := NewMongoStore(client, dbName, "test", 7, []CheckConfig{
		{ID: "s1", Name: "Seed 1", Type: "api", Target: "https://1.com"},
		{ID: "s2", Name: "Seed 2", Type: "api", Target: "https://2.com"},
	}, logger)
	if err != nil {
		t.Fatalf("NewMongoStore s2: %v", err)
	}

	state := s2.Snapshot()
	if len(state.Checks) != 1 {
		t.Fatalf("expected 1 check (no re-seed), got %d: %+v", len(state.Checks), state.Checks)
	}
}
