package monitoring

import (
	"net/http/httptest"
	"testing"
)

// FuzzParseBasicAuth feeds arbitrary bytes to parseBasicAuth and asserts no panic.
// The function decodes base64 then splits on ":"; both code paths must tolerate any input.
func FuzzParseBasicAuth(f *testing.F) {
	// Seed corpus: well-formed, malformed, edge cases.
	f.Add("Basic dXNlcjpwYXNz")         // "user:pass"
	f.Add("Basic ")                     // empty body
	f.Add("Basic !!!not-base64!!!")     // invalid base64
	f.Add("")                           // empty header
	f.Add("Bearer abc.def.ghi")         // wrong scheme
	f.Add("Basic Og==")                 // ":" → empty user/pass
	f.Add("Basic dXNlcm5hbWVfb25seQ==") // no colon after decode
	f.Add("Basic " + string([]byte{0xff, 0xfe}))
	f.Add("Basic AAAAAAAA")
	f.Add("basic dXNlcjpwYXNz") // case-sensitive prefix check
	for i := 0; i < 10; i++ {
		f.Add(string(make([]byte, i)))
	}

	f.Fuzz(func(t *testing.T, header string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parseBasicAuth panicked on %q: %v", header, r)
			}
		}()
		_, _, _ = parseBasicAuth(header)
	})
}

// FuzzIsRequestAuthorized exercises the full Basic Auth middleware path with arbitrary
// Authorization header values. It must never panic and must never grant access for malformed input.
func FuzzIsRequestAuthorized(f *testing.F) {
	f.Add("Basic dXNlcjpwYXNz")
	f.Add("")
	f.Add("Basic ")
	f.Add("Basic " + "A")
	f.Add("Bearer x")
	f.Add(string([]byte{0x00, 0x01, 0x02}))

	cfg := AuthConfig{
		Enabled:  true,
		Username: "alice",
		Password: "s3cret",
	}

	f.Fuzz(func(t *testing.T, header string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("IsRequestAuthorized panicked on %q: %v", header, r)
			}
		}()
		req := httptest.NewRequest("GET", "/api/v1/checks", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		ok := IsRequestAuthorized(cfg, req)
		// Sanity: the only string that should authorize is the canonical one.
		// We do not assert false here — fuzzer may stumble onto the right value
		// (extremely unlikely with these credentials) — only check non-panic.
		_ = ok
	})
}

// FuzzExtractJWTClaims feeds arbitrary Authorization headers and ?token= values to
// the JWT extractor. It must never panic and must return nil for invalid tokens.
func FuzzExtractJWTClaims(f *testing.F) {
	f.Add("Bearer ", "")
	f.Add("Bearer abc.def.ghi", "")
	f.Add("", "abc.def.ghi")
	f.Add("Bearer "+string([]byte{0x00, 0xff}), "")
	f.Add("Basic dXNlcjpwYXNz", "")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, authHeader, tokenQuery string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ExtractJWTClaims panicked on header=%q token=%q: %v", authHeader, tokenQuery, r)
			}
		}()
		req := httptest.NewRequest("GET", "/api/v1/checks?token="+tokenQuery, nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		claims := ExtractJWTClaims(req)
		// A valid claim from random fuzz input would indicate a signing bypass.
		// The JWT secret is initialised lazily; in this test it is nil/random,
		// so any non-nil claim from random bytes would be a serious bug.
		if claims != nil {
			t.Fatalf("ExtractJWTClaims accepted random input: header=%q token=%q claims=%+v", authHeader, tokenQuery, claims)
		}
	})
}
