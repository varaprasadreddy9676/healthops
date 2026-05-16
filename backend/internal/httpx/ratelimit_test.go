package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitSkipsFrontendAssetsAndStreams(t *testing.T) {
	handler := RateLimit(1, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/", "/assets/app.js", "/servers/linux-1", "/api/v1/events", "/api/v1/mysql/live", "/api/v1/servers/linux-1/live"} {
		t.Run(path, func(t *testing.T) {
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("request %d got status %d, want 200", i+1, rec.Code)
				}
			}
		})
	}
}

func TestRateLimitAppliesToAPI(t *testing.T) {
	handler := RateLimit(1, time.Minute, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/checks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request got status %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request got status %d, want 429", rec.Code)
	}
}
