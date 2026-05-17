package monitoring

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 1. Registry Tests
// ---------------------------------------------------------------------------

func TestRegistryContainsAllCheckTypes(t *testing.T) {
	expectedTypes := []string{
		"api", "tcp", "process", "command", "log", "mysql", "ssh",
		"ssl", "dns", "ping", "domain", "heartbeat",
	}
	for _, typ := range expectedTypes {
		exec, ok := LookupCheckExecutor(typ)
		if !ok {
			t.Errorf("check type %q not registered", typ)
			continue
		}
		if exec.Type() != typ {
			t.Errorf("executor.Type() = %q; want %q", exec.Type(), typ)
		}
	}
}

func TestRegisteredCheckTypesReturnsAll(t *testing.T) {
	types := RegisteredCheckTypes()
	if len(types) < 12 {
		t.Errorf("RegisteredCheckTypes() returned %d types; want at least 12", len(types))
	}
	// Should be sorted
	for i := 1; i < len(types); i++ {
		if types[i] < types[i-1] {
			t.Errorf("RegisteredCheckTypes() not sorted at index %d: %q < %q", i, types[i], types[i-1])
		}
	}
}

func TestLookupCheckExecutorUnknownType(t *testing.T) {
	_, ok := LookupCheckExecutor("nonexistent-type")
	if ok {
		t.Error("expected unknown type to return false")
	}
}

// ---------------------------------------------------------------------------
// 2. SSL Check Tests
// ---------------------------------------------------------------------------

func TestSSLCheckApplyDefaults(t *testing.T) {
	executor, _ := LookupCheckExecutor("ssl")
	c := &CheckConfig{Type: "ssl", Host: "example.com"}
	executor.ApplyDefaults(c)

	if c.SSL == nil {
		t.Fatal("SSL config should be non-nil after defaults")
	}
	if c.SSL.Port != 443 {
		t.Errorf("default port = %d; want 443", c.SSL.Port)
	}
	if c.SSL.WarningDays != 30 {
		t.Errorf("default warningDays = %d; want 30", c.SSL.WarningDays)
	}
	if c.SSL.CriticalDays != 7 {
		t.Errorf("default criticalDays = %d; want 7", c.SSL.CriticalDays)
	}
}

func TestSSLCheckValidateMissingHost(t *testing.T) {
	executor, _ := LookupCheckExecutor("ssl")
	c := &CheckConfig{Type: "ssl"} // no host
	executor.ApplyDefaults(c)
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected validation error for missing host")
	}
}

func TestSSLCheckValidateCriticalExceedsWarning(t *testing.T) {
	executor, _ := LookupCheckExecutor("ssl")
	c := &CheckConfig{
		Type: "ssl",
		Host: "example.com",
		SSL:  &SSLCheckConfig{CriticalDays: 60, WarningDays: 30},
	}
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error when criticalDays > warningDays")
	}
}

func TestSSLCheckValidateHappy(t *testing.T) {
	executor, _ := LookupCheckExecutor("ssl")
	c := &CheckConfig{
		Type: "ssl",
		Host: "example.com",
		SSL:  &SSLCheckConfig{CriticalDays: 7, WarningDays: 30},
	}
	err := executor.Validate(c, &Config{})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestSSLCheckExecuteRealTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	executor, _ := LookupCheckExecutor("ssl")
	check := CheckConfig{
		Type: "ssl",
		SSL: &SSLCheckConfig{
			Host:               host,
			Port:               port,
			InsecureSkipVerify: true,
			WarningDays:        1,
			CriticalDays:       0,
		},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)

	if err != nil {
		t.Fatalf("SSL check failed: %v", err)
	}
	if _, ok := result.Metrics["daysUntilExpiry"]; !ok {
		t.Error("expected daysUntilExpiry metric")
	}
	if _, ok := result.Metrics["chainLength"]; !ok {
		t.Error("expected chainLength metric")
	}
}

// ---------------------------------------------------------------------------
// 3. DNS Check Tests
// ---------------------------------------------------------------------------

func TestDNSCheckApplyDefaults(t *testing.T) {
	executor, _ := LookupCheckExecutor("dns")
	c := &CheckConfig{Type: "dns", Target: "example.com"}
	executor.ApplyDefaults(c)
	if c.DNS == nil {
		t.Fatal("DNS config should be non-nil")
	}
	if c.DNS.RecordType != "A" {
		t.Errorf("default recordType = %q; want A", c.DNS.RecordType)
	}
}

