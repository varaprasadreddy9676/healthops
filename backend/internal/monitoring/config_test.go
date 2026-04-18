package monitoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"checks": [
			{"name": "test", "type": "api", "target": "https://example.com"}
		]
	}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Addr != ":8080" {
		t.Errorf("expected default addr :8080, got %q", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeoutSeconds != 10 {
		t.Errorf("expected default read timeout 10, got %d", cfg.Server.ReadTimeoutSeconds)
	}
	if cfg.Server.WriteTimeoutSeconds != 10 {
		t.Errorf("expected default write timeout 10, got %d", cfg.Server.WriteTimeoutSeconds)
	}
	if cfg.Server.IdleTimeoutSeconds != 60 {
		t.Errorf("expected default idle timeout 60, got %d", cfg.Server.IdleTimeoutSeconds)
	}
	if cfg.RetentionDays != 7 {
		t.Errorf("expected default retention 7, got %d", cfg.RetentionDays)
	}
	if cfg.CheckIntervalSeconds != 60 {
		t.Errorf("expected default interval 60, got %d", cfg.CheckIntervalSeconds)
	}
	if cfg.Workers != 8 {
		t.Errorf("expected default workers 8, got %d", cfg.Workers)
	}

	if len(cfg.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(cfg.Checks))
	}
	check := cfg.Checks[0]
	if check.TimeoutSeconds != 5 {
		t.Errorf("expected default timeout 5, got %d", check.TimeoutSeconds)
	}
	if check.ExpectedStatus != 200 {
		t.Errorf("expected default status 200, got %d", check.ExpectedStatus)
	}
	if check.Enabled == nil {
		t.Errorf("expected enabled to be set")
	} else if !*check.Enabled {
		t.Errorf("expected enabled true, got false")
	}
	if check.Server != "default" {
		t.Errorf("expected default server, got %q", check.Server)
	}
}

func TestLoadConfigRejectsEmptyChecks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{"server": {"addr": ":8080"}}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for empty checks")
	}
	if err.Error() != "config must define at least one check" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsDuplicateCheckIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"checks": [
			{"id": "dup", "name": "First", "type": "api", "target": "https://one.com"},
			{"id": "dup", "name": "Second", "type": "api", "target": "https://two.com"}
		]
	}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

func TestLoadConfigValidatesCheckTypes(t *testing.T) {
	tests := []struct {
		name    string
		check   string
		wantErr string
	}{
		{
			name:    "api missing target",
			check:   `{"name": "x", "type": "api"}`,
			wantErr: "target is required",
		},
		{
			name:    "tcp missing port",
			check:   `{"name": "x", "type": "tcp"}`,
			wantErr: "port is required",
		},
		{
			name:    "process missing target",
			check:   `{"name": "x", "type": "process"}`,
			wantErr: "target is required",
		},
		{
			name:    "command missing command",
			check:   `{"name": "x", "type": "command"}`,
			wantErr: "command is required",
		},
		{
			name:    "log missing path",
			check:   `{"name": "x", "type": "log"}`,
			wantErr: "path is required",
		},
		{
			name:    "unsupported type",
			check:   `{"name": "x", "type": "unknown"}`,
			wantErr: "unsupported type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			raw := []byte(`{"checks": [` + tt.check + `]}`)
			if err := os.WriteFile(path, raw, 0644); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := LoadConfig(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() == "" {
				t.Errorf("expected error containing %q, got empty error", tt.wantErr)
			}
		})
	}
}

func TestConfigValidateLogCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Missing freshnessSeconds
	raw := []byte(`{
		"checks": [
			{"name": "log", "type": "log", "path": "/var/log/test.log"}
		]
	}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for log check missing freshnessSeconds")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"API Check", "api-check"},
		{"  spaces  ", "spaces"},
		{"Special!!!Chars???", "special-chars"},
		{"Multiple---Dashes", "multiple-dashes"},
		{"", "check"},
		{"---", "check"},
		{"123", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := slugify(tt.input); got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildCheckID(t *testing.T) {
	tests := []struct {
		name  string
		check CheckConfig
		want  string
	}{
		{
			name:  "explicit id",
			check: CheckConfig{ID: "my-id", Name: "Ignore Me"},
			want:  "my-id",
		},
		{
			name:  "from name",
			check: CheckConfig{Name: "API Health Check"},
			want:  "api-health-check",
		},
		{
			name:  "from target",
			check: CheckConfig{Target: "https://example.com/health"},
			want:  "https-example-com-health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildCheckID(&tt.check); got != tt.want {
				t.Errorf("buildCheckID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigSerialization(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			Addr:                ":9090",
			ReadTimeoutSeconds:  30,
			WriteTimeoutSeconds: 30,
			IdleTimeoutSeconds:  120,
		},
		RetentionDays:        14,
		CheckIntervalSeconds: 30,
		Workers:              4,
		Checks: []CheckConfig{
			{
				ID:             "test-api",
				Name:           "Test API",
				Type:           "api",
				Target:         "https://test.com/health",
				ExpectedStatus: 200,
				TimeoutSeconds: 10,
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Server.Addr != cfg.Server.Addr {
		t.Errorf("addr mismatch")
	}
	if len(decoded.Checks) != 1 {
		t.Fatalf("check count mismatch")
	}
	if decoded.Checks[0].ID != "test-api" {
		t.Errorf("check id mismatch")
	}
}

func TestCheckConfigIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"nil enabled (default)", nil, true},
		{"explicitly enabled", boolPtr(true), true},
		{"explicitly disabled", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CheckConfig{Enabled: tt.enabled}
			if got := c.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTCPAddress(t *testing.T) {
	tests := []struct {
		name  string
		check CheckConfig
		want  string
	}{
		{
			name:  "from target with colon",
			check: CheckConfig{Target: "example.com:8080"},
			want:  "example.com:8080",
		},
		{
			name:  "from host and port",
			check: CheckConfig{Host: "192.168.1.1", Port: 3000},
			want:  "192.168.1.1:3000",
		},
		{
			name:  "default host with port",
			check: CheckConfig{Port: 9090},
			want:  "127.0.0.1:9090",
		},
		{
			name:  "target host with port field",
			check: CheckConfig{Target: "example.com", Port: 443},
			want:  "127.0.0.1:443", // Target without colon is ignored, defaults to 127.0.0.1 with given port
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTCPAddress(tt.check); got != tt.want {
				t.Errorf("resolveTCPAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPerCheckSchedulingValidation(t *testing.T) {
	tests := []struct {
		name    string
		check   string
		wantErr string
	}{
		{
			name:    "interval below minimum",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "intervalSeconds": 5}`,
			wantErr: "intervalSeconds must be >= 10",
		},
		{
			name:    "interval at minimum accepted",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "intervalSeconds": 10}`,
			wantErr: "",
		},
		{
			name:    "interval above minimum accepted",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "intervalSeconds": 300}`,
			wantErr: "",
		},
		{
			name:    "negative retry count",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "retryCount": -1}`,
			wantErr: "retryCount must be >= 0",
		},
		{
			name:    "zero retry count accepted",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "retryCount": 0}`,
			wantErr: "",
		},
		{
			name:    "positive retry count accepted",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "retryCount": 3, "retryDelaySeconds": 10}`,
			wantErr: "",
		},
		{
			name:    "retry count without delay",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "retryCount": 2, "retryDelaySeconds": 0}`,
			wantErr: "retryDelaySeconds must be >= 1 when retryCount > 0",
		},
		{
			name:    "retry count with valid delay",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "retryCount": 2, "retryDelaySeconds": 5}`,
			wantErr: "",
		},
		{
			name:    "negative cooldown",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "cooldownSeconds": -1}`,
			wantErr: "cooldownSeconds must be >= 0",
		},
		{
			name:    "zero cooldown accepted",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "cooldownSeconds": 0}`,
			wantErr: "",
		},
		{
			name:    "positive cooldown accepted",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "cooldownSeconds": 30}`,
			wantErr: "",
		},
		{
			name:    "all scheduling fields valid",
			check:   `{"name": "x", "type": "api", "target": "https://test.com", "intervalSeconds": 120, "retryCount": 3, "retryDelaySeconds": 10, "cooldownSeconds": 60}`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			raw := []byte(`{"checks": [` + tt.check + `]}`)
			if err := os.WriteFile(path, raw, 0644); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := LoadConfig(path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if err != nil && !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}
