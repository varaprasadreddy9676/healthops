package monitoring

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeMirror struct {
	syncCalls int
	readCalls int
	syncErr   error
	readState State
	readErr   error
}

func (f *fakeMirror) SyncState(context.Context, State) error {
	f.syncCalls++
	return f.syncErr
}

func (f *fakeMirror) ReadState(context.Context) (State, error) {
	f.readCalls++
	return f.readState, f.readErr
}

func (f *fakeMirror) ReadDashboardSnapshot(context.Context) (DashboardSnapshot, error) {
	f.readCalls++
	return buildDashboardSnapshot(f.readState), f.readErr
}

func TestHybridStoreSnapshotUsesLocalState(t *testing.T) {
	local := newTestFileStore(t, []CheckConfig{{ID: "local-check", Name: "Local Check", Type: "api", Target: "https://example.com/health"}})
	mirror := &fakeMirror{readErr: errors.New("mongo unavailable")}
	store := &HybridStore{
		local:       local,
		mirror:      mirror,
		logger:      log.New(io.Discard, "", 0),
		readTimeout: time.Second,
		syncTimeout: time.Second,
	}

	state := store.Snapshot()
	if got := len(state.Checks); got != 1 {
		t.Fatalf("expected 1 local check, got %d", got)
	}
	if state.Checks[0].ID != "local-check" {
		t.Fatalf("expected local snapshot, got %+v", state.Checks[0])
	}
	// Mongo is primary, so it should be tried first
	if mirror.readCalls != 1 {
		t.Fatalf("expected 1 mirror read attempt, got %d", mirror.readCalls)
	}
}

func TestHybridStoreDashboardSnapshotPrefersFreshMirrorWhenAvailable(t *testing.T) {
	local := newTestFileStore(t, []CheckConfig{{ID: "local-check", Name: "Local Check", Type: "api", Target: "https://example.com/health"}})
	localState := local.Snapshot()
	mirror := &fakeMirror{
		readState: State{
			Checks:    []CheckConfig{{ID: "mongo-check", Name: "Mongo Check", Type: "tcp", Port: 8080}},
			UpdatedAt: localState.UpdatedAt,
		},
	}
	store := &HybridStore{
		local:       local,
		mirror:      mirror,
		logger:      log.New(io.Discard, "", 0),
		readTimeout: time.Second,
		syncTimeout: time.Second,
	}

	snapshot := store.DashboardSnapshot()
	if got := len(snapshot.State.Checks); got != 1 {
		t.Fatalf("expected 1 mirror check, got %d", got)
	}
	if snapshot.State.Checks[0].ID != "mongo-check" {
		t.Fatalf("expected mirror snapshot, got %+v", snapshot.State.Checks[0])
	}
	if mirror.readCalls != 1 {
		t.Fatalf("expected 1 mirror read, got %d", mirror.readCalls)
	}
}

func TestHybridStoreDashboardSnapshotFallsBackToLocalWhenMirrorFails(t *testing.T) {
	local := newTestFileStore(t, []CheckConfig{{ID: "local-check", Name: "Local Check", Type: "api", Target: "https://example.com/health"}})
	mirror := &fakeMirror{readErr: errors.New("mongo unavailable")}
	store := &HybridStore{
		local:       local,
		mirror:      mirror,
		logger:      log.New(io.Discard, "", 0),
		readTimeout: time.Second,
		syncTimeout: time.Second,
	}

	snapshot := store.DashboardSnapshot()
	if got := len(snapshot.State.Checks); got != 1 {
		t.Fatalf("expected local check after mirror failure, got %d", got)
	}
	if snapshot.State.Checks[0].ID != "local-check" {
		t.Fatalf("expected local dashboard snapshot, got %+v", snapshot.State.Checks[0])
	}
}

func TestHybridStoreUpdateKeepsLocalStateWhenMirrorSyncFails(t *testing.T) {
	local := newTestFileStore(t, nil)
	mirror := &fakeMirror{syncErr: errors.New("mongo down")}
	store := &HybridStore{
		local:       local,
		mirror:      mirror,
		logger:      log.New(io.Discard, "", 0),
		readTimeout: time.Second,
		syncTimeout: time.Second,
	}

	check := CheckConfig{
		ID:     "api-check",
		Name:   "API Check",
		Type:   "api",
		Target: "https://example.com/health",
	}
	if err := store.UpsertCheck(check); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	state := store.Snapshot()
	if got := len(state.Checks); got != 1 {
		t.Fatalf("expected 1 local check, got %d", got)
	}
	if state.Checks[0].ID != "api-check" {
		t.Fatalf("expected local check after failed sync, got %+v", state.Checks[0])
	}
	if mirror.syncCalls == 0 {
		t.Fatalf("expected mirror sync to be attempted")
	}
}

func newTestFileStore(t *testing.T, checks []CheckConfig) *FileStore {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store, err := NewFileStore(path, checks)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}
	return store
}
