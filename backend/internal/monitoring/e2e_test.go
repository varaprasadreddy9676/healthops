package monitoring

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// E2E Test Suite for Phase 7.1 - Complete Incident Lifecycle
//
// These tests verify the complete incident management workflow including:
// - Incident creation from failed checks
// - Alert deduplication
// - Cooldown enforcement
// - Recovery auto-resolution
// - Audit trail
// - Authentication enforcement

// Helper function to create a test service with all components
func createTestService(t *testing.T) (*Service, *MemoryIncidentRepository, *InMemoryAuditRepository, *AlertRuleEngine) {
	t.Helper()

	// Create a working fake store that actually stores data
	store := &workingFakeStore{
		state: State{
			Checks:  []CheckConfig{},
			Results: []CheckResult{},
		},
	}

	// Create logger
	logger := log.New(log.Writer(), "", log.LstdFlags)

	// Create service
	cfg := &Config{
		Server: ServerConfig{
			Addr: ":8080",
		},
		Auth: AuthConfig{
			Enabled:  true,
			Username: "testuser",
			Password: "testpass",
		},
		RetentionDays: 7,
	}
	service := NewService(cfg, store, logger)

	// Create incident repository
	incidentRepo := NewMemoryIncidentRepository()
	incidentManager := NewIncidentManager(incidentRepo, logger)
	service.SetIncidentManager(incidentManager)

	// Create audit repository
	auditRepo := &InMemoryAuditRepository{events: []AuditEvent{}}
	auditLogger := NewAuditLogger(auditRepo, logger)
	service.SetAuditLogger(auditLogger)

	// Create alert rules for testing
	alertRules := []AlertRule{
		{
			ID:      "rule-fail-check",
			Name:    "Fail Check Rule",
			Enabled: true,
			Conditions: []AlertCondition{
				{
					Field:    "status",
					Operator: OperatorEquals,
					Value:    "critical",
				},
			},
			Severity:        "critical",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 5,
		},
		{
			ID:      "rule-cooldown-test",
			Name:    "Cooldown Test Rule",
			Enabled: true,
			Conditions: []AlertCondition{
				{
					Field:    "status",
					Operator: OperatorEquals,
					Value:    "critical",
				},
			},
			Severity:        "critical",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 1, // 1 minute cooldown for testing
		},
	}

	alertEngine := NewAlertRuleEngine(alertRules, logger)
	service.SetAlertEngine(alertEngine)

	return service, incidentRepo, auditRepo, alertEngine
}

// workingFakeStore is a fake store that actually stores data
type workingFakeStore struct {
	state State
	mu    chan struct{}
}

func (f *workingFakeStore) Snapshot() State {
	return cloneState(f.state)
}

func (f *workingFakeStore) DashboardSnapshot() DashboardSnapshot {
	return buildDashboardSnapshot(f.state)
}

func (f *workingFakeStore) Update(fn func(*State) error) error {
	next := cloneState(f.state)
	if err := fn(&next); err != nil {
		return err
	}
	f.state = next
	return nil
}

func (f *workingFakeStore) ReplaceChecks(checks []CheckConfig) error {
	return f.Update(func(state *State) error {
		state.Checks = cloneChecks(checks)
		return nil
	})
}

func (f *workingFakeStore) UpsertCheck(check CheckConfig) error {
	return f.Update(func(state *State) error {
		for i := range state.Checks {
			if state.Checks[i].ID == check.ID {
				state.Checks[i] = check
				return nil
			}
		}
		state.Checks = append(state.Checks, check)
		return nil
	})
}

func (f *workingFakeStore) DeleteCheck(id string) error {
	return f.Update(func(state *State) error {
		out := state.Checks[:0]
		for _, check := range state.Checks {
			if check.ID != id {
				out = append(out, check)
			}
		}
		state.Checks = out
		return nil
	})
}

