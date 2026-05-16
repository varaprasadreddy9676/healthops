package rca

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// APIHandler serves RCA endpoints.
type APIHandler struct {
	analyzer        *Analyzer
	incidentLookup  func(id string) *IncidentRef
	logFamilyLookup func() []ErrorFamilyRef
	logger          *log.Logger
}

// NewAPIHandler creates an RCA API handler.
func NewAPIHandler(
	analyzer *Analyzer,
	incidentLookup func(id string) *IncidentRef,
	logFamilyLookup func() []ErrorFamilyRef,
	logger *log.Logger,
) *APIHandler {
	return &APIHandler{
		analyzer:        analyzer,
		incidentLookup:  incidentLookup,
		logFamilyLookup: logFamilyLookup,
		logger:          logger,
	}
}

// RegisterRoutes implements the RouteRegistrar interface.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/rca/analyze/{incidentId}", h.handleAnalyze)
	mux.HandleFunc("GET /api/v1/rca/reports/{incidentId}", h.handleReportsForIncident)
	mux.HandleFunc("GET /api/v1/rca/report/{id}", h.handleGetReport)
	mux.HandleFunc("GET /api/v1/rca/timeline/{incidentId}", h.handleTimeline)
	mux.HandleFunc("GET /api/v1/rca/reports", h.handleAllReports)
}

func (h *APIHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incidentId")
	if incidentID == "" {
		writeJSON(w, http.StatusBadRequest, envelope{Success: false, Error: &apiErr{Code: 400, Message: "incidentId required"}})
		return
	}

	incident := h.incidentLookup(incidentID)
	if incident == nil {
		writeJSON(w, http.StatusNotFound, envelope{Success: false, Error: &apiErr{Code: 404, Message: "incident not found"}})
		return
	}

	// Get related log families
	var families []ErrorFamilyRef
	if h.logFamilyLookup != nil {
		families = h.logFamilyLookup()
	}

	report, err := h.analyzer.Analyze(r.Context(), *incident, families)
	if err != nil {
		// Still return the report (it has error info)
		if report != nil {
			writeJSON(w, http.StatusOK, envelope{Success: true, Data: report})
			return
		}
		writeJSON(w, http.StatusInternalServerError, envelope{Success: false, Error: &apiErr{Code: 500, Message: err.Error()}})
		return
	}

	writeJSON(w, http.StatusOK, envelope{Success: true, Data: report})
}

func (h *APIHandler) handleReportsForIncident(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incidentId")
	if incidentID == "" {
		writeJSON(w, http.StatusBadRequest, envelope{Success: false, Error: &apiErr{Code: 400, Message: "incidentId required"}})
		return
	}

	reports := h.analyzer.ReportsForIncident(incidentID)
	if reports == nil {
		reports = []RCAReport{}
	}
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: reports})
}

func (h *APIHandler) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, envelope{Success: false, Error: &apiErr{Code: 400, Message: "id required"}})
		return
	}

	report := h.analyzer.GetReport(id)
	if report == nil {
		writeJSON(w, http.StatusNotFound, envelope{Success: false, Error: &apiErr{Code: 404, Message: "report not found"}})
		return
	}
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: report})
}

func (h *APIHandler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incidentId")
	if incidentID == "" {
		writeJSON(w, http.StatusBadRequest, envelope{Success: false, Error: &apiErr{Code: 400, Message: "incidentId required"}})
		return
	}

	incident := h.incidentLookup(incidentID)
	if incident == nil {
		writeJSON(w, http.StatusNotFound, envelope{Success: false, Error: &apiErr{Code: 404, Message: "incident not found"}})
		return
	}

	// Collect context for timeline view
	ctx := h.analyzer.collector.CollectContext(*incident, 15*time.Minute)
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: struct {
		Timeline []TimelineEvent `json:"timeline"`
		Signals  []SignalSeries  `json:"signals"`
	}{
		Timeline: ctx.RecentEvents,
		Signals:  ctx.Signals,
	}})
}

func (h *APIHandler) handleAllReports(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if _, err := json.Number(limitStr).Int64(); err == nil {
			n, _ := json.Number(limitStr).Int64()
			limit = int(n)
		}
	}

	reports := h.analyzer.AllReports(limit)
	if reports == nil {
		reports = []RCAReport{}
	}
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: reports})
}

// --- Response helpers ---

type envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *apiErr     `json:"error,omitempty"`
}

type apiErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
