package monitoring

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// HeartbeatAPIHandler handles heartbeat ping endpoints.
type HeartbeatAPIHandler struct {
	store *HeartbeatStore
}

// NewHeartbeatAPIHandler creates a heartbeat API handler.
func NewHeartbeatAPIHandler() *HeartbeatAPIHandler {
	return &HeartbeatAPIHandler{store: GetHeartbeatStore()}
}

// RegisterRoutes registers heartbeat routes.
// Ping endpoints are unauthenticated (cron jobs / external services).
// The list endpoint should be behind auth (handled by the service router).
func (h *HeartbeatAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/heartbeats/{token}", h.handlePing)
	mux.HandleFunc("POST /api/v1/heartbeats/{token}", h.handlePing)
	mux.HandleFunc("HEAD /api/v1/heartbeats/{token}", h.handlePing)
	mux.HandleFunc("GET /api/v1/heartbeats", h.handleList)
}

func (h *HeartbeatAPIHandler) handlePing(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, http.StatusBadRequest)
		return
	}

	ping := HeartbeatPing{
		Token:     token,
		PingedAt:  time.Now().UTC(),
		IPAddress: extractIP(r),
		Status:    "success",
	}

	if r.Method == http.MethodPost && r.Body != nil {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		if len(body) > 0 {
			var payload struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &payload); err == nil {
				if payload.Status != "" {
					ping.Status = payload.Status
				}
				ping.Message = payload.Message
			}
		}
	}

	if s := r.URL.Query().Get("status"); s != "" {
		ping.Status = s
	}
	if m := r.URL.Query().Get("msg"); m != "" {
		ping.Message = m
	}

	if err := h.store.RecordPing(ping); err != nil {
		http.Error(w, `{"error":"unknown token"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"pingedAt": ping.PingedAt,
	})
}

func (h *HeartbeatAPIHandler) handleList(w http.ResponseWriter, r *http.Request) {
	states := h.store.AllStates()

	// Mask tokens: show only last 6 characters to prevent leaking secrets
	type safeState struct {
		Token        string         `json:"token"`
		CheckID      string         `json:"checkId"`
		LastPing     *HeartbeatPing `json:"lastPing,omitempty"`
		PingCount    int64          `json:"pingCount"`
		MissedCount  int64          `json:"missedCount"`
		CurrentState string         `json:"currentState"`
	}
	safe := make([]safeState, len(states))
	for i, s := range states {
		masked := maskToken(s.Token)
		var lastPing *HeartbeatPing
		if s.LastPing != nil {
			// Copy and mask the embedded ping's token to avoid leaking it.
			lp := *s.LastPing
			lp.Token = maskToken(lp.Token)
			lastPing = &lp
		}
		safe[i] = safeState{
			Token:        masked,
			CheckID:      s.CheckID,
			LastPing:     lastPing,
			PingCount:    s.PingCount,
			MissedCount:  s.MissedCount,
			CurrentState: s.CurrentState,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"heartbeats": safe,
		"total":      len(safe),
	})
}

// maskToken returns a string with all but the last 6 characters replaced by '*'.
// Strings of 6 chars or fewer are returned unchanged (they are not sensitive enough
// to mask away entirely, but they should never have been issued as tokens anyway).
func maskToken(token string) string {
	if len(token) <= 6 {
		return token
	}
	return strings.Repeat("*", len(token)-6) + token[len(token)-6:]
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	return host
}
