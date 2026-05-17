package monitoring

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMaintenanceWindowIsActive(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name   string
		window MaintenanceWindow
		want   bool
	}{
		{
			name: "active window",
			window: MaintenanceWindow{
				Enabled:   true,
				StartTime: now.Add(-1 * time.Hour),
				EndTime:   now.Add(1 * time.Hour),
			},
			want: true,
		},
		{
			name: "future window",
			window: MaintenanceWindow{
				Enabled:   true,
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(2 * time.Hour),
			},
			want: false,
		},
		{
			name: "past window",
			window: MaintenanceWindow{
				Enabled:   true,
				StartTime: now.Add(-2 * time.Hour),
				EndTime:   now.Add(-1 * time.Hour),
			},
			want: false,
		},
		{
			name: "disabled window",
			window: MaintenanceWindow{
				Enabled:   false,
				StartTime: now.Add(-1 * time.Hour),
				EndTime:   now.Add(1 * time.Hour),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.window.IsActive(now); got != tt.want {
				t.Errorf("IsActive() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestMaintenanceWindowCoversCheck(t *testing.T) {
	tests := []struct {
		name   string
		window MaintenanceWindow
		check  CheckConfig
		want   bool
	}{
		{
			name:   "empty scope covers all",
			window: MaintenanceWindow{},
			check:  CheckConfig{ID: "any", Server: "web-1"},
			want:   true,
		},
		{
			name:   "covers by check ID",
			window: MaintenanceWindow{CheckIDs: []string{"check-1", "check-2"}},
			check:  CheckConfig{ID: "check-1"},
			want:   true,
		},
		{
			name:   "does not cover non-matching ID",
			window: MaintenanceWindow{CheckIDs: []string{"check-1"}},
			check:  CheckConfig{ID: "check-99"},
			want:   false,
		},
		{
			name:   "covers by tag",
			window: MaintenanceWindow{Tags: []string{"production"}},
			check:  CheckConfig{ID: "c", Tags: []string{"production", "web"}},
			want:   true,
		},
		{
			name:   "does not cover non-matching tag",
			window: MaintenanceWindow{Tags: []string{"staging"}},
			check:  CheckConfig{ID: "c", Tags: []string{"production"}},
			want:   false,
		},
		{
			name:   "covers by server",
			window: MaintenanceWindow{Servers: []string{"web-1", "web-2"}},
			check:  CheckConfig{ID: "c", Server: "web-1"},
			want:   true,
		},
		{
			name:   "does not cover non-matching server",
			window: MaintenanceWindow{Servers: []string{"web-1"}},
			check:  CheckConfig{ID: "c", Server: "db-1"},
			want:   false,
		},
		{
			name:   "check with no tags against tag-scoped window",
			window: MaintenanceWindow{Tags: []string{"production"}},
			check:  CheckConfig{ID: "c"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.window.CoversCheck(tt.check); got != tt.want {
				t.Errorf("CoversCheck() = %v; want %v", got, tt.want)
			}
		})
	}
}

func newTempMaintenanceStore(t *testing.T) *MaintenanceStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewMaintenanceStore(filepath.Join(dir, "maint.json"))
	if err != nil {
		t.Fatalf("NewMaintenanceStore: %v", err)
	}
	return store
}

func TestMaintenanceStoreCRUD(t *testing.T) {
	store := newTempMaintenanceStore(t)
	now := time.Now().UTC()

	// Create
	mw := MaintenanceWindow{
		ID:        "mw-1",
		Name:      "Deploy Upgrade",
		StartTime: now.Add(-10 * time.Minute),
		EndTime:   now.Add(50 * time.Minute),
		Enabled:   true,
	}
	if err := store.Create(mw); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Get
	got, err := store.Get("mw-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Deploy Upgrade" {
		t.Errorf("Name = %q; want 'Deploy Upgrade'", got.Name)
	}

	// List
	all := store.List()
	if len(all) != 1 {
		t.Fatalf("List len = %d; want 1", len(all))
	}

	// Update
	if err := store.Update("mw-1", func(w *MaintenanceWindow) error {
		w.Name = "Updated Deploy"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = store.Get("mw-1")
	if got.Name != "Updated Deploy" {
		t.Errorf("after update Name = %q; want 'Updated Deploy'", got.Name)
	}

	// Delete
	if err := store.Delete("mw-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = store.Get("mw-1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestMaintenanceStoreGetNotFound(t *testing.T) {
	store := newTempMaintenanceStore(t)
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent window")
	}
}

func TestMaintenanceStoreDeleteNotFound(t *testing.T) {
	store := newTempMaintenanceStore(t)
	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent window")
	}
}

func TestMaintenanceStoreUpdateNotFound(t *testing.T) {
	store := newTempMaintenanceStore(t)
	err := store.Update("nonexistent", func(w *MaintenanceWindow) error {
		w.Name = "nope"
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nonexistent window")
	}
}

func TestMaintenanceStoreListActive(t *testing.T) {
	store := newTempMaintenanceStore(t)
	now := time.Now().UTC()

	// Active window
	store.Create(MaintenanceWindow{
		ID:        "active",
		Name:      "Active",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})
	// Future window
	store.Create(MaintenanceWindow{
		ID:        "future",
		Name:      "Future",
		StartTime: now.Add(1 * time.Hour),
		EndTime:   now.Add(2 * time.Hour),
		Enabled:   true,
	})
	// Disabled window
	store.Create(MaintenanceWindow{
		ID:        "disabled",
		Name:      "Disabled",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   false,
	})

	active := store.ListActive()
	if len(active) != 1 {
		t.Fatalf("ListActive len = %d; want 1", len(active))
	}
	if active[0].ID != "active" {
		t.Errorf("active window ID = %q; want 'active'", active[0].ID)
	}
}

func TestMaintenanceStoreIsCheckInMaintenance(t *testing.T) {
	store := newTempMaintenanceStore(t)
	now := time.Now().UTC()

	store.Create(MaintenanceWindow{
		ID:        "mw-scope",
		Name:      "Scoped",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
		CheckIDs:  []string{"check-A"},
	})

	if !store.IsCheckInMaintenance(CheckConfig{ID: "check-A"}) {
		t.Error("check-A should be in maintenance")
	}
	if store.IsCheckInMaintenance(CheckConfig{ID: "check-B"}) {
		t.Error("check-B should NOT be in maintenance")
	}
}

func TestMaintenanceStorePruneExpired(t *testing.T) {
	store := newTempMaintenanceStore(t)
	now := time.Now().UTC()

	// Past non-recurring
	store.Create(MaintenanceWindow{
		ID:        "past",
		Name:      "Past",
		StartTime: now.Add(-48 * time.Hour),
		EndTime:   now.Add(-24 * time.Hour),
		Enabled:   true,
	})
	// Past recurring — should NOT be pruned
	store.Create(MaintenanceWindow{
		ID:        "recurring",
		Name:      "Recurring",
		StartTime: now.Add(-48 * time.Hour),
		EndTime:   now.Add(-24 * time.Hour),
		Enabled:   true,
		Recurring: true,
	})
	// Active
	store.Create(MaintenanceWindow{
		ID:        "active",
		Name:      "Active",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})

	removed, err := store.PruneExpired(now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d; want 1", removed)
	}

	all := store.List()
	if len(all) != 2 {
		t.Fatalf("after prune len = %d; want 2", len(all))
	}
}

func TestMaintenanceStorePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "maint.json")
	now := time.Now().UTC()

	// Create and add a window
	store1, err := NewMaintenanceStore(path)
	if err != nil {
		t.Fatalf("NewMaintenanceStore: %v", err)
	}
	store1.Create(MaintenanceWindow{
		ID:        "persist-test",
		Name:      "Persisted",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})

	// Reload from disk
	store2, err := NewMaintenanceStore(path)
	if err != nil {
		t.Fatalf("NewMaintenanceStore reload: %v", err)
	}
	all := store2.List()
	if len(all) != 1 {
		t.Fatalf("persisted len = %d; want 1", len(all))
	}
	if all[0].Name != "Persisted" {
		t.Errorf("persisted Name = %q; want 'Persisted'", all[0].Name)
	}
}

func TestMaintenanceStoreAutoID(t *testing.T) {
	store := newTempMaintenanceStore(t)
	now := time.Now().UTC()

	mw := MaintenanceWindow{
		Name:      "No ID",
		StartTime: now,
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	}
	if err := store.Create(mw); err != nil {
		t.Fatalf("Create: %v", err)
	}

	all := store.List()
	if len(all) != 1 {
		t.Fatalf("len = %d; want 1", len(all))
	}
	if all[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
}

func TestMaintenanceStoreNewWithBadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	// Write invalid JSON
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := NewMaintenanceStore(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