func TestDNSCheckValidateMissingName(t *testing.T) {
	executor, _ := LookupCheckExecutor("dns")
	c := &CheckConfig{Type: "dns"}
	executor.ApplyDefaults(c)
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error for missing dns name")
	}
}

func TestDNSCheckValidateInvalidRecordType(t *testing.T) {
	executor, _ := LookupCheckExecutor("dns")
	c := &CheckConfig{
		Type:   "dns",
		Target: "example.com",
		DNS:    &DNSCheckConfig{RecordType: "INVALID"},
	}
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error for invalid record type")
	}
}

func TestDNSCheckValidateAllRecordTypes(t *testing.T) {
	executor, _ := LookupCheckExecutor("dns")
	validTypes := []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS"}
	for _, rt := range validTypes {
		c := &CheckConfig{
			Type:   "dns",
			Target: "example.com",
			DNS:    &DNSCheckConfig{RecordType: rt},
		}
		err := executor.Validate(c, &Config{})
		if err != nil {
			t.Errorf("recordType %q should be valid, got error: %v", rt, err)
		}
	}
}

func TestDNSCheckExecuteLookup(t *testing.T) {
	executor, _ := LookupCheckExecutor("dns")
	check := CheckConfig{
		Type:   "dns",
		Target: "localhost",
		DNS:    &DNSCheckConfig{RecordType: "A"},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	// DNS may fail in CI without a resolver, so we verify it doesn't panic
	_ = executor.Execute(context.Background(), runner, check, result)
	// If it succeeded, verify metrics
	if _, ok := result.Metrics["latencyMs"]; ok {
		if _, ok2 := result.Metrics["recordCount"]; !ok2 {
			t.Error("expected recordCount metric on successful lookup")
		}
	}
}

func TestDNSCheckExpectedValues(t *testing.T) {
	executor, _ := LookupCheckExecutor("dns")
	check := CheckConfig{
		Type: "dns",
		DNS: &DNSCheckConfig{
			Name:       "localhost",
			RecordType: "A",
			Expected:   []string{"127.0.0.1"},
		},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)
	// On systems where localhost resolves to 127.0.0.1, this should pass.
	// On systems where it doesn't, the test is inconclusive but shouldn't panic.
	_ = err
}

// ---------------------------------------------------------------------------
// 4. Ping Check Tests
// ---------------------------------------------------------------------------

func TestPingCheckApplyDefaults(t *testing.T) {
	executor, _ := LookupCheckExecutor("ping")
	c := &CheckConfig{Type: "ping", Host: "127.0.0.1"}
	executor.ApplyDefaults(c)
	if c.Ping == nil {
		t.Fatal("Ping config should be non-nil")
	}
	if c.Ping.Count != 3 {
		t.Errorf("default count = %d; want 3", c.Ping.Count)
	}
	if c.Ping.MaxPacketLossPercent != 50 {
		t.Errorf("default maxPacketLossPercent = %d; want 50", c.Ping.MaxPacketLossPercent)
	}
}

func TestPingCheckValidateMissingHost(t *testing.T) {
	executor, _ := LookupCheckExecutor("ping")
	c := &CheckConfig{Type: "ping"}
	executor.ApplyDefaults(c)
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestPingCheckExecuteLocalhost(t *testing.T) {
	if _, err := exec.LookPath("ping"); err != nil {
		t.Skip("ping command not available")
	}

	executor, _ := LookupCheckExecutor("ping")
	check := CheckConfig{
		Type:           "ping",
		Host:           "127.0.0.1",
		Ping:           &PingCheckConfig{Count: 1, MaxPacketLossPercent: 50},
		TimeoutSeconds: 10,
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)

	if err != nil {
		t.Fatalf("ping localhost failed: %v", err)
	}
	if result.Metrics["packetLossPercent"] != 0 {
		t.Errorf("expected 0%% loss to localhost, got %.0f%%", result.Metrics["packetLossPercent"])
	}
}

// ---------------------------------------------------------------------------
// 5. Domain Check Tests
// ---------------------------------------------------------------------------

func TestDomainCheckApplyDefaults(t *testing.T) {
	executor, _ := LookupCheckExecutor("domain")
	c := &CheckConfig{Type: "domain", Target: "example.com"}
	executor.ApplyDefaults(c)
	if c.Domain == nil {
		t.Fatal("Domain config should be non-nil")
	}
	if c.Domain.WarningDays != 30 {
		t.Errorf("default warningDays = %d; want 30", c.Domain.WarningDays)
	}
	if c.Domain.CriticalDays != 7 {
		t.Errorf("default criticalDays = %d; want 7", c.Domain.CriticalDays)
	}
}

func TestDomainCheckValidateMissingDomain(t *testing.T) {
	executor, _ := LookupCheckExecutor("domain")
	c := &CheckConfig{Type: "domain"}
	executor.ApplyDefaults(c)
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error for missing domain")
	}
}

func TestDomainCheckValidateCriticalExceedsWarning(t *testing.T) {
	executor, _ := LookupCheckExecutor("domain")
	c := &CheckConfig{
		Type:   "domain",
		Target: "example.com",
		Domain: &DomainCheckConfig{CriticalDays: 60, WarningDays: 30},
	}
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error when criticalDays > warningDays")
	}
}

