package monitoring

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestAlertIncidentChainWithPluginResults verifies the full pipeline:
// plugin-dispatched check result → AlertRuleEngine → IncidentManager → callback
func TestAlertIncidentChainWithPluginResults(t *testing.T) {
	// Setup: alert rule that fires on any critical status
	rules := []AlertRule{
		{
			ID:       "rule-all-critical",
			Name:     "All Critical",
			Enabled:  true,
			CheckIDs: []string{}, // applies to all
			Conditions: []AlertCondition{
				{Field: "status", Operator: OperatorEquals, Value: "critical"},
			},
			Severity:        "critical",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 0,
		},
	}

	logger := log.New(os.Stderr, "test ", log.LstdFlags)
	engine := NewAlertRuleEngine(rules, &mockLogger{})
	repo := NewMemoryIncidentRepository()
	incidentManager := NewIncidentManager(repo, logger)

	// Track callbacks
	var callbackFired atomic.Int32
	var lastIncidentID string
	incidentManager.SetOnIncidentCreated(func(incident Incident) {
		callbackFired.Add(1)
		lastIncidentID = incident.ID
	})

	// Simulate check result from a plugin (e.g. SSL check finding expired cert)
	checkResult := CheckResult{
		ID:         "ssl-check-1-123456",
		CheckID:    "ssl-check-1",
		Name:       "SSL Certificate",
		Type:       "ssl",
		Status:     "critical",
		Healthy:    false,
		DurationMs: 50,
		Message:    "certificate expired 2 days ago",
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
		Metrics: map[string]float64{
			"daysUntilExpiry": -2,
			"chainLength":     3,
		},
	}

	// Step 1: Alert engine evaluates results
	alerts := engine.Evaluate([]CheckResult{checkResult})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].CheckID != "ssl-check-1" {
		t.Errorf("alert.CheckID = %q; want 'ssl-check-1'", alerts[0].CheckID)
	}

	// Step 2: Feed alert into incident manager (same path as service.go callback)
	for _, alert := range alerts {
		metadata := map[string]string{
			"ruleId":   alert.RuleID,
			"ruleName": alert.RuleName,
			"message":  alert.Message,
		}
		err := incidentManager.ProcessAlert(
			alert.CheckID,
			alert.CheckName,
			checkResult.Type,
			alert.Severity,
			alert.Message,
			metadata,
		)
		if err != nil {
			t.Fatalf("ProcessAlert: %v", err)
		}
	}

	// Step 3: Verify incident was created
	incidents, err := repo.ListIncidents()
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	inc := incidents[0]
	if inc.CheckID != "ssl-check-1" {
		t.Errorf("incident.CheckID = %q; want 'ssl-check-1'", inc.CheckID)
	}
	if inc.Type != "ssl" {
		t.Errorf("incident.Type = %q; want 'ssl'", inc.Type)
	}
	if inc.Severity != "critical" {
		t.Errorf("incident.Severity = %q; want 'critical'", inc.Severity)
	}
	if inc.Status != "open" {
		t.Errorf("incident.Status = %q; want 'open'", inc.Status)
	}

	// Step 4: Verify callback fired (this is what enqueues AI analysis)
	if callbackFired.Load() != 1 {
		t.Errorf("callbackFired = %d; want 1", callbackFired.Load())
	}
	if lastIncidentID == "" {
		t.Error("lastIncidentID should be set by callback")
	}
}

// TestAlertIncidentChainDNSCheck verifies DNS check results flow through the pipeline.
func TestAlertIncidentChainDNSCheck(t *testing.T) {
	rules := []AlertRule{
		{
			ID:       "rule-dns-critical",
			Name:     "DNS Critical",
			Enabled:  true,
			CheckIDs: []string{"dns-check-1"},
			Conditions: []AlertCondition{
				{Field: "healthy", Operator: OperatorEquals, Value: false},
			},
			Severity:        "warning",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 0,
		},
	}

	engine := NewAlertRuleEngine(rules, &mockLogger{})
	repo := NewMemoryIncidentRepository()
	incidentManager := NewIncidentManager(repo, nil)

	var incidentCreated bool
	incidentManager.SetOnIncidentCreated(func(incident Incident) {
		incidentCreated = true
	})

	// DNS check failure
	result := CheckResult{
		ID:         "dns-check-1-999",
		CheckID:    "dns-check-1",
		Name:       "DNS Resolution",
		Type:       "dns",
		Status:     "critical",
		Healthy:    false,
		DurationMs: 5000,
		Message:    "expected A record 1.2.3.4 not found",
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
		Metrics:    map[string]float64{"latencyMs": 5000, "recordCount": 0},
	}

	alerts := engine.Evaluate([]CheckResult{result})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	for _, alert := range alerts {
		incidentManager.ProcessAlert(alert.CheckID, alert.CheckName, result.Type, alert.Severity, alert.Message, nil)
	}

	if !incidentCreated {
		t.Error("expected incident created callback for DNS check failure")
	}

	incidents, _ := repo.ListIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Type != "dns" {
		t.Errorf("incident.Type = %q; want 'dns'", incidents[0].Type)
	}
}