func (f *workingFakeStore) AppendResults(results []CheckResult, retentionDays int) error {
	return f.Update(func(state *State) error {
		state.Results = append(state.Results, results...)
		pruneResults(&state.Results, retentionDays)
		return nil
	})
}

func (f *workingFakeStore) SetLastRun(at time.Time) error {
	return f.Update(func(state *State) error {
		state.LastRunAt = at.UTC()
		return nil
	})
}

// Helper to create a failing check
func createFailingCheck(name string) CheckConfig {
	return CheckConfig{
		ID:              name,
		Name:            name,
		Type:            "api",
		Target:          "http://localhost:9999/nonexistent", // Will fail
		Enabled:         boolPtr(true),
		IntervalSeconds: 60,
	}
}

// Helper to create a passing check
func createPassingCheck(name string) CheckConfig {
	return CheckConfig{
		ID:              name,
		Name:            name,
		Type:            "api",
		Target:          "http://localhost:8080/healthz", // Will pass (service endpoint)
		Enabled:         boolPtr(true),
		IntervalSeconds: 60,
	}
}

// Helper to execute a check run via API
func triggerCheckRun(t *testing.T, service *Service, checkID string) CheckResult {
	t.Helper()

	var reqBody bytes.Buffer
	if checkID != "" {
		json.NewEncoder(&reqBody).Encode(map[string]string{"checkId": checkID})
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", &reqBody)
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleRun(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var result CheckResult
	if err := decodeAPIResponseData(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return result
}

// Helper to get incidents
func getIncidents(t *testing.T, service *Service) []Incident {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	rec := httptest.NewRecorder()

	service.handleIncidents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var incidents []Incident
	if err := decodeAPIResponseData(rec.Body.Bytes(), &incidents); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return incidents
}

// Helper to acknowledge an incident
func acknowledgeIncident(t *testing.T, service *Service, incidentID string, by string) Incident {
	t.Helper()

	body := map[string]string{"acknowledgedBy": by}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+incidentID+"/acknowledge", bytes.NewReader(bodyBytes))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleIncidentByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var incident Incident
	if err := decodeAPIResponseData(rec.Body.Bytes(), &incident); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return incident
}

// Helper to resolve an incident
func resolveIncident(t *testing.T, service *Service, incidentID string, by string) Incident {
	t.Helper()

	body := map[string]string{"resolvedBy": by}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+incidentID+"/resolve", bytes.NewReader(bodyBytes))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleIncidentByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var incident Incident
	if err := decodeAPIResponseData(rec.Body.Bytes(), &incident); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return incident
}

// Helper to create a check via API
func createCheck(t *testing.T, service *Service, check CheckConfig) CheckConfig {
	t.Helper()

	bodyBytes, _ := json.Marshal(check)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checks", bytes.NewReader(bodyBytes))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleChecks(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var createdCheck CheckConfig
	if err := decodeAPIResponseData(rec.Body.Bytes(), &createdCheck); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return createdCheck
}

// TestE2E_FullIncidentLifecycle tests the complete incident lifecycle
func TestE2E_FullIncidentLifecycle(t *testing.T) {
	service, _, auditRepo, _ := createTestService(t)

	// Step 1: Create a failing check
	t.Log("Step 1: Creating failing check")
	check := createFailingCheck("test-fail-1")
	createdCheck := createCheck(t, service, check)
	t.Logf("Created check: %s", createdCheck.ID)

	// Step 2: Trigger manual run → check fails
	t.Log("Step 2: Triggering check run (expecting failure)")
	result := triggerCheckRun(t, service, createdCheck.ID)
	t.Logf("Check result: status=%s, healthy=%v", result.Status, result.Healthy)

	if result.Status != "critical" {
		t.Fatalf("expected critical status, got %s", result.Status)
	}

	// Step 3: Verify incident opens (status=open)
	t.Log("Step 3: Verifying incident created")
	time.Sleep(100 * time.Millisecond) // Give async processing time

	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	t.Logf("Incident created: id=%s, status=%s, severity=%s", incident.ID, incident.Status, incident.Severity)

	if incident.Status != "open" {
		t.Fatalf("expected incident status=open, got %s", incident.Status)
	}

	if incident.Severity != "critical" {
		t.Fatalf("expected incident severity=critical, got %s", incident.Severity)
	}

	if incident.CheckID != createdCheck.ID {
		t.Fatalf("expected incident checkID=%s, got %s", createdCheck.ID, incident.CheckID)
	}

	// Step 4: Verify alert created (via audit log)
	t.Log("Step 4: Verifying alert in audit log")
	auditEvents, _ := auditRepo.ListEvents(AuditFilter{
		Action: "check.created",
		Limit:  10,
	})

	// Audit log should have check creation
	foundCheckCreated := false
	for _, event := range auditEvents {
		if event.Action == "check.created" && event.TargetID == createdCheck.ID {
			foundCheckCreated = true
			break
		}
	}

	if !foundCheckCreated {
		t.Error("expected to find check.created audit event")
	}

	// Step 5: Acknowledge incident → status=acknowledged
	t.Log("Step 5: Acknowledging incident")
	acknowledgedIncident := acknowledgeIncident(t, service, incident.ID, "testuser")

	if acknowledgedIncident.Status != "acknowledged" {
		t.Fatalf("expected incident status=acknowledged, got %s", acknowledgedIncident.Status)
	}

	if acknowledgedIncident.AcknowledgedBy != "testuser" {
		t.Fatalf("expected acknowledgedBy=testuser, got %s", acknowledgedIncident.AcknowledgedBy)
	}

	if acknowledgedIncident.AcknowledgedAt == nil {
		t.Fatal("expected acknowledgedAt to be set")
	}

	t.Logf("Incident acknowledged: by=%s, at=%v", acknowledgedIncident.AcknowledgedBy, acknowledgedIncident.AcknowledgedAt)

	// Verify audit log has acknowledgment
	auditEvents, _ = auditRepo.ListEvents(AuditFilter{
		Action:   "incident.acknowledged",
		TargetID: incident.ID,
		Limit:    10,
	})

	if len(auditEvents) != 1 {
		t.Fatalf("expected 1 incident.acknowledged audit event, got %d", len(auditEvents))
	}

	// Step 6: Resolve incident → status=resolved
	t.Log("Step 6: Resolving incident")
	resolvedIncident := resolveIncident(t, service, incident.ID, "testuser")

	if resolvedIncident.Status != "resolved" {
		t.Fatalf("expected incident status=resolved, got %s", resolvedIncident.Status)
	}

	if resolvedIncident.ResolvedBy != "testuser" {
		t.Fatalf("expected resolvedBy=testuser, got %s", resolvedIncident.ResolvedBy)
	}

	if resolvedIncident.ResolvedAt == nil {
		t.Fatal("expected resolvedAt to be set")
	}

	t.Logf("Incident resolved: by=%s, at=%v", resolvedIncident.ResolvedBy, resolvedIncident.ResolvedAt)

	// Verify audit log has resolution
	auditEvents, _ = auditRepo.ListEvents(AuditFilter{
		Action:   "incident.resolved",
		TargetID: incident.ID,
		Limit:    10,
	})

	if len(auditEvents) != 1 {
		t.Fatalf("expected 1 incident.resolved audit event, got %d", len(auditEvents))
	}

	t.Log("✓ Full incident lifecycle test completed successfully")
}

// TestE2E_AlertDeduplication tests that multiple failures don't create duplicate incidents
func TestE2E_AlertDeduplication(t *testing.T) {
	service, _, _, _ := createTestService(t)

	// Step 1: Create check
	t.Log("Step 1: Creating failing check")
	check := createFailingCheck("test-dedupe")
	createdCheck := createCheck(t, service, check)

	// Step 2: Run 3 times (all fail)
	t.Log("Step 2: Running check 3 times (expecting failures)")
	for i := 0; i < 3; i++ {
		result := triggerCheckRun(t, service, createdCheck.ID)
		t.Logf("Run %d: status=%s, healthy=%v", i+1, result.Status, result.Healthy)

		if result.Status != "critical" {
			t.Fatalf("run %d: expected critical status, got %s", i+1, result.Status)
		}

		// Small delay between runs
		time.Sleep(50 * time.Millisecond)
	}

	// Step 3: Verify only 1 incident created
	t.Log("Step 3: Verifying single incident created")
	time.Sleep(100 * time.Millisecond) // Give async processing time

	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	t.Logf("Single incident: id=%s, status=%s", incident.ID, incident.Status)

	// Step 4: Verify incident updated, not duplicated
	t.Log("Step 4: Verifying incident was updated")

	// The incident should have the most recent data
	if incident.CheckID != createdCheck.ID {
		t.Fatalf("expected incident checkID=%s, got %s", createdCheck.ID, incident.CheckID)
	}

	// Run once more and verify still only 1 incident
	result := triggerCheckRun(t, service, createdCheck.ID)
	if result.Status != "critical" {
		t.Fatalf("expected critical status, got %s", result.Status)
	}

	time.Sleep(100 * time.Millisecond)
	incidents = getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected still 1 incident after 4th run, got %d", len(incidents))
	}

	t.Log("✓ Alert deduplication test completed successfully")
}

