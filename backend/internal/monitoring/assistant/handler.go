package assistant

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"health-ops/backend/internal/monitoring"
)

// AIProvider is a function that calls the AI with system + user messages.
type AIProvider func(ctx context.Context, systemMsg, userMsg string) (string, error)

// Handler serves the natural-language ops assistant API.
type Handler struct {
	ctxBuilder *ContextBuilder
	aiCall     AIProvider
	logger     *log.Logger
}

// NewHandler creates the assistant HTTP handler.
func NewHandler(store monitoring.Store, incidentRepo monitoring.IncidentRepository, aiCall AIProvider, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{
		ctxBuilder: NewContextBuilder(store, incidentRepo),
		aiCall:     aiCall,
		logger:     logger,
	}
}

// RegisterRoutes implements monitoring.RouteRegistrar.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/assistant/ask", h.handleAsk)
	mux.HandleFunc("GET /api/v1/assistant/status", h.handleStatus)
}

func (h *Handler) handleAsk(w http.ResponseWriter, r *http.Request) {
	if h.aiCall == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "AI_NOT_CONFIGURED", "message": "No AI provider configured. Add an API key in Settings > AI to enable the assistant."},
		})
		return
	}

	var req AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "INVALID_REQUEST", "message": "Invalid JSON body"},
		})
		return
	}

	if req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "MISSING_QUESTION", "message": "Question field is required"},
		})
		return
	}

	// Limit question length
	if len(req.Question) > 2000 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "QUESTION_TOO_LONG", "message": "Question must be under 2000 characters"},
		})
		return
	}

	// Limit history to prevent context overflow
	if len(req.History) > MaxHistoryMessages {
		req.History = req.History[len(req.History)-MaxHistoryMessages:]
	}

	start := time.Now()

	// Build telemetry context with user-specified lookback
	lookback := time.Duration(req.LookbackMinutes) * time.Minute
	telemetryJSON, refs, err := h.ctxBuilder.Build(lookback)
	if err != nil {
		h.logger.Printf("[assistant] context build error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "CONTEXT_ERROR", "message": "Failed to gather system context"},
		})
		return
	}

	// Build prompt
	systemMsg, userMsg := h.ctxBuilder.BuildPrompt(req.Question, req.History, telemetryJSON)

	// Call AI provider with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	answer, err := h.aiCall(ctx, systemMsg, userMsg)
	if err != nil {
		h.logger.Printf("[assistant] AI call error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "AI_ERROR", "message": "AI provider failed to respond. Try again."},
		})
		return
	}

	duration := time.Since(start).Milliseconds()

	resp := AskResponse{
		Answer:     answer,
		References: refs,
		Duration:   duration,
		Provider:   "default",
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    resp,
	})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	available := h.aiCall != nil
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"available": available,
			"model":     "assistant-v1",
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
