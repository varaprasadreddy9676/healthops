package automation

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"health-ops/backend/internal/monitoring"
)

// Handler serves the automation API.
type Handler struct {
	engine *Engine
}

// NewHandler creates a new automation handler.
func NewHandler(store monitoring.Store, incidentRepo monitoring.IncidentRepository, aiCall AIProvider, logger *log.Logger) *Handler {
	return &Handler{
		engine: NewEngine(store, incidentRepo, aiCall, logger),
	}
}

// RegisterRoutes implements RouteRegistrar.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/automation/actions", h.handleListActions)
	mux.HandleFunc("GET /api/v1/automation/actions/{id}", h.handleGetAction)
	mux.HandleFunc("POST /api/v1/automation/suggest", h.handleSuggest)
	mux.HandleFunc("POST /api/v1/automation/actions/{id}/approve", h.handleApprove)
	mux.HandleFunc("POST /api/v1/automation/actions/{id}/reject", h.handleReject)
	mux.HandleFunc("GET /api/v1/automation/audit", h.handleAuditLog)
	mux.HandleFunc("GET /api/v1/automation/status", h.handleStatus)
}

func (h *Handler) handleListActions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	actions := h.engine.ListActions(status)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"actions": actions,
			"total":   len(actions),
		},
	})
}

func (h *Handler) handleGetAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	action, found := h.engine.GetAction(id)
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"success": false,
			"error":   map[string]string{"code": "NOT_FOUND", "message": "action not found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    action,
	})
}

func (h *Handler) handleSuggest(w http.ResponseWriter, r *http.Request) {
	var req SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   map[string]string{"code": "INVALID_BODY", "message": "invalid request body"},
		})
		return
	}

	actions, err := h.engine.Suggest(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   map[string]string{"code": "SUGGEST_FAILED", "message": err.Error()},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"actions":     actions,
			"generatedAt": time.Now(),
		},
	})
}

func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req ApproveRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Actor == "" {
		req.Actor = "unknown"
	}

	if err := h.engine.Approve(id, req.Actor); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "action "+id+" not found" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{
			"success": false,
			"error":   map[string]string{"code": "APPROVE_FAILED", "message": err.Error()},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]string{"id": id, "status": "approved"},
	})
}

func (h *Handler) handleReject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req RejectRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Actor == "" {
		req.Actor = "unknown"
	}

	if err := h.engine.Reject(id, req.Actor, req.Reason); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "action "+id+" not found" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{
			"success": false,
			"error":   map[string]string{"code": "REJECT_FAILED", "message": err.Error()},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]string{"id": id, "status": "rejected"},
	})
}

func (h *Handler) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	entries := h.engine.AuditLog()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"entries": entries,
			"total":   len(entries),
		},
	})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	available := h.engine.aiCall != nil
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"enabled":     available,
			"aiAvailable": available,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