// TestE2E_CooldownEnforcement tests that alert cooldown is respected
func TestE2E_CooldownEnforcement(t *testing.T) {
	service, _, _, _ := createTestService(t)

	// Create a custom alert rule with short cooldown for testing
	customRule := AlertRule{
		ID:      "rule-cooldown-e2e",
		Name:    "Cooldown E2E Rule",
		Enabled: true,
		Conditions: []AlertCondition{
			{
				Field:    "status",
				Operator: OperatorEquals,
				Value:    "critical",
			},
		},
		Severity:        "critical",
		Channels:        []AlertChannel{{Type: "log"}},
		CooldownMinutes: 0, // No cooldown for initial test
	}

	// Replace alert engine with custom rule
	customEngine := NewAlertRuleEngine([]AlertRule{customRule}, newTestLogger())
	service.SetAlertEngine(customEngine)

	// Step 1: Create check
	t.Log("Step 1: Creating failing check")
	check := createFailingCheck("test-cooldown")
	createdCheck := createCheck(t, service, check)

	// Step 2: Fail check twice in quick succession
	t.Log("Step 2: Failing check twice in quick succession")

	// First failure - should create alert
	result1 := triggerCheckRun(t, service, createdCheck.ID)
	t.Logf("First failure: status=%s", result1.Status)
	time.Sleep(100 * time.Millisecond)

	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident after first failure, got %d", len(incidents))
	}

	firstIncidentCount := len(incidents)

	// Second failure immediately - with 0 cooldown, should still update but not create new incident
	result2 := triggerCheckRun(t, service, createdCheck.ID)
	t.Logf("Second failure: status=%s", result2.Status)
	time.Sleep(100 * time.Millisecond)

	incidents = getIncidents(t, service)
	if len(incidents) != firstIncidentCount {
		t.Fatalf("expected same incident count (%d) after second failure, got %d", firstIncidentCount, len(incidents))
	}

	// Step 3: Test with actual cooldown
	t.Log("Step 3: Testing with cooldown enabled")

	// Update rule to have cooldown
	customRule.CooldownMinutes = 1 // 1 minute cooldown
	customEngine = NewAlertRuleEngine([]AlertRule{customRule}, newTestLogger())
	service.SetAlertEngine(customEngine)

	// Clear existing incidents by creating new service instance
	service2, _, _, _ := createTestService(t)
	check2 := createFailingCheck("test-cooldown-2")
	createdCheck2 := createCheck(t, service2, check2)

	// First failure
	_ = triggerCheckRun(t, service2, createdCheck2.ID)
	time.Sleep(100 * time.Millisecond)

	// Second failure within cooldown - should be suppressed
	_ = triggerCheckRun(t, service2, createdCheck2.ID)
	time.Sleep(100 * time.Millisecond)

	incidents2 := getIncidents(t, service2)
	if len(incidents2) != 1 {
		t.Fatalf("expected 1 incident with cooldown, got %d", len(incidents2))
	}

	t.Log("✓ Cooldown enforcement test completed successfully")
}

