package monitoring

import (
	"bytes"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// FuzzValidateCheckID feeds arbitrary strings to the public ID validator. It must
// never panic; it must reject any string containing characters outside [a-z0-9-].
func FuzzValidateCheckID(f *testing.F) {
	f.Add("good-id")
	f.Add("")
	f.Add(strings.Repeat("a", 200))
	f.Add("../etc/passwd")
	f.Add("UPPER")
	f.Add("with space")
	f.Add(string([]byte{0x00, 0x01, 0xff}))
	f.Add("emoji-🔥")

	f.Fuzz(func(t *testing.T, id string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ValidateCheckID panicked on %q: %v", id, r)
			}
		}()
		err := ValidateCheckID(id)
		if err == nil {
			// Passed validation — verify each rune is in the allowed set as a contract check.
			if id == "" || len(id) > maxIDLength {
				t.Fatalf("ValidateCheckID accepted invalid length: %q", id)
			}
			for _, r := range id {
				if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
					t.Fatalf("ValidateCheckID accepted disallowed rune %q in %q", r, id)
				}
			}
		}
	})
}

// FuzzValidateAndDecodeCheck feeds arbitrary bytes through the JSON decoder + validator
// used by the HTTP create-check handler. Must not panic on any input.
func FuzzValidateAndDecodeCheck(f *testing.F) {
	f.Add([]byte(`{"id":"a","name":"a","type":"api"}`))
	f.Add([]byte(``))
	f.Add([]byte(`{`))
	f.Add([]byte(`{"id":"../../etc/passwd","name":"x","type":"api"}`))
	f.Add([]byte(`{"name":"<script>alert(1)</script>","type":"api"}`))
	f.Add([]byte(`{"id":"a","name":"` + strings.Repeat("x", 1024) + `","type":"api"}`))
	// Oversized payload (just over the 1 MiB limit enforced by ValidateAndDecodeCheck).
	big := append([]byte(`{"id":"a","name":"a","type":"api","extra":"`), bytes.Repeat([]byte("x"), 1<<20)...)
	big = append(big, '"', '}')
	f.Add(big)

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ValidateAndDecodeCheck panicked on %d bytes: %v", len(data), r)
			}
		}()
		_, _ = ValidateAndDecodeCheck(bytes.NewReader(data))
	})
}

// FuzzQueryIntRange exercises the query-parameter integer parser. It must never panic
// and must always return a value within [min, max].
func FuzzQueryIntRange(f *testing.F) {
	f.Add("10")
	f.Add("")
	f.Add("-99999999999999999999")
	f.Add("999999999999999999999")
	f.Add("not-a-number")
	f.Add("0x10")
	f.Add(string([]byte{0xff}))

	f.Fuzz(func(t *testing.T, raw string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("QueryIntRange panicked on %q: %v", raw, r)
			}
		}()
		// URL-escape so arbitrary fuzz bytes don't make httptest.NewRequest panic
		// on a malformed request line (that would be a test-harness bug, not a
		// production bug — production receives already-parsed query values).
		req := httptest.NewRequest("GET", "/?limit="+url.QueryEscape(raw), nil)
		v := QueryIntRange(req, "limit", 1, 100, 25)
		if v < 1 || v > 100 {
			t.Fatalf("QueryIntRange returned %d outside [1,100] for %q", v, raw)
		}
	})
}
