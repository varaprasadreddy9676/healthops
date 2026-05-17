package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CustomDashboardAPIHandler handles CRUD for custom dashboards.
type CustomDashboardAPIHandler struct {
	store        CustomDashboardRepository
	checkStore   Store
	incidentRepo IncidentRepository
}

// NewCustomDashboardAPIHandler creates the handler.
func NewCustomDashboardAPIHandler(store CustomDashboardRepository, checkStore Store, incidentRepo IncidentRepository) *CustomDashboardAPIHandler {
	return &CustomDashboardAPIHandler{
		store:        store,
		checkStore:   checkStore,
		incidentRepo: incidentRepo,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (h *CustomDashboardAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/dashboards", h.handleDashboards)
	mux.HandleFunc("/api/v1/dashboards/", h.handleDashboardByID)
}

func (h *CustomDashboardAPIHandler) handleDashboards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listDashboards(w, r)
	case http.MethodPost:
		h.createDashboard(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CustomDashboardAPIHandler) handleDashboardByID(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL: /api/v1/dashboards/{id} or /api/v1/dashboards/{id}/data or /api/v1/dashboards/{id}/duplicate
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/dashboards/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if id == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("dashboard ID required"))
		return
	}

	// Sub-resource routing
	if len(parts) > 1 {
		switch parts[1] {
		case "data":
			h.getDashboardData(w, r, id)
			return
		case "duplicate":
			h.duplicateDashboard(w, r, id)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.getDashboard(w, r, id)
	case http.MethodPut:
		h.updateDashboard(w, r, id)
	case http.MethodDelete:
		h.deleteDashboard(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CustomDashboardAPIHandler) listDashboards(w http.ResponseWriter, r *http.Request) {
	// Owner is derived from JWT claims, not from query params
	owner := ownerFromRequest(r)
	dashboards := h.store.List(owner)
	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(dashboards))
}

func (h *CustomDashboardAPIHandler) createDashboard(w http.ResponseWriter, r *http.Request) {
	var req CustomDashboard
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	if req.Name == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	// Owner is derived from JWT claims
	req.Owner = ownerFromRequest(r)

	dashboard, err := h.store.Create(req)
	if err != nil {
		WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}
	WriteAPIResponse(w, http.StatusCreated, NewAPIResponse(dashboard))
}

func (h *CustomDashboardAPIHandler) getDashboard(w http.ResponseWriter, r *http.Request, id string) {
	d, err := h.store.Get(id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, err)
		return
	}
	if !canViewDashboard(d, r) {
		WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
		return
	}
	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(d))
}

func (h *CustomDashboardAPIHandler) updateDashboard(w http.ResponseWriter, r *http.Request, id string) {
	existing, err := h.store.Get(id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, err)
		return
	}
	if !canModifyDashboard(existing, r) {
		WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
		return
	}

	var update CustomDashboard
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&update); err != nil {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	// Prevent ownership transfer via update body.
	update.Owner = existing.Owner

	d, err := h.store.Update(id, update)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}
	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(d))
}

func (h *CustomDashboardAPIHandler) deleteDashboard(w http.ResponseWriter, r *http.Request, id string) {
	existing, err := h.store.Get(id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, err)
		return
	}
	if !canModifyDashboard(existing, r) {
		WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
		return
	}
	if err := h.store.Delete(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}
	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(map[string]string{"deleted": id}))
}

func (h *CustomDashboardAPIHandler) duplicateDashboard(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	existing, err := h.store.Get(id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, err)
		return
	}
	// Duplication is a read of the source + write of a new dashboard;
	// require view permission on the source.
	if !canViewDashboard(existing, r) {
		WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if req.Name == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("name is required for duplicate"))
		return
	}

	d, err := h.store.Duplicate(id, req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		WriteAPIError(w, http.StatusInternalServerError, err)
		return
	}
	// Reassign ownership of the duplicate to the caller and persist.
	if owner := ownerFromRequest(r); owner != "" && d != nil {
		copy := *d
		copy.Owner = owner
		if updated, uerr := h.store.Update(d.ID, copy); uerr == nil {
			d = updated
		}
	}
	WriteAPIResponse(w, http.StatusCreated, NewAPIResponse(d))
}