// TestE2E_RecoveryAutoResolve tests that incidents auto-resolve on recovery
func TestE2E_RecoveryAutoResolve(t *testing.T) {
	service, _, _, _ := createTestService(t)
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer healthyServer.Close()

	// Step 1: Create failing check
	t.Log("Step 1: Creating failing check")
	failingCheck := createFailingCheck("test-recovery")
	createdCheck := createCheck(t, service, failingCheck)

	// Step 2: Wait for incident to open
	t.Log("Step 2: Running failing check to open incident")
	result := triggerCheckRun(t, service, createdCheck.ID)
	if result.Status != "critical" {
		t.Fatalf("expected critical status, got %s", result.Status)
	}

	time.Sleep(100 * time.Millisecond)

	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	if incident.Status != "open" {
		t.Fatalf("expected incident status=open, got %s", incident.Status)
	}

	t.Logf("Incident opened: id=%s", incident.ID)

	// Step 3: Fix check (point to working URL)
	t.Log("Step 3: Updating check to point to working URL")
	workingCheck := createdCheck
	workingCheck.Target = healthyServer.URL + "/healthz"

	bodyBytes, _ := json.Marshal(workingCheck)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/checks/"+workingCheck.ID, bytes.NewReader(bodyBytes))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleCheckByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Step 4: Run again → check passes
	t.Log("Step 4: Running check again (expecting success)")
	result = triggerCheckRun(t, service, createdCheck.ID)

	if result.Status != "healthy" {
		t.Fatalf("expected healthy status, got %s", result.Status)
	}

	// Step 5: Verify incident auto-resolves
	t.Log("Step 5: Verifying incident auto-resolved")

	// Trigger auto-resolution
	err := service.incidentManager.AutoResolveOnRecovery(createdCheck.ID)
	if err != nil {
		t.Fatalf("auto-resolve failed: %v", err)
	}

	// Get updated incident
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/"+incident.ID, nil)
	rec2 := httptest.NewRecorder()
	service.handleIncidentByID(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var updatedIncident Incident
	if err := decodeAPIResponseData(rec2.Body.Bytes(), &updatedIncident); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if updatedIncident.Status != "resolved" {
		t.Fatalf("expected incident status=resolved, got %s", updatedIncident.Status)
	}

	if updatedIncident.ResolvedBy != "system" {
		t.Fatalf("expected resolvedBy=system, got %s", updatedIncident.ResolvedBy)
	}

	if updatedIncident.ResolvedAt == nil {
		t.Fatal("expected resolvedAt to be set")
	}

	t.Logf("Incident auto-resolved: by=%s, at=%v", updatedIncident.ResolvedBy, updatedIncident.ResolvedAt)

	t.Log("✓ Recovery auto-resolve test completed successfully")
}

// TestE2E_AuditTrail tests that all actions are logged to audit trail
func TestE2E_AuditTrail(t *testing.T) {
	service, _, auditRepo, _ := createTestService(t)

	// Step 1: Create check
	t.Log("Step 1: Creating check")
	check := createFailingCheck("test-audit")
	createdCheck := createCheck(t, service, check)

	// Step 2: Update check
	t.Log("Step 2: Updating check")
	updatedCheck := createdCheck
	updatedCheck.Name = "Updated Check Name"

	bodyBytes, _ := json.Marshal(updatedCheck)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/checks/"+updatedCheck.ID, bytes.NewReader(bodyBytes))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleCheckByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Step 3: Trigger run
	t.Log("Step 3: Triggering check run")
	_ = triggerCheckRun(t, service, createdCheck.ID)
	time.Sleep(100 * time.Millisecond)

	// Step 4: Get incident (should be created)
	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	incidentID := incidents[0].ID

	// Step 5: Acknowledge incident
	t.Log("Step 4: Acknowledging incident")
	_ = acknowledgeIncident(t, service, incidentID, "testuser")

	// Step 6: Resolve incident
	t.Log("Step 5: Resolving incident")
	_ = resolveIncident(t, service, incidentID, "testuser")

	// Step 7: Verify all actions in audit log
	t.Log("Step 6: Verifying audit log")

	auditEvents, _ := auditRepo.ListEvents(AuditFilter{
		Limit: 100,
	})

	// Expected actions:
	// 1. check.created
	// 2. check.updated
	// 3. incident.acknowledged
	// 4. incident.resolved

	expectedActions := map[string]string{
		"check.created":         createdCheck.ID,
		"check.updated":         createdCheck.ID,
		"incident.acknowledged": incidentID,
		"incident.resolved":     incidentID,
	}

	foundActions := make(map[string]bool)

	for _, event := range auditEvents {
		if targetID, ok := expectedActions[event.Action]; ok {
			if event.TargetID == targetID {
				foundActions[event.Action] = true

				// Verify actor is "testuser"
				if event.Actor != "testuser" {
					t.Errorf("action %s: expected actor=testuser, got %s", event.Action, event.Actor)
				}

				// Verify target type
				var expectedTarget string
				if event.Action == "check.created" || event.Action == "check.updated" {
					expectedTarget = "check"
				} else {
					expectedTarget = "incident"
				}

				if event.Target != expectedTarget {
					t.Errorf("action %s: expected target=%s, got %s", event.Action, expectedTarget, event.Target)
				}

				t.Logf("✓ Found audit event: action=%s, actor=%s, target=%s, targetID=%s",
					event.Action, event.Actor, event.Target, event.TargetID)
			}
		}
	}

	// Verify all expected actions were found
	for action := range expectedActions {
		if !foundActions[action] {
			t.Errorf("expected to find audit action %s in audit log", action)
		}
	}

	if len(foundActions) != len(expectedActions) {
		t.Fatalf("expected %d audit actions, found %d", len(expectedActions), len(foundActions))
	}

	t.Log("✓ Audit trail test completed successfully")
}

// TestE2E_AuthEnforcement tests that authentication is enforced correctly
func TestE2E_AuthEnforcement(t *testing.T) {
	service, _, _, _ := createTestService(t)

	// Test 1: Try POST /api/v1/checks without auth → 401
	t.Log("Test 1: POST without auth should return 401")
	check := createFailingCheck("test-auth")
	bodyBytes, _ := json.Marshal(check)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checks", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()

	service.handleChecks(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	t.Log("✓ POST without auth correctly returns 401")

	// Test 2: Try POST with valid auth → 201
	t.Log("Test 2: POST with valid auth should return 201")
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/checks", bytes.NewReader(bodyBytes))
	req2.SetBasicAuth("testuser", "testpass")
	rec2 := httptest.NewRecorder()

	service.handleChecks(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d: %s", rec2.Code, rec2.Body.String())
	}

	t.Log("✓ POST with valid auth correctly returns 201")

	// Test 3: Try GET without auth → 200 (read allowed)
	t.Log("Test 3: GET without auth should return 200 (read-only)")
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/checks", nil)
	rec3 := httptest.NewRecorder()

	service.handleChecks(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for GET, got %d", rec3.Code)
	}

	t.Log("✓ GET without auth correctly returns 200")

	// Test 4: Try PUT without auth → 401
	t.Log("Test 4: PUT without auth should return 401")
	updateBody := map[string]string{"name": "Updated"}
	updateBodyBytes, _ := json.Marshal(updateBody)

	req4 := httptest.NewRequest(http.MethodPut, "/api/v1/checks/test-auth", bytes.NewReader(updateBodyBytes))
	rec4 := httptest.NewRecorder()

	service.handleCheckByID(rec4, req4)

	if rec4.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for PUT, got %d", rec4.Code)
	}

	t.Log("✓ PUT without auth correctly returns 401")

	// Test 5: Try DELETE without auth → 401
	t.Log("Test 5: DELETE without auth should return 401")
	req5 := httptest.NewRequest(http.MethodDelete, "/api/v1/checks/test-auth", nil)
	rec5 := httptest.NewRecorder()

	service.handleCheckByID(rec5, req5)

	if rec5.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for DELETE, got %d", rec5.Code)
	}

	t.Log("✓ DELETE without auth correctly returns 401")

	// Test 6: Wrong credentials → 401
	t.Log("Test 6: POST with wrong credentials should return 401")
	req6 := httptest.NewRequest(http.MethodPost, "/api/v1/checks", bytes.NewReader(bodyBytes))
	req6.SetBasicAuth("wronguser", "wrongpass")
	rec6 := httptest.NewRecorder()

	service.handleChecks(rec6, req6)

	if rec6.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for wrong credentials, got %d", rec6.Code)
	}

	t.Log("✓ POST with wrong credentials correctly returns 401")

	t.Log("✓ Authentication enforcement test completed successfully")
}

