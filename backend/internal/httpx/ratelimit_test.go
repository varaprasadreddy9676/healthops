package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestPerIPLimiter_LoginThrottle verifies that a 5/min per-IP limit (as used
// by the /api/v1/auth/login wrapper) blocks the 6th request from the same IP
// inside the window with a 429 response.
func TestPerIPLimiter_LoginThrottle(t *testing.T) {
	called := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})

	limiter := NewPerIPLimiter(5, time.Minute, inner)

	const clientIP = "203.0.113.7"
	doRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = clientIP + ":4242"
		rr := httptest.NewRecorder()
		limiter.ServeHTTP(rr, req)
		return rr
	}

	for i := 1; i <= 5; i++ {
		rr := doRequest()
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i, rr.Code)
		}
	}

	rr := doRequest()
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("6th request: want 429, got %d", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After header: want 60, got %q", got)
	}
	if called != 5 {
		t.Errorf("inner handler invocations: want 5, got %d", called)
	}
}

// TestPerIPLimiter_PerIPIsolation verifies that one IP being throttled does
// not affect other IPs.
func TestPerIPLimiter_PerIPIsolation(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	limiter := NewPerIPLimiter(2, time.Minute, inner)

	doRequest := func(ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = ip + ":1234"
		rr := httptest.NewRecorder()
		limiter.ServeHTTP(rr, req)
		return rr.Code
	}

	// Exhaust IP A
	if c := doRequest("198.51.100.1"); c != http.StatusOK {
		t.Fatalf("A req1: want 200, got %d", c)
	}
	if c := doRequest("198.51.100.1"); c != http.StatusOK {
		t.Fatalf("A req2: want 200, got %d", c)
	}
	if c := doRequest("198.51.100.1"); c != http.StatusTooManyRequests {
		t.Fatalf("A req3: want 429, got %d", c)
	}

	// IP B still has its full quota
	if c := doRequest("198.51.100.2"); c != http.StatusOK {
		t.Fatalf("B req1: want 200, got %d", c)
	}
}
