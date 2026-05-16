package httpx

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter implements a simple per-IP token bucket rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // requests per window
	window   time.Duration // time window
}

type visitor struct {
	tokens    int
	lastReset time.Time
}

func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
	}
	// Clean up stale entries periodically
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists || now.Sub(v.lastReset) >= rl.window {
		rl.visitors[ip] = &visitor{tokens: rl.rate - 1, lastReset: now}
		return true
	}

	if v.tokens > 0 {
		v.tokens--
		return true
	}
	return false
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastReset) > 2*rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit enforces per-IP rate limiting.
// rate is the max requests per window. Returns 429 when exceeded.
func RateLimit(rate int, window time.Duration, next http.Handler) http.Handler {
	limiter := newRateLimiter(rate, window)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isRateLimitExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractIP(r)
		if !limiter.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"success":false,"error":{"code":429,"message":"rate limit exceeded, try again later"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isRateLimitExempt(path string) bool {
	if path == "/healthz" || path == "/readyz" || path == "/" || path == "/favicon.ico" {
		return true
	}
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	if path == "/api/v1/events" || path == "/api/v1/mysql/live" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/servers/") && strings.HasSuffix(path, "/live") {
		return true
	}
	return !strings.HasPrefix(path, "/api/")
}

// extractIP gets the client IP from X-Forwarded-For, X-Real-IP, or RemoteAddr.
func extractIP(r *http.Request) string {
	// Trust X-Real-IP first (set by reverse proxy)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Then X-Forwarded-For (first entry is the client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := len(xff); idx > 0 {
			for i, c := range xff {
				if c == ',' {
					return xff[:i]
				}
				_ = i
			}
			return xff
		}
	}
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