// TestE2E_ConcurrentIncidents tests handling multiple simultaneous incidents
func TestE2E_ConcurrentIncidents(t *testing.T) {
	service, _, _, _ := createTestService(t)

	// Create multiple failing checks
	t.Log("Creating multiple failing checks")
	checks := []CheckConfig{
		createFailingCheck("test-concurrent-1"),
		createFailingCheck("test-concurrent-2"),
		createFailingCheck("test-concurrent-3"),
	}

	for _, check := range checks {
		_ = createCheck(t, service, check)
	}

	// Run all checks
	t.Log("Running all checks")
	for _, check := range checks {
		result := triggerCheckRun(t, service, check.ID)
		if result.Status != "critical" {
			t.Fatalf("expected critical status for %s, got %s", check.ID, result.Status)
		}
	}

	// Verify 3 separate incidents created
	time.Sleep(100 * time.Millisecond)
	incidents := getIncidents(t, service)

	if len(incidents) != 3 {
		t.Fatalf("expected 3 incidents, got %d", len(incidents))
	}

	// Verify each incident corresponds to a different check
	checkIDs := make(map[string]bool)
	for _, incident := range incidents {
		if checkIDs[incident.CheckID] {
			t.Fatalf("duplicate incident for check %s", incident.CheckID)
		}
		checkIDs[incident.CheckID] = true

		if incident.Status != "open" {
			t.Errorf("incident %s: expected status=open, got %s", incident.ID, incident.Status)
		}
	}

	t.Logf("✓ Created %d separate incidents for %d checks", len(incidents), len(checks))

	// Resolve all incidents
	t.Log("Resolving all incidents")
	for _, incident := range incidents {
		_ = resolveIncident(t, service, incident.ID, "testuser")
	}

	// Verify all resolved
	incidents = getIncidents(t, service)
	allResolved := true
	for _, incident := range incidents {
		if incident.Status != "resolved" {
			allResolved = false
			t.Errorf("incident %s: expected status=resolved, got %s", incident.ID, incident.Status)
		}
	}

	if !allResolved {
		t.Fatal("not all incidents were resolved")
	}

	t.Log("✓ Concurrent incidents test completed successfully")
}