// canViewDashboard returns true if the caller may read the dashboard.
// Public/shared dashboards are visible to any authenticated user; private
// dashboards are visible only to their owner. An empty owner on the dashboard
// is treated as legacy data and allowed (preserves pre-ownership behavior).
func canViewDashboard(d *CustomDashboard, r *http.Request) bool {
	if d == nil {
		return false
	}
	if d.Visibility != "private" {
		return true
	}
	if d.Owner == "" {
		return true
	}
	owner := ownerFromRequest(r)
	return owner == "" || owner == d.Owner
}

// canModifyDashboard returns true if the caller may update or delete the
// dashboard. Only the owner may modify; legacy dashboards without an owner
// remain editable for migration purposes.
func canModifyDashboard(d *CustomDashboard, r *http.Request) bool {
	if d == nil {
		return false
	}
	if d.Owner == "" {
		return true
	}
	owner := ownerFromRequest(r)
	return owner == "" || owner == d.Owner
}

// getDashboardData returns live data for a custom dashboard with filtered checks/results.
func (h *CustomDashboardAPIHandler) getDashboardData(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dashboard, err := h.store.Get(id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, err)
		return
	}
	if !canViewDashboard(dashboard, r) {
		WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
		return
	}

	state := h.checkStore.Snapshot()

	// Filter checks based on dashboard criteria
	filteredChecks := h.filterChecks(state.Checks, dashboard)

	// Build results map for filtered checks
	checkIDSet := make(map[string]bool, len(filteredChecks))
	for _, c := range filteredChecks {
		checkIDSet[c.ID] = true
	}

	resultsMap := make(map[string][]CheckResult)
	for _, r := range state.Results {
		if checkIDSet[r.CheckID] {
			resultsMap[r.CheckID] = append(resultsMap[r.CheckID], r)
		}
	}

	// Build summary for filtered checks
	summary := buildSummary(filteredChecks, state.Results, &state.LastRunAt)

	// Get recent incidents for filtered checks
	var incidents []Incident
	if h.incidentRepo != nil {
		allIncidents, _ := h.incidentRepo.ListIncidents()
		for _, inc := range allIncidents {
			if checkIDSet[inc.CheckID] {
				incidents = append(incidents, inc)
			}
		}
	}

	data := DashboardData{
		Dashboard:   *dashboard,
		Checks:      sanitizeChecksForList(filteredChecks),
		Results:     resultsMap,
		Summary:     summary,
		Incidents:   incidents,
		GeneratedAt: time.Now().UTC(),
	}

	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(data))
}

func (h *CustomDashboardAPIHandler) filterChecks(checks []CheckConfig, dashboard *CustomDashboard) []CheckConfig {
	// No filters = all checks
	if len(dashboard.CheckIDs) == 0 && len(dashboard.Tags) == 0 && len(dashboard.Servers) == 0 {
		return checks
	}

	// Build filter sets
	checkIDSet := make(map[string]bool)
	for _, id := range dashboard.CheckIDs {
		checkIDSet[id] = true
	}
	tagSet := make(map[string]bool)
	for _, t := range dashboard.Tags {
		tagSet[t] = true
	}
	serverSet := make(map[string]bool)
	for _, s := range dashboard.Servers {
		serverSet[s] = true
	}

	var filtered []CheckConfig
	for _, c := range checks {
		// Match by explicit ID
		if len(checkIDSet) > 0 && checkIDSet[c.ID] {
			filtered = append(filtered, c)
			continue
		}
		// Match by server
		if len(serverSet) > 0 && serverSet[c.Server] {
			filtered = append(filtered, c)
			continue
		}
		// Match by tag (using CheckConfig.Tags)
		if len(tagSet) > 0 && len(c.Tags) > 0 {
			for _, t := range c.Tags {
				if tagSet[t] {
					filtered = append(filtered, c)
					break
				}
			}
			continue
		}
	}
	return filtered
}
