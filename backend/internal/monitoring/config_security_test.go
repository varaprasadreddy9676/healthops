package monitoring

import (
	"encoding/json"
	"testing"
)

// TestCommandChecksDisabledByDefault verifies that command checks are rejected
// when allowCommandChecks is not explicitly set to true.
func TestCommandChecksDisabledByDefault(t *testing.T) {
	configJSON := `{
		"server": {"addr": ":8080"},
		"retentionDays": 7,
		"checkIntervalSeconds": 60,
		"workers": 8,
		"checks": [
			{
				"id": "test-command",
				"name": "Test Command",
				"type": "command",
				"command": "echo hello"
			}
		]
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	cfg.applyDefaults()

	// Should fail validation because command checks are disabled by default
	err := cfg.validate()
	if err == nil {
		t.Error("Expected validation error for command check when allowCommandChecks is false")
	}

	// Verify the error message contains security guidance
	if err != nil && !containsString(err.Error(), "allowCommandChecks=true") {
		t.Errorf("Expected error message to mention allowCommandChecks=true, got: %v", err)
	}
}

// TestCommandChecksEnabledWhenExplicitlyAllowed verifies that command checks
// are accepted when allowCommandChecks is explicitly set to true.
func TestCommandChecksEnabledWhenExplicitlyAllowed(t *testing.T) {
	configJSON := `{
		"server": {"addr": ":8080"},
		"retentionDays": 7,
		"checkIntervalSeconds": 60,
		"workers": 8,
		"allowCommandChecks": true,
		"checks": [
			{
				"id": "test-command",
				"name": "Test Command",
				"type": "command",
				"command": "echo hello"
			}
		]
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	cfg.applyDefaults()

	// Should pass validation when command checks are explicitly allowed
	err := cfg.validate()
	if err != nil {
		t.Errorf("Expected validation to pass when allowCommandChecks=true, got: %v", err)
	}
}

// TestCommandCheckMissingCommandField verifies that command checks
// still require the command field even when enabled.
func TestCommandCheckMissingCommandField(t *testing.T) {
	configJSON := `{
		"server": {"addr": ":8080"},
		"retentionDays": 7,
		"checkIntervalSeconds": 60,
		"workers": 8,
		"allowCommandChecks": true,
		"checks": [
			{
				"id": "test-command",
				"name": "Test Command",
				"type": "command"
			}
		]
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	cfg.applyDefaults()

	err := cfg.validate()
	if err == nil {
		t.Error("Expected validation error for command check with missing command field")
	}

	if err != nil && !containsString(err.Error(), "command is required") {
		t.Errorf("Expected error message to mention command is required, got: %v", err)
	}
}

// TestOtherCheckTypesWorkWithCommandChecksDisabled verifies that
// other check types are not affected by the allowCommandChecks setting.
func TestOtherCheckTypesWorkWithCommandChecksDisabled(t *testing.T) {
	configJSON := `{
		"server": {"addr": ":8080"},
		"retentionDays": 7,
		"checkIntervalSeconds": 60,
		"workers": 8,
		"allowCommandChecks": false,
		"checks": [
			{
				"id": "test-api",
				"name": "Test API",
				"type": "api",
				"target": "http://example.com/health"
			},
			{
				"id": "test-tcp",
				"name": "Test TCP",
				"type": "tcp",
				"port": 8080,
				"host": "localhost"
			},
			{
				"id": "test-process",
				"name": "Test Process",
				"type": "process",
				"target": "nginx"
			},
			{
				"id": "test-log",
				"name": "Test Log",
				"type": "log",
				"path": "/var/log/test.log",
				"freshnessSeconds": 300
			}
		]
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	cfg.applyDefaults()

	// Should pass validation for all non-command check types
	err := cfg.validate()
	if err != nil {
		t.Errorf("Expected validation to pass for non-command checks, got: %v", err)
	}

	// Verify we have all checks
	if len(cfg.Checks) != 4 {
		t.Errorf("Expected 4 checks, got %d", len(cfg.Checks))
	}
}

// TestConfigDefaultAllowCommandChecksIsFalse verifies the default value.
func TestConfigDefaultAllowCommandChecksIsFalse(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Addr: ":8080",
		},
		Checks: []CheckConfig{
			{
				ID:      "test-api",
				Name:    "Test API",
				Type:    "api",
				Target:  "http://example.com/health",
				Enabled: boolPtr(true),
			},
		},
	}

	cfg.applyDefaults()

	if cfg.AllowCommandChecks {
		t.Error("Expected AllowCommandChecks to default to false")
	}
}

// TestConfigWithSecurityDocumentation validates that the security documentation
// in error messages is clear and helpful.
func TestConfigWithSecurityDocumentation(t *testing.T) {
	configJSON := `{
		"server": {"addr": ":8080"},
		"retentionDays": 7,
		"checkIntervalSeconds": 60,
		"workers": 8,
		"checks": [
			{
				"id": "malicious",
				"name": "Potential Command Injection",
				"type": "command",
				"command": "rm -rf /; echo bad"
			}
		]
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	cfg.applyDefaults()

	err := cfg.validate()
	if err == nil {
		t.Error("Expected validation to block command check by default")
	}

	errorMsg := err.Error()

	// Verify error message contains key security information
	expectedStrings := []string{
		"command checks are disabled",
		"allowCommandChecks=true",
		"use with caution",
		"security",
	}

	for _, expected := range expectedStrings {
		if !containsString(errorMsg, expected) {
			t.Errorf("Expected error message to contain %q, got: %v", expected, errorMsg)
		}
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
