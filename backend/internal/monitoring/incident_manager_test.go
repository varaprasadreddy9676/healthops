package monitoring

import (
	"testing"
	"time"
)

// TestIncidentCreationFromAlert tests creating a new incident from an alert
func TestIncidentCreationFromAlert(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"
	checkName := "API Health Check"
	checkType := "api"
	severity := "critical"
	message := "API endpoint returned 500"
	metadata := map[string]string{"url": "https://api.example.com", "status": "500"}

	err := manager.ProcessAlert(checkID, checkName, checkType, severity, message, metadata)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	// Verify incident was created
	incidents, err := repo.ListIncidents()
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}

	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	if incident.CheckID != checkID {
		t.Errorf("expected CheckID %s, got %s", checkID, incident.CheckID)
	}

	if incident.CheckName != checkName {
		t.Errorf("expected CheckName %s, got %s", checkName, incident.CheckName)
	}

	if incident.Type != checkType {
		t.Errorf("expected Type %s, got %s", checkType, incident.Type)
	}

	if incident.Status != "open" {
		t.Errorf("expected Status 'open', got %s", incident.Status)
	}

	if incident.Severity != severity {
		t.Errorf("expected Severity %s, got %s", severity, incident.Severity)
	}

	if incident.Message != message {
		t.Errorf("expected Message %s, got %s", message, incident.Message)
	}

	if incident.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}

	if incident.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

// TestIncidentUpdateForExisting tests updating an existing incident
func TestIncidentUpdateForExisting(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create initial incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "warning", "Slow response", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	// Update with increased severity
	newMetadata := map[string]string{"latency": "5000ms"}
	err = manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "Timeout", newMetadata)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	// Verify only one incident exists
	incidents, err := repo.ListIncidents()
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}

	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	if incident.Severity != "critical" {
		t.Errorf("expected Severity 'critical', got %s", incident.Severity)
	}

	if incident.Message != "Timeout" {
		t.Errorf("expected Message 'Timeout', got %s", incident.Message)
	}

	if incident.Metadata == nil {
		t.Error("expected Metadata to be set")
	} else if incident.Metadata["latency"] != "5000ms" {
		t.Errorf("expected latency '5000ms', got %s", incident.Metadata["latency"])
	}
}

// TestFindOpenIncidentLogic tests finding open incidents
func TestFindOpenIncidentLogic(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create an incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "API down", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	// Find open incident
	incident, err := repo.FindOpenIncident(checkID)
	if err != nil {
		t.Fatalf("FindOpenIncident failed: %v", err)
	}

	if incident.ID == "" {
		t.Fatal("expected to find open incident")
	}

	if incident.Status != "open" {
		t.Errorf("expected Status 'open', got %s", incident.Status)
	}

	// Resolve the incident
	err = manager.ResolveIncident(incident.ID, "test-user")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	// Should not find any open incidents now
	incident, err = repo.FindOpenIncident(checkID)
	if err != nil {
		t.Fatalf("FindOpenIncident failed: %v", err)
	}

	if incident.ID != "" {
		t.Error("expected no open incident after resolution")
	}
}

// TestStateTransitions tests incident state transitions
func TestStateTransitions(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "API down", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	incidents, _ := repo.ListIncidents()
	incident := incidents[0]

	// Test open -> acknowledged transition
	if incident.Status != "open" {
		t.Errorf("expected initial Status 'open', got %s", incident.Status)
	}

	err = manager.AcknowledgeIncident(incident.ID, "user1")
	if err != nil {
		t.Fatalf("AcknowledgeIncident failed: %v", err)
	}

	incident, _ = repo.GetIncident(incident.ID)
	if incident.Status != "acknowledged" {
		t.Errorf("expected Status 'acknowledged', got %s", incident.Status)
	}

	if incident.AcknowledgedAt == nil {
		t.Error("expected AcknowledgedAt to be set")
	}

	if incident.AcknowledgedBy != "user1" {
		t.Errorf("expected AcknowledgedBy 'user1', got %s", incident.AcknowledgedBy)
	}

	// Test acknowledged -> resolved transition
	err = manager.ResolveIncident(incident.ID, "user2")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	incident, _ = repo.GetIncident(incident.ID)
	if incident.Status != "resolved" {
		t.Errorf("expected Status 'resolved', got %s", incident.Status)
	}

	if incident.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}

	if incident.ResolvedBy != "user2" {
		t.Errorf("expected ResolvedBy 'user2', got %s", incident.ResolvedBy)
	}
}