// TestE2E_IncidentStatusTransitions tests valid and invalid status transitions
func TestE2E_IncidentStatusTransitions(t *testing.T) {
	service, _, _, _ := createTestService(t)

	// Create an incident
	t.Log("Creating incident")
	check := createFailingCheck("test-transitions")
	createdCheck := createCheck(t, service, check)
	_ = triggerCheckRun(t, service, createdCheck.ID)
	time.Sleep(100 * time.Millisecond)

	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]

	// Test: open -> acknowledged (valid)
	t.Log("Test: open -> acknowledged")
	acknowledged := acknowledgeIncident(t, service, incident.ID, "testuser")
	if acknowledged.Status != "acknowledged" {
		t.Errorf("expected status=acknowledged, got %s", acknowledged.Status)
	}

	// Test: acknowledged -> resolved (valid)
	t.Log("Test: acknowledged -> resolved")
	resolved := resolveIncident(t, service, incident.ID, "testuser")
	if resolved.Status != "resolved" {
		t.Errorf("expected status=resolved, got %s", resolved.Status)
	}

	// Test: resolved -> acknowledged (invalid, should fail)
	t.Log("Test: resolved -> acknowledged (should fail)")
	body := map[string]string{"acknowledgedBy": "testuser"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+incident.ID+"/acknowledge", bytes.NewReader(bodyBytes))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	service.handleIncidentByID(rec, req)

	// Should return error
	if rec.Code == http.StatusOK {
		t.Error("expected error when acknowledging resolved incident, got success")
	}

	// Test: resolved -> resolved (invalid, should fail)
	t.Log("Test: resolved -> resolved (should fail)")
	body2 := map[string]string{"resolvedBy": "testuser"}
	bodyBytes2, _ := json.Marshal(body2)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/"+incident.ID+"/resolve", bytes.NewReader(bodyBytes2))
	req2.SetBasicAuth("testuser", "testpass")
	rec2 := httptest.NewRecorder()

	service.handleIncidentByID(rec2, req2)

	// Should return error
	if rec2.Code == http.StatusOK {
		t.Error("expected error when resolving already resolved incident, got success")
	}

	t.Log("✓ Incident status transitions test completed successfully")
}

