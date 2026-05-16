package evidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// --- Response envelope (matches monitoring.APIResponse) ---

type envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *apiErr     `json:"error,omitempty"`
}

type apiErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIHandler serves the evidence backbone and AI Incident Brief HTTP endpoints.
type APIHandler struct {
	briefGen   *BriefGenerator
	briefRepo  *BriefRepository
	eventRepo  *IncidentEventRepository
	signalRepo *SignalEventRepository
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
	mux.HandleFunc("/api/v1/evidence/incidents/", h.handleIncidentEvidence)
	mux.HandleFunc("/api/v1/evidence/brief/", h.handleBrief)
}

func (h *APIHandler) handleIncidentEvidence(w http.ResponseWriter, r *http.Request) {
	// Routes:
	// GET /api/v1/evidence/incidents/{id}/timeline  — incident timeline events
	// GET /api/v1/evidence/incidents/{id}/brief     — latest AI brief
	// POST /api/v1/evidence/incidents/{id}/brief    — generate new AI brief

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/evidence/incidents/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return // Let the default handler deal with non-evidence routes
	}

	incidentID := parts[0]
	action := parts[1]

	switch action {
	case "timeline":
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
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
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	default:
		return // Not an evidence route
	}
}

func (h *APIHandler) handleBrief(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/evidence/brief/{incidentId} — generate brief
	incidentID := strings.TrimPrefix(r.URL.Path, "/api/v1/evidence/brief/")
	if incidentID == "" {
		writeErr(w, http.StatusBadRequest, "incident ID required")
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleGenerateBrief(w, r, incidentID)
	case http.MethodGet:
		h.handleGetBrief(w, r, incidentID)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *APIHandler) handleTimeline(w http.ResponseWriter, r *http.Request, incidentID string) {
	if h.eventRepo == nil {
		writeErr(w, http.StatusServiceUnavailable, "incident event repository not available (requires MongoDB)")
		return
	}

	events, err := h.eventRepo.FindByIncident(r.Context(), incidentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("fetch timeline: %v", err))
		return
	}

	writeOK(w, http.StatusOK, map[string]interface{}{
		"incidentId": incidentID,
		"events":     events,
		"count":      len(events),
	})
}

func (h *APIHandler) handleGetBrief(w http.ResponseWriter, r *http.Request, incidentID string) {
	if h.briefRepo == nil {
		writeErr(w, http.StatusServiceUnavailable, "brief repository not available (requires MongoDB)")
		return
	}

	brief, err := h.briefRepo.GetLatest(r.Context(), incidentID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || strings.Contains(strings.ToLower(err.Error()), "no documents") {
			writeOK(w, http.StatusOK, nil)
			return
		}
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("fetch brief: %v", err))
		return
	}

	writeOK(w, http.StatusOK, brief)
}

func (h *APIHandler) handleGenerateBrief(w http.ResponseWriter, r *http.Request, incidentID string) {
	if h.briefGen == nil {
		writeErr(w, http.StatusServiceUnavailable, "brief generator not available")
		return
	}

	brief, err := h.briefGen.GenerateBrief(r.Context(), incidentID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("generate brief: %v", err))
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

	writeOK(w, http.StatusOK, brief)
}

func writeOK(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(envelope{Success: true, Data: data})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(envelope{Success: false, Error: &apiErr{Code: status, Message: msg}})
}
