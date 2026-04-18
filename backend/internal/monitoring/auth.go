package monitoring

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// basicAuthMiddleware returns middleware that enforces basic auth.
// GET requests are allowed without auth (read-only).
// POST/PUT/PATCH/DELETE require auth.
// When auth is disabled, all requests pass through.
func basicAuthMiddleware(cfg AuthConfig, next http.Handler) http.Handler {
	if !cfg.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutatingMethod(r.Method) && !isRequestAuthorized(cfg, r) {
			requestAuth(w)
			return
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

func isRequestAuthorized(cfg AuthConfig, r *http.Request) bool {
	if !cfg.Enabled {
		return true
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

// requestAuth sends a 401 Unauthorized response with WWW-Authenticate header.
func requestAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="HealthMon"`)
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
