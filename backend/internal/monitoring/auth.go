package monitoring

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// authMiddleware enforces authentication via JWT Bearer tokens or Basic Auth.
// Public endpoints (login, healthz) are exempt.
// GET requests require authentication but allow any role.
// Mutating requests (POST/PUT/PATCH/DELETE) require "admin" role.
func authMiddleware(cfg AuthConfig, userStore UserStoreBackend, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Public endpoints — no auth required
		if path == "/healthz" || path == "/readyz" ||
			path == "/api/v1/system/status" ||
			path == "/api/v1/auth/login" || strings.HasPrefix(path, "/assets/") ||
			path == "/" || path == "/favicon.ico" ||
			strings.HasPrefix(path, "/api/v1/help/") {
			next.ServeHTTP(w, r)
			return
		}

		// Heartbeat ping endpoints are unauthenticated (cron jobs / external services).
		// Only exempt /api/v1/heartbeats/{token} (with a token segment), NOT /api/v1/heartbeats (the list).
		if isHeartbeatPingPath(path) {
			next.ServeHTTP(w, r)
			return
		}

		// Public status pages — unauthenticated
		if strings.HasPrefix(path, "/status/") {
			next.ServeHTTP(w, r)
			return
		}

		// Serve frontend SPA files without auth
		if !strings.HasPrefix(path, "/api/") && path != "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// If user store is available, use JWT auth
		if userStore != nil {
			claims := ExtractJWTClaims(r)
			if claims == nil {
				// Also try legacy Basic Auth for backward compatibility
				if cfg.Enabled && IsRequestAuthorized(cfg, r) {
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"success":false,"error":{"code":401,"message":"authentication required"}}`))
				return
			}

			// Role-based: ops users can only GET
			if isMutatingMethod(r.Method) && claims.Role != RoleAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"success":false,"error":{"code":403,"message":"admin role required for this action"}}`))
				return
			}

			next.ServeHTTP(w, r)
			return
		}

		// Fallback: legacy Basic Auth
		if cfg.Enabled {
			// Mutating requests always require auth.
			// Sensitive reads (heartbeat list reveals tokens; private dashboard
			// data exposes potentially confidential layouts/results) also require
			// auth even though they are GETs.
			if isMutatingMethod(r.Method) || isSensitiveReadPath(path) {
				if !IsRequestAuthorized(cfg, r) {
					RequestAuth(w)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// isSensitiveReadPath returns true for GET endpoints whose responses may include
// secrets or per-user/private data, and which therefore must require auth even
// in legacy Basic Auth fallback mode (where most GETs are otherwise public).
func isSensitiveReadPath(path string) bool {
	switch {
	case path == "/api/v1/heartbeats":
		return true
	case path == "/api/v1/dashboards" || strings.HasPrefix(path, "/api/v1/dashboards/"):
		return true
	case path == "/api/v1/chat" || strings.HasPrefix(path, "/api/v1/chat/"):
		return true
	case path == "/api/v1/status-pages" || strings.HasPrefix(path, "/api/v1/status-pages/"):
		return true
	case path == "/api/v1/maintenance" || strings.HasPrefix(path, "/api/v1/maintenance/"):
		return true
	}
	return false
}

func isHeartbeatPingPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/heartbeats/") && len(path) > len("/api/v1/heartbeats/")
}

// basicAuthMiddleware returns middleware that enforces basic auth.
// Kept for backward compatibility.
func basicAuthMiddleware(cfg AuthConfig, next http.Handler) http.Handler {
	if !cfg.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if isHeartbeatPingPath(path) || strings.HasPrefix(path, "/status/") {
			next.ServeHTTP(w, r)
			return
		}
		if isMutatingMethod(r.Method) || isSensitiveReadPath(path) {
			if !IsRequestAuthorized(cfg, r) {
				RequestAuth(w)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func IsRequestAuthorized(cfg AuthConfig, r *http.Request) bool {
	if !cfg.Enabled {
		return true
	}

	if claims := ExtractJWTClaims(r); claims != nil {
		return claims.Role == RoleAdmin
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}
	username, password, ok := parseBasicAuth(auth)
	if !ok {
		return false
	}

	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1
	return usernameMatch && passwordMatch
}

// RequestAuth sends a 401 Unauthorized response with WWW-Authenticate header.
func RequestAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="HealthOps"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("Unauthorized\n"))
}

// parseBasicAuth parses the Authorization header and extracts username and password.
// Returns ok=false if the header is malformed or not Basic auth.
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}

	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return
	}

	return creds[0], creds[1], true
}

// ownerFromRequest extracts the authenticated user identity from JWT claims.
// Returns the username from JWT, or empty string if no JWT auth is present
// (e.g. legacy Basic Auth mode where users are not individually identified).
func ownerFromRequest(r *http.Request) string {
	if claims := ExtractJWTClaims(r); claims != nil {
		return claims.Username
	}
	return ""
}