// TestAlertIncidentChainHeartbeatMissed verifies heartbeat miss flows through pipeline.
func TestAlertIncidentChainHeartbeatMissed(t *testing.T) {
	rules := []AlertRule{
		{
			ID:       "rule-heartbeat",
			Name:     "Heartbeat Miss",
			Enabled:  true,
			CheckIDs: []string{},
			Conditions: []AlertCondition{
				{Field: "status", Operator: OperatorEquals, Value: "critical"},
			},
			Severity:        "critical",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 0,
		},
	}

	engine := NewAlertRuleEngine(rules, &mockLogger{})
	repo := NewMemoryIncidentRepository()
	incidentManager := NewIncidentManager(repo, nil)

	var callbackCount atomic.Int32
	incidentManager.SetOnIncidentCreated(func(incident Incident) {
		callbackCount.Add(1)
	})

	// Heartbeat missed
	result := CheckResult{
		ID:         "heartbeat-1-111",
		CheckID:    "heartbeat-cron-backup",
		Name:       "Nightly Backup Heartbeat",
		Type:       "heartbeat",
		Status:     "critical",
		Healthy:    false,
		DurationMs: 1,
		Message:    "heartbeat is late: last ping 3600s ago, expected every 300s",
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
		Metrics:    map[string]float64{"lastPingAgeSeconds": 3600, "expectedIntervalSeconds": 300},
	}

	alerts := engine.Evaluate([]CheckResult{result})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for heartbeat miss, got %d", len(alerts))
	}

	for _, alert := range alerts {
		incidentManager.ProcessAlert(alert.CheckID, alert.CheckName, result.Type, alert.Severity, alert.Message, nil)
	}

	if callbackCount.Load() != 1 {
		t.Errorf("callbackCount = %d; want 1", callbackCount.Load())
	}
}

// TestAutoResolveOnRecoveryWithCallback verifies that a recovered check auto-resolves its incident.
func TestAutoResolveOnRecoveryWithCallback(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	incidentManager := NewIncidentManager(repo, nil)

	var resolvedCallback atomic.Int32
	incidentManager.SetOnIncidentResolved(func(incident Incident) {
		resolvedCallback.Add(1)
	})

	// Create an incident
	err := incidentManager.ProcessAlert("check-ssl", "SSL Check", "ssl", "critical", "cert expired", nil)
	if err != nil {
		t.Fatalf("ProcessAlert: %v", err)
	}

	// Verify incident is open
	incidents, _ := repo.ListIncidents()
	if len(incidents) != 1 || incidents[0].Status != "open" {
		t.Fatalf("expected 1 open incident")
	}

	// Auto-resolve on recovery
	err = incidentManager.AutoResolveOnRecovery("check-ssl")
	if err != nil {
		t.Fatalf("AutoResolveOnRecovery: %v", err)
	}

	// Verify resolved
	incidents, _ = repo.ListIncidents()
	if incidents[0].Status != "resolved" {
		t.Errorf("status = %q; want 'resolved'", incidents[0].Status)
	}

	if resolvedCallback.Load() != 1 {
		t.Errorf("resolvedCallback = %d; want 1", resolvedCallback.Load())
	}
}

// TestMaintenanceWindowSuppressesAlerts verifies the maintenance window integration.
func TestMaintenanceWindowSuppressesAlerts(t *testing.T) {
	dir := t.TempDir()
	maintenanceStore, err := NewMaintenanceStore(dir + "/maint.json")
	if err != nil {
		t.Fatalf("NewMaintenanceStore: %v", err)
	}

	now := time.Now().UTC()
	// Create active maintenance window for check-ssl
	maintenanceStore.Create(MaintenanceWindow{
		ID:        "mw-1",
		Name:      "SSL Renewal",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
		CheckIDs:  []string{"check-ssl"},
	})

	// Simulate the service.go logic: check if in maintenance before alerting
	check := CheckConfig{ID: "check-ssl", Name: "SSL Check", Type: "ssl"}
	if !maintenanceStore.IsCheckInMaintenance(check) {
		t.Fatal("check-ssl should be in maintenance")
	}

	// Non-maintenance check should not be suppressed
	check2 := CheckConfig{ID: "check-api", Name: "API Check", Type: "api"}
	if maintenanceStore.IsCheckInMaintenance(check2) {
		t.Fatal("check-api should NOT be in maintenance")
	}
}

