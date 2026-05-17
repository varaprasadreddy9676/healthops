package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"health-ops/backend/internal/monitoring"
)

// AIProvider calls the configured AI model.
type AIProvider func(ctx context.Context, systemMsg, userMsg string) (string, error)

// Handler serves the recommendations API.
type Handler struct {
	engine      *Engine
	aiCall      AIProvider
	logger      *log.Logger
	mu          sync.RWMutex
	cached      []Recommendation
	cachedAt    time.Time
	cacheMaxAge time.Duration
	dismissed   map[string]time.Time // id → dismissed at
}

// NewHandler creates a new recommendations handler.
func NewHandler(store monitoring.Store, incidentRepo monitoring.IncidentRepository, aiCall AIProvider, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{
		engine:      NewEngine(store, incidentRepo),
		aiCall:      aiCall,
		logger:      logger,
		cacheMaxAge: 5 * time.Minute,
		dismissed:   make(map[string]time.Time),
	}
}

// RegisterRoutes implements RouteRegistrar.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/recommendations", h.handleList)
	mux.HandleFunc("POST /api/v1/recommendations/generate", h.handleGenerate)
	mux.HandleFunc("POST /api/v1/recommendations/{id}/dismiss", h.handleDismiss)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	recs := h.getCachedOrGenerate()

	// Filter by category if requested
	category := r.URL.Query().Get("category")
	if category != "" {
		var filtered []Recommendation
		for _, rec := range recs {
			if string(rec.Category) == category {
				filtered = append(filtered, rec)
			}
		}
		recs = filtered
	}

	// Filter out dismissed unless ?include_dismissed=true
	if r.URL.Query().Get("include_dismissed") != "true" {
		var active []Recommendation
		for _, rec := range recs {
			if !rec.Dismissed {
				active = append(active, rec)
			}
		}
		recs = active
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"recommendations": recs,
			"total":           len(recs),
			"generatedAt":     h.cachedAt,
		},
	})
}

func (h *Handler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	recs := h.engine.Generate()
	h.applyDismissals(recs)

	// Optionally enrich with AI explanations
	aiEnriched := false
	if req.UseAI && h.aiCall != nil && len(recs) > 0 {
		enriched := h.enrichWithAI(r.Context(), recs)
		if enriched {
			aiEnriched = true
		}
	}

	h.mu.Lock()
	h.cached = recs
	h.cachedAt = time.Now()
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": GenerateResponse{
			Recommendations: recs,
			GeneratedAt:     time.Now(),
			AIEnriched:      aiEnriched,
		},
	})
}

func (h *Handler) handleDismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   map[string]string{"code": "INVALID_ID", "message": "recommendation ID required"},
		})
		return
	}

	var req DismissRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	h.mu.Lock()
	h.dismissed[id] = time.Now()
	// Update cached list
	for i := range h.cached {
		if h.cached[i].ID == id {
			h.cached[i].Dismissed = true
			now := time.Now()
			h.cached[i].DismissedAt = &now
			break
		}
	}
	h.mu.Unlock()

	h.logger.Printf("[recommendations] dismissed %s (reason: %s)", id, req.Reason)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]string{"id": id, "status": "dismissed"},
	})
}

// getCachedOrGenerate returns cached recommendations or generates fresh ones.
func (h *Handler) getCachedOrGenerate() []Recommendation {
	h.mu.RLock()
	if h.cached != nil && time.Since(h.cachedAt) < h.cacheMaxAge {
		result := make([]Recommendation, len(h.cached))
		copy(result, h.cached)
		h.mu.RUnlock()
		return result
	}
	h.mu.RUnlock()

	recs := h.engine.Generate()
	h.applyDismissals(recs)

	h.mu.Lock()
	h.cached = recs
	h.cachedAt = time.Now()
	h.mu.Unlock()

	return recs
}

func (h *Handler) applyDismissals(recs []Recommendation) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for i := range recs {
		if dismissedAt, ok := h.dismissed[recs[i].ID]; ok {
			recs[i].Dismissed = true
			recs[i].DismissedAt = &dismissedAt
		}
	}
}

// enrichWithAI asks AI to enhance recommendation descriptions.
func (h *Handler) enrichWithAI(ctx context.Context, recs []Recommendation) bool {
	if len(recs) == 0 {
		return false
	}

	// Build a summary for AI
	var sb strings.Builder
	sb.WriteString("Given these infrastructure monitoring recommendations, provide a brief expert analysis for each:\n\n")
	for i, rec := range recs {
		if i >= 5 { // Limit AI context
			break
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s: %s\n", i+1, rec.Category, rec.Title, rec.Reason))
	}

	systemMsg := "You are an infrastructure reliability expert. For each recommendation, provide a 1-sentence actionable insight. Respond as a numbered list matching the input."
	userMsg := sb.String()

	aiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := h.aiCall(aiCtx, systemMsg, userMsg)
	if err != nil {
		h.logger.Printf("[recommendations] AI enrichment failed: %v", err)
		return false
	}

	// Parse AI response and append to descriptions
	lines := strings.Split(response, "\n")
	lineIdx := 0
	for i := range recs {
		if i >= 5 {
			break
		}
		for lineIdx < len(lines) {
			line := strings.TrimSpace(lines[lineIdx])
			lineIdx++
			if line != "" {
				recs[i].Description += "\n\n**AI Insight:** " + strings.TrimLeft(line, "0123456789.) ")
				break
			}
		}
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
