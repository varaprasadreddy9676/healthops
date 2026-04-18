package monitoring

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuthMiddleware_ValidAuth(t *testing.T) {
	cfg := AuthConfig{
		Enabled:  true,
		Username: "admin",
		Password: "secret",
	}

	middleware := basicAuthMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name       string
		method     string
		authHeader string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "POST with valid auth",
			method:     http.MethodPost,
			authHeader: "Basic " + basicAuth("admin", "secret"),
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "PUT with valid auth",
			method:     http.MethodPut,
			authHeader: "Basic " + basicAuth("admin", "secret"),
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "PATCH with valid auth",
			method:     http.MethodPatch,
			authHeader: "Basic " + basicAuth("admin", "secret"),
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "DELETE with valid auth",
			method:     http.MethodDelete,
			authHeader: "Basic " + basicAuth("admin", "secret"),
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
		{
			name:       "GET without auth bypasses",
			method:     http.MethodGet,
			authHeader: "",
			wantStatus: http.StatusOK,
			wantBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/checks", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if status := w.Code; status != tt.wantStatus {
				t.Errorf("status = %v, want %v", status, tt.wantStatus)
			}
			if body := w.Body.String(); body != tt.wantBody {
				t.Errorf("body = %v, want %v", body, tt.wantBody)
			}
		})
	}
}

func TestBasicAuthMiddleware_InvalidAuth(t *testing.T) {
	cfg := AuthConfig{
		Enabled:  true,
		Username: "admin",
		Password: "secret",
	}

	middleware := basicAuthMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name       string
		method     string
		authHeader string
		wantStatus int
		wantHeader string
	}{
		{
			name:       "POST without auth header",
			method:     http.MethodPost,
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
			wantHeader: `Basic realm="HealthMon"`,
		},
		{
			name:       "POST with wrong username",
			method:     http.MethodPost,
			authHeader: "Basic " + basicAuth("wrong", "secret"),
			wantStatus: http.StatusUnauthorized,
			wantHeader: `Basic realm="HealthMon"`,
		},
		{
			name:       "POST with wrong password",
			method:     http.MethodPost,
			authHeader: "Basic " + basicAuth("admin", "wrong"),
			wantStatus: http.StatusUnauthorized,
			wantHeader: `Basic realm="HealthMon"`,
		},
		{
			name:       "POST with malformed auth",
			method:     http.MethodPost,
			authHeader: "Basic invalidbase64==",
			wantStatus: http.StatusUnauthorized,
			wantHeader: `Basic realm="HealthMon"`,
		},
		{
			name:       "POST with non-Basic auth",
			method:     http.MethodPost,
			authHeader: "Bearer token123",
			wantStatus: http.StatusUnauthorized,
			wantHeader: `Basic realm="HealthMon"`,
		},
		{
			name:       "PUT with wrong credentials",
			method:     http.MethodPut,
			authHeader: "Basic " + basicAuth("user", "pass"),
			wantStatus: http.StatusUnauthorized,
			wantHeader: `Basic realm="HealthMon"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/checks", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if status := w.Code; status != tt.wantStatus {
				t.Errorf("status = %v, want %v", status, tt.wantStatus)
			}
			if header := w.Header().Get("WWW-Authenticate"); header != tt.wantHeader {
				t.Errorf("WWW-Authenticate = %v, want %v", header, tt.wantHeader)
			}
		})
	}
}

func TestBasicAuthMiddleware_DisabledAuth(t *testing.T) {
	cfg := AuthConfig{
		Enabled: false,
	}

	middleware := basicAuthMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "POST without auth when disabled",
			method: http.MethodPost,
		},
		{
			name:   "PUT without auth when disabled",
			method: http.MethodPut,
		},
		{
			name:   "DELETE without auth when disabled",
			method: http.MethodDelete,
		},
		{
			name:   "GET without auth when disabled",
			method: http.MethodGet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/checks", nil)
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if status := w.Code; status != http.StatusOK {
				t.Errorf("status = %v, want %v", status, http.StatusOK)
			}
			if body := w.Body.String(); body != "success" {
				t.Errorf("body = %v, want success", body)
			}
		})
	}
}

func TestParseBasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		auth     string
		wantUser string
		wantPass string
		wantOK   bool
	}{
		{
			name:     "valid basic auth",
			auth:     "Basic " + basicAuth("admin", "secret"),
			wantUser: "admin",
			wantPass: "secret",
			wantOK:   true,
		},
		{
			name:     "empty string",
			auth:     "",
			wantUser: "",
			wantPass: "",
			wantOK:   false,
		},
		{
			name:     "no Basic prefix",
			auth:     "Bearer token",
			wantUser: "",
			wantPass: "",
			wantOK:   false,
		},
		{
			name:     "invalid base64",
			auth:     "Basic !invalid!",
			wantUser: "",
			wantPass: "",
			wantOK:   false,
		},
		{
			name:     "no colon in credentials",
			auth:     "Basic " + basicAuthEncode("nocolon"),
			wantUser: "",
			wantPass: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, pass, ok := parseBasicAuth(tt.auth)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if user != tt.wantUser {
				t.Errorf("user = %v, want %v", user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Errorf("pass = %v, want %v", pass, tt.wantPass)
			}
		})
	}
}

// Helper function to create basic auth header value
func basicAuth(username, password string) string {
	return basicAuthEncode(username + ":" + password)
}

func basicAuthEncode(s string) string {
	// Simple base64 encoding (this is just for testing, use crypto/base64 in production)
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var encoded string
	r := []byte(s)
	for i := 0; i < len(r); i += 3 {
		switch {
		case i+2 < len(r):
			encoded += string(charset[r[i]>>2]) +
				string(charset[(r[i]&0x3)<<4|r[i+1]>>4]) +
				string(charset[(r[i+1]&0xF)<<2|r[i+2]>>6]) +
				string(charset[r[i+2]&0x3F])
		case i+1 < len(r):
			encoded += string(charset[r[i]>>2]) +
				string(charset[(r[i]&0x3)<<4|r[i+1]>>4]) +
				string(charset[(r[i+1]&0xF)<<2]) +
				"="
		default:
			encoded += string(charset[r[i]>>2]) +
				string(charset[(r[i]&0x3)<<4]) +
				"=="
		}
	}
	return encoded
}