// TestRunnerProducesCorrectResultsForAlertEngine tests that the runner
// produces CheckResults that the alert engine can process.
func TestRunnerProducesCorrectResultsForAlertEngine(t *testing.T) {
	// Create a test HTTP server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	store := &fakeStore{
		snapshot: State{
			Checks: []CheckConfig{
				{
					ID:             "api-failing",
					Name:           "Failing API",
					Type:           "api",
					Target:         server.URL,
					ExpectedStatus: 200,
					TimeoutSeconds: 5,
					Enabled:        boolPtr(true),
				},
			},
		},
	}

	cfg := &Config{Workers: 1, RetentionDays: 7}
	runner := NewRunner(cfg, store)

	ctx := context.Background()
	result := runner.executeCheck(ctx, store.snapshot.Checks[0])

	// The result should be unhealthy due to wrong status code
	if result.Healthy {
		t.Fatal("expected unhealthy result for 500 response")
	}
	if result.Status != "critical" {
		t.Errorf("status = %q; want critical", result.Status)
	}

	// Feed into alert engine
	rules := []AlertRule{
		{
			ID:       "rule-status-check",
			Name:     "Any Unhealthy",
			Enabled:  true,
			CheckIDs: []string{"api-failing"},
			Conditions: []AlertCondition{
				{Field: "healthy", Operator: OperatorEquals, Value: false},
			},
			Severity:        "critical",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 0,
		},
	}
	engine := NewAlertRuleEngine(rules, &mockLogger{})

	alerts := engine.Evaluate([]CheckResult{result})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert from failing API check, got %d", len(alerts))
	}
	if alerts[0].CheckID != "api-failing" {
		t.Errorf("alert.CheckID = %q; want 'api-failing'", alerts[0].CheckID)
	}
}

// TestAlertMetricsConditionWithPluginResult verifies that custom metrics from
// plugin checks (like daysUntilExpiry from SSL) can be used in alert conditions.
func TestAlertMetricsConditionWithPluginResult(t *testing.T) {
	rules := []AlertRule{
		{
			ID:       "rule-ssl-expiry",
			Name:     "SSL Expiring Soon",
			Enabled:  true,
			CheckIDs: []string{},
			Conditions: []AlertCondition{
				{Field: "daysUntilExpiry", Operator: OperatorLessThan, Value: float64(14)},
			},
			Severity:        "warning",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 0,
		},
	}

	engine := NewAlertRuleEngine(rules, &mockLogger{})

	// SSL check with cert expiring in 5 days
	result := CheckResult{
		ID:         "ssl-1-ts",
		CheckID:    "ssl-prod",
		Name:       "Prod SSL",
		Type:       "ssl",
		Status:     "warning",
		Healthy:    false,
		DurationMs: 100,
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
		Metrics:    map[string]float64{"daysUntilExpiry": 5, "chainLength": 3},
	}

	alerts := engine.Evaluate([]CheckResult{result})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for SSL expiry metric, got %d", len(alerts))
	}
	if alerts[0].Severity != "warning" {
		t.Errorf("severity = %q; want 'warning'", alerts[0].Severity)
	}

	// SSL check with 30 days remaining — should NOT fire
	result.Metrics["daysUntilExpiry"] = 30
	result.ID = "ssl-2-ts"
	alerts = engine.Evaluate([]CheckResult{result})
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for 30-day cert, got %d", len(alerts))
	}
}

