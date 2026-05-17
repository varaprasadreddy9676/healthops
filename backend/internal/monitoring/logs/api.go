package logs

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"health-ops/backend/internal/monitoring"
)

// APIHandler handles log intelligence HTTP endpoints.
type APIHandler struct {
	repo        Repository
	categorizer *Categorizer
	logger      *log.Logger
}

// NewAPIHandler creates a log intelligence API handler.
func NewAPIHandler(repo Repository, categorizer *Categorizer, logger *log.Logger) *APIHandler {
	return &APIHandler{
		repo:        repo,
		categorizer: categorizer,
		logger:      logger,
	}
}

// RegisterRoutes registers log intelligence routes on the mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/logs/ingest", h.handleIngest)
	mux.HandleFunc("/api/v1/logs/entries", h.handleEntries)
	mux.HandleFunc("/api/v1/logs/families", h.handleFamilies)
	mux.HandleFunc("/api/v1/logs/families/", h.handleFamilyDetail)
	mux.HandleFunc("/api/v1/logs/stats", h.handleStats)
	mux.HandleFunc("/api/v1/logs/categorize", h.handleCategorize)
}

// POST /api/v1/logs/ingest — ingest log entries
func (h *APIHandler) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req LogIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		monitoring.WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return
	}

	if len(req.Entries) == 0 {
		monitoring.WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("at least one entry is required"))
		return
	}

	if len(req.Entries) > 1000 {
		monitoring.WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("maximum 1000 entries per request"))
		return
	}

	// Convert to internal LogEntry
	entries := make([]LogEntry, 0, len(req.Entries))
	now := time.Now().UTC()
	for i, e := range req.Entries {
		if e.Message == "" {
			monitoring.WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("entry[%d]: message is required", i))
			return
		}
		if e.Source == "" {
			monitoring.WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("entry[%d]: source is required", i))
			return
		}
		if e.Level == "" {
			e.Level = "error"
		}

		ts := now
		if e.Timestamp != "" {
			parsed, err := time.Parse(time.RFC3339, e.Timestamp)
			if err == nil {
				ts = parsed.UTC()
			}
		}

		entry := LogEntry{
			ID:         fmt.Sprintf("log-%d-%d", now.UnixNano(), i),
			Timestamp:  ts,
			Level:      strings.ToLower(e.Level),
			Message:    e.Message,
			Source:     e.Source,
			Server:     e.Server,
			StackTrace: e.StackTrace,
			Tags:       e.Tags,
			Meta:       e.Meta,
		}
		entries = append(entries, entry)
	}

	if err := h.repo.IngestEntries(entries); err != nil {
		h.logger.Printf("logs/api: ingest error: %v", err)
		monitoring.WriteAPIError(w, http.StatusInternalServerError, fmt.Errorf("ingest failed"))
		return
	}

	monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(map[string]interface{}{
		"ingested": len(entries),
		"families": h.familyCountForEntries(entries),
	}))
}

func (h *APIHandler) familyCountForEntries(entries []LogEntry) int {
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.FamilyID] = true
	}
	return len(seen)
}

// GET /api/v1/logs/entries?source=&limit=
func (h *APIHandler) handleEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	source := r.URL.Query().Get("source")
	limit := queryInt(r, "limit", 50)
	if limit > 500 {
		limit = 500
	}

	entries, err := h.repo.RecentEntries(source, limit)
	if err != nil {
		monitoring.WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if entries == nil {
		entries = []LogEntry{}
	}
	monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(entries))
}

// GET /api/v1/logs/families?status=&limit=&category=
func (h *APIHandler) handleFamilies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status := r.URL.Query().Get("status")
	category := r.URL.Query().Get("category")
	limit := queryInt(r, "limit", 50)

	// Apply category before limit so category chips and filtered lists agree.
	fetchLimit := limit
	if category != "" {
		fetchLimit = 0
	}
	families, err := h.repo.ListFamilies(status, fetchLimit)
	if err != nil {
		monitoring.WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}

	if category != "" {
		var filtered []ErrorFamily
		for _, f := range families {
			if f.Category == category {
				filtered = append(filtered, f)
			}
		}
		families = filtered
		if limit > 0 && len(families) > limit {
			families = families[:limit]
		}
	}

	if families == nil {
		families = []ErrorFamily{}
	}
	monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(families))
}

// GET /api/v1/logs/families/{id}
// PATCH /api/v1/logs/families/{id} — update status/category
// POST /api/v1/logs/families/{id}/categorize — AI-categorize one family
func (h *APIHandler) handleFamilyDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/logs/families/")
	categorizeOne := false
	if strings.HasSuffix(id, "/categorize") {
		categorizeOne = true
		id = strings.TrimSuffix(id, "/categorize")
		id = strings.TrimSuffix(id, "/")
	}
	if id == "" {
		monitoring.WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("missing family ID"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		family, err := h.repo.GetFamily(id)
		if err != nil {
			monitoring.WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		familyCopy := *family
		familyCopy.Category = effectiveFamilyCategory(familyCopy)
		// Also include recent entries
		entries, _ := h.repo.EntriesByFamily(id, 20)
		monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(map[string]interface{}{
			"family":  familyCopy,
			"entries": entries,
		}))

	case http.MethodPatch:
		family, err := h.repo.GetFamily(id)
		if err != nil {
			monitoring.WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		var patch struct {
			Status   string `json:"status"`
			Category string `json:"category"`
			Severity string `json:"severity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			monitoring.WriteAPIError(w, http.StatusBadRequest, err)
			return
		}
		if patch.Status != "" {
			family.Status = patch.Status
		}
		if patch.Category != "" {
			family.Category = patch.Category
			family.AILabel = patch.Category
		}
		if patch.Severity != "" {
			family.Severity = patch.Severity
		}
		if err := h.repo.UpdateFamily(*family); err != nil {
			monitoring.WriteAPIError(w, http.StatusInternalServerError, err)
			return
		}
		monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(family))

	case http.MethodPost:
		if !categorizeOne {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if h.categorizer == nil {
			monitoring.WriteAPIError(w, http.StatusServiceUnavailable, fmt.Errorf("AI categorization not available"))
			return
		}
		if err := h.categorizer.CategorizeFamily(r.Context(), id); err != nil {
			monitoring.WriteAPIError(w, http.StatusInternalServerError, err)
			return
		}
		family, err := h.repo.GetFamily(id)
		if err != nil {
			monitoring.WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(family))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// GET /api/v1/logs/stats
func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	stats := h.repo.FamilyStats()
	monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(stats))
}

// POST /api/v1/logs/categorize — trigger AI categorization on unlabeled families
func (h *APIHandler) handleCategorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.categorizer == nil {
		monitoring.WriteAPIError(w, http.StatusServiceUnavailable, fmt.Errorf("AI categorization not available"))
		return
	}

	limit := queryInt(r, "limit", 10)
	count, err := h.categorizer.CategorizeFamilies(r.Context(), limit)
	if err != nil {
		monitoring.WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}

	monitoring.WriteAPIResponse(w, http.StatusOK, monitoring.NewAPIResponse(map[string]interface{}{
		"categorized": count,
	}))
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
