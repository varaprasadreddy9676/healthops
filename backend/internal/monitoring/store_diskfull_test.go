package monitoring

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDiskFullPreservesPreviousState simulates a disk-full / read-only volume
// at the syscall level by chmod'ing the state directory to 0500 (read+exec
// only). This causes os.WriteFile and os.Rename inside writeLocked to fail
// with EACCES, mimicking what the kernel returns on ENOSPC for the rename
// step. The contract under test:
//
//  1. Update returns a non-nil error (no panic, no silent corruption).
//  2. The on-disk state.json is bit-for-bit identical to the version present
//     before the failed write.
//  3. The in-memory store state is NOT advanced to the would-be-new value.
//  4. After permissions are restored, a subsequent Update succeeds and the
//     "phantom" mutation is genuinely absent on disk and in memory.
//
// We intentionally avoid Linux-only tmpfs/quota syscalls so this runs on
// macOS dev machines and Linux CI without root privileges. The test is
// skipped on Windows (chmod semantics differ) and when running as root
// (where mode 0500 is honoured only for non-root processes).
func TestDiskFullPreservesPreviousState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based write-failure simulation is not portable to Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 0500 does not deny writes for root")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewFileStore(path, nil)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Step 1: do a normal Update so we have a known committed baseline on disk.
	if err := store.Update(func(s *State) error {
		s.Checks = []CheckConfig{{
			ID:     "before",
			Name:   "Committed Before Failure",
			Type:   "api",
			Target: "https://example.com",
		}}
		return nil
	}); err != nil {
		t.Fatalf("baseline update: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var beforeState State
	if err := json.Unmarshal(before, &beforeState); err != nil {
		t.Fatalf("baseline unparseable: %v", err)
	}

	// Step 2: make the directory un-writable. New file creation and rename
	// will both fail with EACCES — the same error class an actual ENOSPC
	// surface would manifest as for callers above the syscall layer.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod down: %v", err)
	}
	// Best-effort restore so t.TempDir cleanup can run even if assertions fail.
	defer func() { _ = os.Chmod(dir, 0o700) }()

	updateErr := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Update panicked on write failure (must return error): %v", r)
			}
		}()
		return store.Update(func(s *State) error {
			s.Checks = append(s.Checks, CheckConfig{
				ID:     "during-failure",
				Name:   "MUST NOT PERSIST",
				Type:   "api",
				Target: "https://example.com",
			})
			return nil
		})
	}()
	if updateErr == nil {
		t.Fatal("Update returned nil error despite un-writable directory; expected EACCES surfaced as error")
	}
	t.Logf("expected write failure surfaced: %v", updateErr)

	// Restore perms for read-back.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod up: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after-failure: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("state.json was modified despite Update failure\nbefore=%s\nafter =%s",
			before, after)
	}

	// In-memory snapshot must reflect the pre-failure state, not the phantom.
	snap := store.Snapshot()
	if len(snap.Checks) != 1 || snap.Checks[0].ID != "before" {
		t.Fatalf("in-memory state advanced past failed write: %+v", snap.Checks)
	}

	// No leftover .tmp scratch files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover scratch file from failed write: %s", e.Name())
		}
	}

	// Step 3: recovery — a fresh Update after permissions are restored must
	// succeed cleanly and contain ONLY {before, after}, never the phantom.
	if err := store.Update(func(s *State) error {
		s.Checks = append(s.Checks, CheckConfig{
			ID:     "after",
			Name:   "Committed After Recovery",
			Type:   "api",
			Target: "https://example.com",
		})
		return nil
	}); err != nil {
		t.Fatalf("recovery update: %v", err)
	}

	store2, err := NewFileStore(path, nil)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	final := store2.Snapshot()
	if len(final.Checks) != 2 {
		t.Fatalf("recovery state: want 2 checks (before, after), got %d: %+v",
			len(final.Checks), final.Checks)
	}
	got := map[string]bool{}
	for _, c := range final.Checks {
		got[c.ID] = true
	}
	if !got["before"] || !got["after"] || got["during-failure"] {
		t.Fatalf("recovery state has wrong contents: before=%v after=%v during-failure=%v",
			got["before"], got["after"], got["during-failure"])
	}
}
