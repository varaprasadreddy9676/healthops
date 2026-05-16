package logs

import (
	"os"
	"testing"
	"time"
)

func TestComputeFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		msg1        string
		msg2        string
		source      string
		shouldMatch bool
	}{
		{
			name:        "same message different IPs should match",
			msg1:        "connection refused to 192.168.1.100:3306",
			msg2:        "connection refused to 10.0.0.5:3306",
			source:      "mysql",
			shouldMatch: true,
		},
		{
			name:        "same message different UUIDs should match",
			msg1:        "failed to process request 550e8400-e29b-41d4-a716-446655440000",
			msg2:        "failed to process request a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			source:      "api",
			shouldMatch: true,
		},
		{
			name:        "same message different timestamps should match",
			msg1:        "timeout after 1500ms waiting for response",
			msg2:        "timeout after 3200ms waiting for response",
			source:      "http",
			shouldMatch: true,
		},
		{
			name:        "completely different messages should not match",
			msg1:        "authentication failed for user admin",
			msg2:        "disk space critical on /dev/sda1",
			source:      "system",
			shouldMatch: false,
		},
		{
			name:        "same error different port numbers should match",
			msg1:        "connection refused on port 5432",
			msg2:        "connection refused on port 3306",
			source:      "db",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp1 := ComputeFingerprint(tt.msg1, "", tt.source)
			fp2 := ComputeFingerprint(tt.msg2, "", tt.source)

			if tt.shouldMatch && fp1 != fp2 {
				t.Errorf("expected fingerprints to match:\n  msg1=%q → %s\n  msg2=%q → %s", tt.msg1, fp1, tt.msg2, fp2)
			}
			if !tt.shouldMatch && fp1 == fp2 {
				t.Errorf("expected fingerprints to differ:\n  msg1=%q → %s\n  msg2=%q → %s", tt.msg1, fp1, tt.msg2, fp2)
			}
		})
	}
}

func TestExtractPattern(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "connection refused to 192.168.1.100:3306",
			expected: "connection refused to <ip>",
		},
		{
			input:    "request 550e8400-e29b-41d4-a716-446655440000 failed",
			expected: "request <uuid> failed",
		},
		{
			input:    "timeout after 1500ms",
			expected: "timeout after <duration>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractPattern(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractPattern(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFileRepository_IngestAndCluster(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()

	// Ingest several similar entries
	entries := []LogEntry{
		{ID: "1", Timestamp: now, Level: "error", Message: "connection refused to 192.168.1.100:3306", Source: "mysql-client", Server: "web-1"},
		{ID: "2", Timestamp: now.Add(time.Second), Level: "error", Message: "connection refused to 192.168.1.100:3306", Source: "mysql-client", Server: "web-2"},
		{ID: "3", Timestamp: now.Add(2 * time.Second), Level: "error", Message: "connection refused to 10.0.0.5:3306", Source: "mysql-client", Server: "web-1"},
		{ID: "4", Timestamp: now.Add(3 * time.Second), Level: "warn", Message: "disk space warning: 85% used on /dev/sda1", Source: "system", Server: "db-1"},
	}

	if err := repo.IngestEntries(entries); err != nil {
		t.Fatal(err)
	}

	// Should have 2 families (one for connection refused, one for disk space)
	families, err := repo.ListFamilies("", 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(families) != 2 {
		t.Fatalf("expected 2 families, got %d", len(families))
	}

	// Find the connection refused family
	var connFamily *ErrorFamily
	for i := range families {
		if families[i].OccurrenceCount == 3 {
			connFamily = &families[i]
			break
		}
	}
	if connFamily == nil {
		t.Fatal("expected a family with 3 occurrences")
	}

	if connFamily.Source != "mysql-client" {
		t.Errorf("expected source 'mysql-client', got %q", connFamily.Source)
	}
	if len(connFamily.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d: %v", len(connFamily.Servers), connFamily.Servers)
	}

	// Test entries by family
	familyEntries, err := repo.EntriesByFamily(connFamily.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(familyEntries) != 3 {
		t.Errorf("expected 3 entries in family, got %d", len(familyEntries))
	}
}

func TestFileRepository_Prune(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)

	entries := []LogEntry{
		{ID: "old-1", Timestamp: old, Level: "error", Message: "old error", Source: "app"},
		{ID: "new-1", Timestamp: now, Level: "error", Message: "new error", Source: "app"},
	}

	if err := repo.IngestEntries(entries); err != nil {
		t.Fatal(err)
	}

	if repo.TotalEntries() != 2 {
		t.Fatalf("expected 2 entries, got %d", repo.TotalEntries())
	}

	// Prune entries older than 24h
	cutoff := now.Add(-24 * time.Hour)
	if err := repo.PruneBefore(cutoff); err != nil {
		t.Fatal(err)
	}

	if repo.TotalEntries() != 1 {
		t.Errorf("expected 1 entry after prune, got %d", repo.TotalEntries())
	}
}

func TestFileRepository_Persistence(t *testing.T) {
	dir := t.TempDir()

	// Create repo and ingest data
	repo1, err := NewFileRepository(dir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []LogEntry{
		{ID: "1", Timestamp: time.Now().UTC(), Level: "error", Message: "test error", Source: "app"},
	}
	if err := repo1.IngestEntries(entries); err != nil {
		t.Fatal(err)
	}

	// Create new repo from same dir (simulates restart)
	repo2, err := NewFileRepository(dir)
	if err != nil {
		t.Fatal(err)
	}

	if repo2.TotalEntries() != 1 {
		t.Errorf("expected 1 entry after reload, got %d", repo2.TotalEntries())
	}

	families, err := repo2.ListFamilies("", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 1 {
		t.Errorf("expected 1 family after reload, got %d", len(families))
	}
}

func TestFileRepository_Stats(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	entries := []LogEntry{
		{ID: "1", Timestamp: now, Level: "error", Message: "db auth failed connecting to 10.0.0.1:3306", Source: "mysql"},
		{ID: "2", Timestamp: now, Level: "error", Message: "db auth failed connecting to 10.0.0.2:3306", Source: "mysql"},
		{ID: "3", Timestamp: now, Level: "warn", Message: "connection timeout to redis on port 6379", Source: "cache"},
	}
	if err := repo.IngestEntries(entries); err != nil {
		t.Fatal(err)
	}

	stats := repo.FamilyStats()
	if stats.TotalFamilies != 2 {
		t.Errorf("expected 2 families, got %d", stats.TotalFamilies)
	}
	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 entries, got %d", stats.TotalEntries)
	}
	if stats.ActiveFamilies != 2 {
		t.Errorf("expected 2 active families, got %d", stats.ActiveFamilies)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