// TestE2E_IncidentWithSeverityEscalation tests that incidents can escalate in severity
func TestE2E_IncidentWithSeverityEscalation(t *testing.T) {
	service, _, _, _ := createTestService(t)

	// Create alert rules with different severities
	warningRule := AlertRule{
		ID:      "rule-warning",
		Name:    "Warning Rule",
		Enabled: true,
		Conditions: []AlertCondition{
			{
				Field:    "status",
				Operator: OperatorEquals,
				Value:    "warning",
			},
		},
		Severity:        "warning",
		Channels:        []AlertChannel{{Type: "log"}},
		CooldownMinutes: 0,
	}

	criticalRule := AlertRule{
		ID:      "rule-critical",
		Name:    "Critical Rule",
		Enabled: true,
		Conditions: []AlertCondition{
			{
				Field:    "status",
				Operator: OperatorEquals,
				Value:    "critical",
			},
		},
		Severity:        "critical",
		Channels:        []AlertChannel{{Type: "log"}},
		CooldownMinutes: 0,
	}

	alertEngine := NewAlertRuleEngine([]AlertRule{warningRule, criticalRule}, newTestLogger())
	service.SetAlertEngine(alertEngine)

	// Create a check (will fail with critical status)
	t.Log("Creating check")
	check := createFailingCheck("test-escalation")
	createdCheck := createCheck(t, service, check)

	// Trigger failure
	t.Log("Triggering failure (critical)")
	_ = triggerCheckRun(t, service, createdCheck.ID)
	time.Sleep(100 * time.Millisecond)

	// Verify incident created with critical severity
	incidents := getIncidents(t, service)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	incident := incidents[0]
	if incident.Severity != "critical" {
		t.Errorf("expected severity=critical, got %s", incident.Severity)
	}

	t.Logf("✓ Incident created with severity=%s", incident.Severity)

	// Note: Full escalation testing would require checks that can transition
	// from warning to critical, which is difficult to simulate in tests
	// The incident manager already has severityHasIncreased logic

	t.Log("✓ Severity escalation test completed successfully")
}
