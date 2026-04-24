package monitoring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// FuzzConfigUnmarshal feeds arbitrary bytes into LoadConfig (file path) and into
// the JSON unmarshal + validate path directly. The contract is: only structured
// errors, never a panic, regardless of input shape.
func FuzzConfigUnmarshal(f *testing.F) {
	// Seed corpus: empty, valid minimal, malformed, deeply nested, unicode, hostile.
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"checks":[]}`))
	f.Add([]byte(`{"checks":[{"id":"a","name":"a","type":"api","target":"http://x"}]}`))
	f.Add([]byte(`{"checks":[{"id":"","name":"","type":""}]}`))
	f.Add([]byte(`{"checkIntervalSeconds":-1}`))
	f.Add([]byte(`{"workers":99999999}`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"checks":[{"id":"a","name":"<script>","type":"api","target":"x"}]}`))
	f.Add([]byte(`{"checks":[{"id":"../../etc","name":"a","type":"api","target":"x"}]}`))
	f.Add([]byte(`{"servers":[{"id":"s","name":"s","host":"h","user":"u"}]}`))
	// Arbitrarily nested object to exercise the JSON decoder.
	deep := []byte(`{"x":`)
	for i := 0; i < 32; i++ {
		deep = append(deep, '{')
		deep = append(deep, 'a', ':')
	}
	f.Add(deep)

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Config unmarshal/validate panicked on %d bytes: %v", len(data), r)
			}
		}()

		// 1. Direct unmarshal + validate (covers the in-process write path used by HTTP handlers).
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			cfg.applyDefaults()
			_ = cfg.validate()
		}

		// 2. LoadConfig path: writes to a temp file, then exercises the full read-parse-validate path.
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Skip()
		}
		_, _ = LoadConfig(path)
	})
}

// FuzzCheckConfigUnmarshal targets the per-check decoder used by API handlers
// (POST /api/v1/checks, PUT /api/v1/checks/{id}). It must never panic.
func FuzzCheckConfigUnmarshal(f *testing.F) {
	f.Add([]byte(`{"id":"a","name":"a","type":"api","target":"http://x"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"type":"command","command":"rm -rf /"}`))
	f.Add([]byte(`{"type":"ssh","ssh":{}}`))
	f.Add([]byte(`{"type":"mysql","mysql":{}}`))
	f.Add([]byte(`{"timeoutSeconds":-1}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("CheckConfig unmarshal panicked on %d bytes: %v", len(data), r)
			}
		}()
		var cc CheckConfig
		if err := json.Unmarshal(data, &cc); err != nil {
			return
		}
		cc.applyDefaults()
		// Validate against an empty parent config so command checks are rejected.
		_ = cc.validate(&Config{})
	})
}