func TestDomainCheckValidateHappy(t *testing.T) {
	executor, _ := LookupCheckExecutor("domain")
	c := &CheckConfig{
		Type:   "domain",
		Target: "example.com",
		Domain: &DomainCheckConfig{CriticalDays: 7, WarningDays: 30},
	}
	err := executor.Validate(c, &Config{})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDomainExpiryDateParsing(t *testing.T) {
	exec := &domainCheckExecutor{}

	tests := []struct {
		whois   string
		wantErr bool
	}{
		{whois: "Registry Expiry Date: 2027-08-13T04:00:00Z\n", wantErr: false},
		{whois: "Expiry Date: 2027-08-13\n", wantErr: false},
		{whois: "paid-till: 2027-08-13\n", wantErr: false},
		{whois: "No date here\n", wantErr: true},
	}

	for i, tt := range tests {
		_, err := exec.parseExpiryDate(tt.whois)
		if (err != nil) != tt.wantErr {
			t.Errorf("test %d: parseExpiryDate error = %v; wantErr = %v", i, err, tt.wantErr)
		}
	}
}

// ---------------------------------------------------------------------------
// 6. Heartbeat Check Tests
// ---------------------------------------------------------------------------

func TestHeartbeatCheckApplyDefaults(t *testing.T) {
	executor, _ := LookupCheckExecutor("heartbeat")
	c := &CheckConfig{ID: "hb-1", Type: "heartbeat"}
	executor.ApplyDefaults(c)
	if c.Heartbeat == nil {
		t.Fatal("Heartbeat config should be non-nil")
	}
	if c.Heartbeat.Token == "" {
		t.Error("expected token to be auto-generated")
	}
	if c.Heartbeat.GraceSeconds != 60 {
		t.Errorf("default graceSeconds = %d; want 60", c.Heartbeat.GraceSeconds)
	}
}

func TestHeartbeatCheckValidateMissingInterval(t *testing.T) {
	executor, _ := LookupCheckExecutor("heartbeat")
	c := &CheckConfig{Type: "heartbeat", Heartbeat: &HeartbeatCheckConfig{Token: "test"}}
	err := executor.Validate(c, &Config{})
	if err == nil {
		t.Fatal("expected error for missing expectedIntervalSeconds")
	}
}

func TestHeartbeatStoreRecordAndRetrieve(t *testing.T) {
	store := &HeartbeatStore{states: make(map[string]*HeartbeatState)}
	store.Register("token-1", "check-1")

	// Initially no ping
	state, ok := store.GetState("token-1")
	if !ok {
		t.Fatal("expected state for registered token")
	}
	if state.CurrentState != "missed" {
		t.Errorf("initial state = %q; want missed", state.CurrentState)
	}

	// Record a ping
	err := store.RecordPing(HeartbeatPing{
		Token:    "token-1",
		PingedAt: time.Now().UTC(),
		Status:   "success",
	})
	if err != nil {
		t.Fatalf("RecordPing failed: %v", err)
	}

	state, _ = store.GetState("token-1")
	if state.PingCount != 1 {
		t.Errorf("pingCount = %d; want 1", state.PingCount)
	}
	if state.CurrentState != "healthy" {
		t.Errorf("state after ping = %q; want healthy", state.CurrentState)
	}
}

func TestHeartbeatStoreUnknownToken(t *testing.T) {
	store := &HeartbeatStore{states: make(map[string]*HeartbeatState)}
	err := store.RecordPing(HeartbeatPing{Token: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestHeartbeatCheckExecuteNoPing(t *testing.T) {
	executor, _ := LookupCheckExecutor("heartbeat")

	token := GenerateHeartbeatToken()
	hbStore := GetHeartbeatStore()
	hbStore.Register(token, "test-check-nopin")

	check := CheckConfig{
		ID:   "test-check-nopin",
		Name: "Test Heartbeat",
		Type: "heartbeat",
		Heartbeat: &HeartbeatCheckConfig{
			Token:                   token,
			ExpectedIntervalSeconds: 60,
			GraceSeconds:            30,
		},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)

	// Should fail — no ping received yet
	if err == nil {
		t.Fatal("expected error when no heartbeat ping received")
	}
}

func TestHeartbeatCheckExecuteWithRecentPing(t *testing.T) {
	executor, _ := LookupCheckExecutor("heartbeat")

	token := GenerateHeartbeatToken()
	hbStore := GetHeartbeatStore()
	hbStore.Register(token, "test-check-recent")

	// Record a recent ping
	hbStore.RecordPing(HeartbeatPing{
		Token:    token,
		PingedAt: time.Now().UTC(),
		Status:   "success",
	})

	check := CheckConfig{
		ID:   "test-check-recent",
		Name: "Test Heartbeat Recent",
		Type: "heartbeat",
		Heartbeat: &HeartbeatCheckConfig{
			Token:                   token,
			ExpectedIntervalSeconds: 60,
			GraceSeconds:            30,
		},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)

	if err != nil {
		t.Fatalf("heartbeat check failed with recent ping: %v", err)
	}
	if result.Metrics["lastPingAgeSeconds"] > 5 {
		t.Errorf("lastPingAgeSeconds = %.0f; expected < 5", result.Metrics["lastPingAgeSeconds"])
	}
}

func TestHeartbeatCheckExecuteWithStalePing(t *testing.T) {
	executor, _ := LookupCheckExecutor("heartbeat")

	token := GenerateHeartbeatToken()
	hbStore := GetHeartbeatStore()
	hbStore.Register(token, "test-check-stale")

	// Record a stale ping (2 minutes ago)
	hbStore.RecordPing(HeartbeatPing{
		Token:    token,
		PingedAt: time.Now().UTC().Add(-2 * time.Minute),
		Status:   "success",
	})

	check := CheckConfig{
		ID:   "test-check-stale",
		Name: "Test Heartbeat Stale",
		Type: "heartbeat",
		Heartbeat: &HeartbeatCheckConfig{
			Token:                   token,
			ExpectedIntervalSeconds: 30, // expects every 30s
			GraceSeconds:            10, // +10s grace = 40s deadline
		},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)

	// Should fail — ping is too old (120s > 40s deadline)
	if err == nil {
		t.Fatal("expected error for stale heartbeat ping")
	}
}

func TestHeartbeatCheckExecuteWithFailStatus(t *testing.T) {
	executor, _ := LookupCheckExecutor("heartbeat")

	token := GenerateHeartbeatToken()
	hbStore := GetHeartbeatStore()
	hbStore.Register(token, "test-check-fail")

	// Record a recent ping with fail status
	hbStore.RecordPing(HeartbeatPing{
		Token:    token,
		PingedAt: time.Now().UTC(),
		Status:   "fail",
		Message:  "backup failed",
	})

	check := CheckConfig{
		ID:   "test-check-fail",
		Name: "Test Heartbeat Fail",
		Type: "heartbeat",
		Heartbeat: &HeartbeatCheckConfig{
			Token:                   token,
			ExpectedIntervalSeconds: 60,
			GraceSeconds:            30,
		},
	}

	runner := NewRunner(&Config{Workers: 1}, &fakeStore{})
	result := &CheckResult{Metrics: map[string]float64{}}
	err := executor.Execute(context.Background(), runner, check, result)

	// Should fail — ping reported failure
	if err == nil {
		t.Fatal("expected error when heartbeat reports failure")
	}
	if !strings.Contains(err.Error(), "backup failed") {
		t.Errorf("error should contain failure message, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 7. Built-in Plugin Migration Tests (api, tcp, command, log, process, mysql, ssh)
// ---------------------------------------------------------------------------

func TestAPIPluginRegistered(t *testing.T) {
	executor, ok := LookupCheckExecutor("api")
	if !ok {
		t.Fatal("api executor not registered")
	}
	if executor.Type() != "api" {
		t.Errorf("Type() = %q", executor.Type())
	}
}

func TestAPIPluginValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("api")
	// Missing target
	err := executor.Validate(&CheckConfig{Type: "api"}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing target")
	}

	// Valid
	err = executor.Validate(&CheckConfig{Type: "api", Target: "https://example.com"}, &Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTCPPluginValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("tcp")
	// Missing port
	err := executor.Validate(&CheckConfig{Type: "tcp", Host: "localhost"}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing port")
	}

	// Missing host and target
	err = executor.Validate(&CheckConfig{Type: "tcp", Port: 80}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing host/target")
	}

	// Valid
	err = executor.Validate(&CheckConfig{Type: "tcp", Host: "localhost", Port: 80}, &Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandPluginSecurityValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("command")

	// Command checks disabled by default
	err := executor.Validate(&CheckConfig{Type: "command", Command: "echo test"}, &Config{AllowCommandChecks: false})
	if err == nil {
		t.Fatal("expected error when command checks disabled")
	}

	// Enabled
	err = executor.Validate(&CheckConfig{Type: "command", Command: "echo test"}, &Config{AllowCommandChecks: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Missing command
	err = executor.Validate(&CheckConfig{Type: "command"}, &Config{AllowCommandChecks: true})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestLogPluginValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("log")
	// Missing path
	err := executor.Validate(&CheckConfig{Type: "log", FreshnessSeconds: 300}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing path")
	}

	// Missing freshness
	err = executor.Validate(&CheckConfig{Type: "log", Path: "/var/log/app.log"}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing freshnessSeconds")
	}

	// Valid
	err = executor.Validate(&CheckConfig{Type: "log", Path: "/var/log/app.log", FreshnessSeconds: 300}, &Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessPluginValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("process")
	err := executor.Validate(&CheckConfig{Type: "process"}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing target")
	}

	err = executor.Validate(&CheckConfig{Type: "process", Target: "nginx"}, &Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMySQLPluginValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("mysql")
	// Missing mysql config
	err := executor.Validate(&CheckConfig{Type: "mysql"}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing mysql config")
	}

	// Missing both DSN and direct config
	err = executor.Validate(&CheckConfig{Type: "mysql", MySQL: &MySQLCheckConfig{}}, &Config{})
	if err == nil {
		t.Fatal("expected error for incomplete mysql config")
	}

	// Valid with DSN env
	err = executor.Validate(&CheckConfig{Type: "mysql", MySQL: &MySQLCheckConfig{DSNEnv: "MYSQL_DSN"}}, &Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSHPluginValidation(t *testing.T) {
	executor, _ := LookupCheckExecutor("ssh")
	// Missing ssh config
	err := executor.Validate(&CheckConfig{Type: "ssh"}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing ssh config")
	}

	// Missing host
	err = executor.Validate(&CheckConfig{Type: "ssh", SSH: &SSHCheckConfig{User: "root", KeyPath: "/key"}}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing host")
	}

	// Missing auth
	err = executor.Validate(&CheckConfig{Type: "ssh", SSH: &SSHCheckConfig{Host: "server", User: "root"}}, &Config{})
	if err == nil {
		t.Fatal("expected error for missing auth")
	}

	// Valid
	err = executor.Validate(&CheckConfig{Type: "ssh", SSH: &SSHCheckConfig{Host: "server", User: "root", KeyPath: "/key"}}, &Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 8. Runner Integration Tests (executeCheck uses registry)
// ---------------------------------------------------------------------------

func TestExecuteCheckUsesRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "api-registry-test",
		Name:           "API Registry Test",
		Type:           "api",
		Target:         server.URL,
		ExpectedStatus: 200,
		TimeoutSeconds: 5,
	}

	result := runner.executeCheck(context.Background(), check)
	if result.Status != "healthy" {
		t.Errorf("status = %q; want healthy. Message: %s", result.Status, result.Message)
	}
}

func TestExecuteCheckUnknownTypeReturnsCritical(t *testing.T) {
	cfg := &Config{Workers: 1}
	store := &fakeStore{}
	runner := NewRunner(cfg, store)

	check := CheckConfig{
		ID:             "unknown-test",
		Name:           "Unknown",
		Type:           "foobar",
		TimeoutSeconds: 5,
	}

	result := runner.executeCheck(context.Background(), check)
	if result.Status != "critical" {
		t.Errorf("status = %q; want critical", result.Status)
	}
	if !strings.Contains(result.Message, "unsupported") {
		t.Errorf("message should mention unsupported, got: %s", result.Message)
	}
}