// TestOpenToResolvedDirectTransition tests direct open -> resolved transition
func TestOpenToResolvedDirectTransition(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "API down", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	incidents, _ := repo.ListIncidents()
	incident := incidents[0]

	// Direct open -> resolved transition
	err = manager.ResolveIncident(incident.ID, "admin")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	incident, _ = repo.GetIncident(incident.ID)
	if incident.Status != "resolved" {
		t.Errorf("expected Status 'resolved', got %s", incident.Status)
	}

	if incident.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

// TestCannotAcknowledgeResolvedIncident tests that resolved incidents cannot be acknowledged
func TestCannotAcknowledgeResolvedIncident(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create and resolve incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "API down", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	incidents, _ := repo.ListIncidents()
	incident := incidents[0]

	err = manager.ResolveIncident(incident.ID, "admin")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	// Try to acknowledge resolved incident
	err = manager.AcknowledgeIncident(incident.ID, "user1")
	if err == nil {
		t.Fatal("expected error when acknowledging resolved incident")
	}

	if err.Error() != "cannot acknowledge resolved incident" {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

// TestCannotResolveAlreadyResolvedIncident tests that resolved incidents cannot be resolved again
func TestCannotResolveAlreadyResolvedIncident(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create and resolve incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "API down", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	incidents, _ := repo.ListIncidents()
	incident := incidents[0]

	err = manager.ResolveIncident(incident.ID, "admin")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	// Try to resolve again
	err = manager.ResolveIncident(incident.ID, "admin2")
	if err == nil {
		t.Fatal("expected error when resolving already resolved incident")
	}

	if err.Error() != "incident already resolved" {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

// TestAutoResolveOnRecovery tests automatic incident resolution on recovery
func TestAutoResolveOnRecovery(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Create incident
	err := manager.ProcessAlert(checkID, "API Health Check", "api", "critical", "API down", nil)
	if err != nil {
		t.Fatalf("ProcessAlert failed: %v", err)
	}

	// Simulate recovery
	err = manager.AutoResolveOnRecovery(checkID)
	if err != nil {
		t.Fatalf("AutoResolveOnRecovery failed: %v", err)
	}

	// Verify incident was resolved
	incidents, _ := repo.ListIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	if incident.Status != "resolved" {
		t.Errorf("expected Status 'resolved', got %s", incident.Status)
	}

	if incident.ResolvedBy != "system" {
		t.Errorf("expected ResolvedBy 'system', got %s", incident.ResolvedBy)
	}
}

// TestAutoResolveWithNoOpenIncident tests auto-resolve when no incident exists
func TestAutoResolveWithNoOpenIncident(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checkID := "check-1"

	// Try to auto-resolve when no incident exists
	err := manager.AutoResolveOnRecovery(checkID)
	if err != nil {
		t.Fatalf("AutoResolveOnRecovery failed: %v", err)
	}

	// Verify no incidents were created
	incidents, _ := repo.ListIncidents()
	if len(incidents) != 0 {
		t.Fatalf("expected 0 incidents, got %d", len(incidents))
	}
}

// TestSeverityIncrease tests that incident severity increases appropriately
func TestSeverityIncrease(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		new      string
		expected bool
	}{
		{"warning to critical", "warning", "critical", true},
		{"critical to warning", "critical", "warning", false},
		{"warning to warning", "warning", "warning", false},
		{"critical to critical", "critical", "critical", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := severityHasIncreased(tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("severityHasIncreased(%q, %q) = %v, want %v", tt.old, tt.new, result, tt.expected)
			}
		})
	}
}

// TestMultipleChecksWithIncidents tests managing incidents for multiple checks
func TestMultipleChecksWithIncidents(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	// Create incidents for different checks
	checks := []struct {
		id       string
		name     string
		severity string
	}{
		{"check-1", "API 1", "critical"},
		{"check-2", "API 2", "warning"},
		{"check-3", "Process 1", "critical"},
	}

	for _, check := range checks {
		err := manager.ProcessAlert(check.id, check.name, "api", check.severity, "Issue detected", nil)
		if err != nil {
			t.Fatalf("ProcessAlert failed for %s: %v", check.id, err)
		}
	}

	// Verify all incidents were created
	incidents, err := repo.ListIncidents()
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}

	if len(incidents) != len(checks) {
		t.Fatalf("expected %d incidents, got %d", len(checks), len(incidents))
	}

	// Find the incident for check-1 and resolve it
	var check1Incident Incident
	for _, inc := range incidents {
		if inc.CheckID == "check-1" {
			check1Incident = inc
			break
		}
	}

	if check1Incident.ID == "" {
		t.Fatal("could not find incident for check-1")
	}

	err = manager.ResolveIncident(check1Incident.ID, "admin")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	// Verify we can still find open incidents for the other checks
	for _, checkID := range []string{"check-2", "check-3"} {
		incident, err := repo.FindOpenIncident(checkID)
		if err != nil {
			t.Fatalf("FindOpenIncident failed for %s: %v", checkID, err)
		}

		if incident.ID == "" {
			t.Errorf("expected to find open incident for %s", checkID)
		}
	}

	// Verify check-1 no longer has an open incident
	incident, err := repo.FindOpenIncident("check-1")
	if err != nil {
		t.Fatalf("FindOpenIncident failed for check-1: %v", err)
	}

	if incident.ID != "" {
		t.Error("expected no open incident for check-1 after resolution")
	}
}

// TestIncidentIDGeneration tests that incident IDs are unique
func TestIncidentIDGeneration(t *testing.T) {
	now := time.Now().UTC()

	id1 := generateIncidentID("check-1", now)
	id2 := generateIncidentID("check-1", now.Add(1*time.Second))
	id3 := generateIncidentID("check-2", now)

	if id1 == id2 {
		t.Error("expected different IDs for different timestamps")
	}

	if id1 == id3 {
		t.Error("expected different IDs for different checks")
	}

	// Verify ID format
	expectedPattern := "check-1-"
	if len(id1) <= len(expectedPattern) {
		t.Error("incident ID format unexpected")
	}
}
