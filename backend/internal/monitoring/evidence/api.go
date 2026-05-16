package evidence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIHandler serves the evidence backbone and AI Incident Brief HTTP endpoints.
type APIHandler struct {
	briefGen     *BriefGenerator
	briefRepo    *BriefRepository
	eventRepo    *IncidentEventRepository
	signalRepo   *SignalEventRepository
}

// NewAPIHandler creates the evidence API handler.
func NewAPIHandler(
	briefGen *BriefGenerator,
	briefRepo *BriefRepository,
	eventRepo *IncidentEventRepository,
	signalRepo *SignalEventRepository,
) *APIHandler {
	return &APIHandler{
		briefGen:   briefGen,
		briefRepo:  briefRepo,
		eventRepo:  eventRepo,
		signalRepo: signalRepo,
	}
}

// RegisterRoutes registers all evidence/brief API routes on the mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/incidents/", h.handleIncidentEvidence)
	mux.HandleFunc("/api/v1/evidence/brief/", h.handleBrief)
}

func (h *APIHandler) handleIncidentEvidence(w http.ResponseWriter, r *http.Request) {
	// Routes:
	// GET /api/v1/incidents/{id}/timeline  — incident timeline events
	// GET /api/v1/incidents/{id}/brief     — latest AI brief
	// POST /api/v1/incidents/{id}/brief    — generate new AI brief

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return // Let the default handler deal with non-evidence routes
	}

	incidentID := parts[0]
	action := parts[1]

	switch action {
	case "timeline":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleTimeline(w, r, incidentID)

	case "brief":
		switch r.Method {
		case http.MethodGet:
			h.handleGetBrief(w, r, incidentID)
		case http.MethodPost:
			h.handleGenerateBrief(w, r, incidentID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}

	default:
		return // Not an evidence route
	}
}

func (h *APIHandler) handleBrief(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/evidence/brief/{incidentId} — generate brief
	incidentID := strings.TrimPrefix(r.URL.Path, "/api/v1/evidence/brief/")
	if incidentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "incident ID required"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleGenerateBrief(w, r, incidentID)
	case http.MethodGet:
		h.handleGetBrief(w, r, incidentID)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *APIHandler) handleTimeline(w http.ResponseWriter, r *http.Request, incidentID string) {
	if h.eventRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "incident event repository not available (requires MongoDB)",
		})
		return
	}

	events, err := h.eventRepo.FindByIncident(r.Context(), incidentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("fetch timeline: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"incidentId": incidentID,
		"events":     events,
		"count":      len(events),
	})
}

func (h *APIHandler) handleGetBrief(w http.ResponseWriter, r *http.Request, incidentID string) {
	if h.briefRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "brief repository not available (requires MongoDB)",
		})
		return
	}

	brief, err := h.briefRepo.GetLatest(r.Context(), incidentID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("no brief found for incident %s", incidentID),
		})
		return
	}

	writeJSON(w, http.StatusOK, brief)
}

func (h *APIHandler) handleGenerateBrief(w http.ResponseWriter, r *http.Request, incidentID string) {
	if h.briefGen == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "brief generator not available",
		})
		return
	}

	brief, err := h.briefGen.GenerateBrief(r.Context(), incidentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("generate brief: %v", err),
		})
		return
	}

	// Persist the brief
	if h.briefRepo != nil {
		if saveErr := h.briefRepo.Save(r.Context(), brief); saveErr != nil {
			// Log but don't fail — the brief was generated successfully
			// just couldn't be persisted
			_ = saveErr
		}
	}

	writeJSON(w, http.StatusOK, brief)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