// TestAlertDurationConditionWithPluginResult verifies latency-based alerts.
func TestAlertDurationConditionWithPluginResult(t *testing.T) {
	rules := []AlertRule{
		{
			ID:       "rule-slow-ping",
			Name:     "Slow Ping",
			Enabled:  true,
			CheckIDs: []string{"ping-prod"},
			Conditions: []AlertCondition{
				{Field: "durationMs", Operator: OperatorGreaterThan, Value: float64(500)},
			},
			Severity:        "warning",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 0,
		},
	}

	engine := NewAlertRuleEngine(rules, &mockLogger{})

	// Fast ping — no alert
	result := CheckResult{
		ID:         "ping-1",
		CheckID:    "ping-prod",
		Name:       "Ping Prod",
		Type:       "ping",
		Status:     "healthy",
		Healthy:    true,
		DurationMs: 50,
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
		Metrics:    map[string]float64{"avgLatencyMs": 10, "packetLossPercent": 0},
	}

	alerts := engine.Evaluate([]CheckResult{result})
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for fast ping, got %d", len(alerts))
	}

	// Slow ping — alert
	result.DurationMs = 1500
	result.ID = "ping-2"
	alerts = engine.Evaluate([]CheckResult{result})
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for slow ping, got %d", len(alerts))
	}
}

// TestIncidentEscalationFromWarningToCritical verifies severity escalation.
func TestIncidentEscalationFromWarningToCritical(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	// First: warning
	manager.ProcessAlert("check-domain", "Domain Check", "domain", "warning", "cert expires in 20 days", nil)

	incidents, _ := repo.ListIncidents()
	if incidents[0].Severity != "warning" {
		t.Errorf("initial severity = %q; want 'warning'", incidents[0].Severity)
	}

	// Second: critical
	manager.ProcessAlert("check-domain", "Domain Check", "domain", "critical", "cert expires in 3 days", nil)

	incidents, _ = repo.ListIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident (updated), got %d", len(incidents))
	}
	if incidents[0].Severity != "critical" {
		t.Errorf("escalated severity = %q; want 'critical'", incidents[0].Severity)
	}
}

// TestMultipleCheckTypesGenerateIndependentIncidents verifies each check type
// creates its own incident.
func TestMultipleCheckTypesGenerateIndependentIncidents(t *testing.T) {
	repo := NewMemoryIncidentRepository()
	manager := NewIncidentManager(repo, nil)

	checks := []struct {
		checkID   string
		checkName string
		checkType string
	}{
		{"ssl-1", "SSL Cert", "ssl"},
		{"dns-1", "DNS Lookup", "dns"},
		{"ping-1", "Server Ping", "ping"},
		{"heartbeat-1", "Cron Heartbeat", "heartbeat"},
		{"domain-1", "Domain Expiry", "domain"},
		{"api-1", "API Health", "api"},
	}

	for _, c := range checks {
		err := manager.ProcessAlert(c.checkID, c.checkName, c.checkType, "critical", c.checkType+" failed", nil)
		if err != nil {
			t.Fatalf("ProcessAlert for %s: %v", c.checkType, err)
		}
	}

	incidents, _ := repo.ListIncidents()
	if len(incidents) != 6 {
		t.Fatalf("expected 6 independent incidents, got %d", len(incidents))
	}

	// Verify each has correct type
	typeMap := make(map[string]bool)
	for _, inc := range incidents {
		typeMap[inc.Type] = true
	}
	for _, c := range checks {
		if !typeMap[c.checkType] {
			t.Errorf("missing incident for type %q", c.checkType)
		}
	}
}

// TestAlertCooldownPreventsSpam verifies cooldown works correctly.
func TestAlertCooldownPreventsSpam(t *testing.T) {
	rules := []AlertRule{
		{
			ID:       "rule-cooldown",
			Name:     "Cooldown Test",
			Enabled:  true,
			CheckIDs: []string{"check-cd"},
			Conditions: []AlertCondition{
				{Field: "status", Operator: OperatorEquals, Value: "critical"},
			},
			Severity:        "critical",
			Channels:        []AlertChannel{{Type: "log"}},
			CooldownMinutes: 5, // 5 minute cooldown
		},
	}

	engine := NewAlertRuleEngine(rules, &mockLogger{})

	result := CheckResult{
		ID:      "cd-1",
		CheckID: "check-cd",
		Name:    "CD Check",
		Status:  "critical",
		Healthy: false,
		Metrics: map[string]float64{},
	}

	// First evaluation — should fire
	alerts := engine.Evaluate([]CheckResult{result})
	if len(alerts) != 1 {
		t.Fatalf("first eval: expected 1 alert, got %d", len(alerts))
	}

	// Second evaluation immediately — should be in cooldown
	result.ID = "cd-2"
	alerts = engine.Evaluate([]CheckResult{result})
	if len(alerts) != 0 {
		t.Errorf("second eval: expected 0 alerts (cooldown), got %d", len(alerts))
	}
}
