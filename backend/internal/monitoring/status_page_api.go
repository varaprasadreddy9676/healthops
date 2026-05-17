package monitoring

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"
)

// StatusPageAPIHandler handles status page CRUD and public rendering.
type StatusPageAPIHandler struct {
	store            StatusPageRepository
	checkStore       Store
	incidentRepo     IncidentRepository
	maintenanceStore MaintenanceWindowStore
}

// NewStatusPageAPIHandler creates the handler.
func NewStatusPageAPIHandler(
	store StatusPageRepository,
	checkStore Store,
	incidentRepo IncidentRepository,
	maintenanceStore MaintenanceWindowStore,
) *StatusPageAPIHandler {
	return &StatusPageAPIHandler{
		store:            store,
		checkStore:       checkStore,
		incidentRepo:     incidentRepo,
		maintenanceStore: maintenanceStore,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (h *StatusPageAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	// Admin CRUD
	mux.HandleFunc("/api/v1/status-pages", h.handleStatusPages)
	mux.HandleFunc("/api/v1/status-pages/", h.handleStatusPageByID)
	// Public rendering (no auth required — handled in middleware exemption)
	mux.HandleFunc("/status/", h.handlePublicStatusPage)
}

// --- Admin CRUD ---

func (h *StatusPageAPIHandler) handleStatusPages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pages := h.store.List()
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(pages))
	case http.MethodPost:
		var cfg StatusPageConfig
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&cfg); err != nil {
			WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
			return
		}
		if cfg.Name == "" {
			WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
			return
		}
		if cfg.Slug == "" {
			WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("slug is required"))
			return
		}
		page, err := h.store.Create(cfg)
		if err != nil {
			if strings.Contains(err.Error(), "already in use") {
				WriteAPIError(w, http.StatusConflict, err)
				return
			}
			WriteAPIError(w, http.StatusInternalServerError, err)
			return
		}
		WriteAPIResponse(w, http.StatusCreated, NewAPIResponse(page))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *StatusPageAPIHandler) handleStatusPageByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/status-pages/")
	if path == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("status page ID required"))
		return
	}

	id := strings.SplitN(path, "/", 2)[0]

	switch r.Method {
	case http.MethodGet:
		page, err := h.store.Get(id)
		if err != nil {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(page))
	case http.MethodPut:
		var update StatusPageConfigUpdate
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&update); err != nil {
			WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
			return
		}
		page, err := h.store.UpdatePartial(id, update)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				WriteAPIError(w, http.StatusNotFound, err)
				return
			}
			WriteAPIError(w, http.StatusInternalServerError, err)
			return
		}
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(page))
	case http.MethodDelete:
		if err := h.store.Delete(id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				WriteAPIError(w, http.StatusNotFound, err)
				return
			}
			WriteAPIError(w, http.StatusInternalServerError, err)
			return
		}
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(map[string]string{"deleted": id}))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Public Status Page ---

func (h *StatusPageAPIHandler) handlePublicStatusPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	slug := strings.TrimPrefix(r.URL.Path, "/status/")
	slug = strings.TrimSuffix(slug, "/")
	if slug == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("status page slug required"))
		return
	}

	page, err := h.store.GetBySlug(slug)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, err)
		return
	}

	if !page.IsPublic {
		WriteAPIError(w, http.StatusNotFound, fmt.Errorf("status page not found"))
		return
	}

	response := h.buildPublicResponse(page)
	if wantsStatusPageHTML(r) {
		h.renderPublicStatusPage(w, response)
		return
	}
	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(response))
}

func wantsStatusPageHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	htmlIndex := strings.Index(accept, "text/html")
	if htmlIndex < 0 {
		return false
	}
	jsonIndex := strings.Index(accept, "application/json")
	return jsonIndex < 0 || htmlIndex < jsonIndex
}

func (h *StatusPageAPIHandler) renderPublicStatusPage(w http.ResponseWriter, resp StatusPageResponse) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' https: data:; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	w.WriteHeader(http.StatusOK)

	statusClass := statusPageStatusClass(resp.Status.Indicator)
	statusNote := statusPageStatusNote(resp.Status.Indicator)
	componentRows := buildStatusPageComponentRows(resp.Components)
	incidentRows := buildStatusPageIncidentRows(resp.Incidents)
	uptimeRows := buildStatusPageUptimeRows(resp.UptimeData)

	fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
:root{color-scheme:light;--bg:#f7f8fa;--panel:#fff;--text:#111827;--subtle:#6b7280;--muted:#8a95a3;--line:#e5e7eb;--line-strong:#d1d5db;--ok:#0a7f5a;--ok-bg:#e9f8f1;--warn:#a16207;--warn-bg:#fff7d6;--bad:#b42318;--bad-bg:#fff0ee;--maint:#475569;--maint-bg:#eef2f7}
*{box-sizing:border-box}html{background:var(--bg)}body{margin:0;background:linear-gradient(180deg,#fff 0,#f7f8fa 220px);color:var(--text);font-family:Inter,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;font-size:15px;line-height:1.45;-webkit-font-smoothing:antialiased}
.wrap{width:min(960px,calc(100%% - 48px));margin:0 auto;padding:38px 0 64px}.header{display:flex;align-items:flex-start;justify-content:space-between;gap:24px;margin-bottom:30px}
.brand{display:flex;align-items:center;gap:13px;min-width:0}.logo,.mark{width:38px;height:38px;border-radius:9px;flex:0 0 auto}.logo{object-fit:cover;background:#f1f5f9}.mark{display:grid;place-items:center;background:#111827;color:#fff;font-size:15px;font-weight:750;letter-spacing:0}
h1{margin:0;color:#111827;font-size:25px;font-weight:680;letter-spacing:0;line-height:1.2}.desc{max-width:620px;margin:4px 0 0;color:var(--subtle);font-size:14px;line-height:1.45}.stamp{margin-top:4px;color:var(--muted);font-size:12px;font-weight:520;text-align:right;white-space:nowrap}
.banner{margin:0 0 18px;border:1px solid #f2d48b;background:#fff9e8;color:#815b16;border-radius:8px;padding:11px 13px;font-size:13px;line-height:1.45}
.overview{display:flex;align-items:center;gap:15px;margin-bottom:30px;border:1px solid var(--line);background:var(--panel);border-radius:12px;padding:19px 20px;box-shadow:0 1px 2px rgba(16,24,40,.04)}
.indicator{width:14px;height:14px;border-radius:999px;flex:none}.overview.none .indicator{background:var(--ok);box-shadow:0 0 0 5px rgba(10,127,90,.11)}.overview.minor .indicator{background:#d99000;box-shadow:0 0 0 5px rgba(217,144,0,.14)}.overview.major .indicator,.overview.critical .indicator{background:#d92d20;box-shadow:0 0 0 5px rgba(217,45,32,.12)}
.status-title{margin:0;color:#111827;font-size:20px;font-weight:660;line-height:1.25}.status-note{margin-top:3px;color:var(--subtle);font-size:13px}
.section{margin-top:28px}.section-head{display:flex;align-items:baseline;justify-content:space-between;gap:16px;margin-bottom:10px}.section h2{margin:0;color:#1f2937;font-size:14px;font-weight:660;letter-spacing:0}.section-meta{color:var(--muted);font-size:12px}.panel{overflow:hidden;border:1px solid var(--line);border-radius:10px;background:var(--panel);box-shadow:0 1px 2px rgba(16,24,40,.03)}
.row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:18px;align-items:center;border-top:1px solid var(--line);padding:16px 18px}.row:first-child{border-top:0}.name{color:#111827;font-size:14px;font-weight:620;line-height:1.35}.sub{margin-top:4px;color:var(--subtle);font-size:12px;line-height:1.45}.pill{justify-self:end;display:inline-flex;align-items:center;min-height:26px;border-radius:999px;padding:4px 10px;font-size:12px;font-weight:620;line-height:1;text-align:center;white-space:nowrap}
.operational{background:var(--ok-bg);color:var(--ok)}.degraded_performance,.partial_outage{background:var(--warn-bg);color:var(--warn)}.major_outage{background:var(--bad-bg);color:var(--bad)}.under_maintenance{background:var(--maint-bg);color:var(--maint)}
.incident .name{font-weight:610}.incident.open{background:#fffafa}.empty{padding:18px;color:var(--subtle);font-size:13px}.uptime{display:grid;grid-template-columns:repeat(auto-fit,minmax(6px,1fr));gap:3px;padding:16px 18px}.bar{height:34px;border-radius:2px;background:#22c55e}.bar.warn{background:#f59e0b}.bar.bad{background:#ef4444}
@media(max-width:680px){body{background:#fff}.wrap{width:100%%;padding:24px 16px 42px}.header{display:block;margin-bottom:22px}.stamp{text-align:left;margin-top:12px}.overview{padding:17px 16px}.row{grid-template-columns:1fr;gap:9px;padding:15px 16px}.pill{justify-self:start}h1{font-size:22px}.status-title{font-size:18px}.section{margin-top:24px}}
</style>
</head>
<body>
<main class="wrap">
<header class="header">
<div class="brand">%s<div><h1>%s</h1><p class="desc">%s</p></div></div>
<div class="stamp">Updated %s</div>
</header>
%s
<section class="overview %s"><span class="indicator"></span><div><p class="status-title">%s</p><div class="status-note">%s</div></div></section>
<section class="section"><div class="section-head"><h2>Components</h2><span class="section-meta">Current status</span></div><div class="panel">%s</div></section>
<section class="section"><div class="section-head"><h2>Recent incidents</h2><span class="section-meta">Last 7 days</span></div><div class="panel">%s</div></section>
%s
</main>
</body>
</html>`,
		escapeHTML(resp.Page.Name),
		statusPageLogoHTML(resp.Page),
		escapeHTML(resp.Page.Name),
		escapeHTML(resp.Page.Description),
		escapeHTML(time.Now().UTC().Format("Jan 2, 2006 15:04 UTC")),
		statusPageAnnouncementHTML(resp.Page.Announcement),
		statusClass,
		escapeHTML(resp.Status.Description),
		escapeHTML(statusNote),
		componentRows,
		incidentRows,
		uptimeRows,
	)
}

func statusPageLogoHTML(page StatusPageMeta) string {
	if page.LogoURL == "" {
		name := strings.TrimSpace(page.Name)
		initial := "H"
		if name != "" {
			initial = strings.ToUpper(string([]rune(name)[0]))
		}
		return fmt.Sprintf(`<div class="mark" aria-hidden="true">%s</div>`, escapeHTML(initial))
	}
	return fmt.Sprintf(`<img class="logo" src="%s" alt="">`, escapeHTML(page.LogoURL))
}

func statusPageAnnouncementHTML(announcement string) string {
	if strings.TrimSpace(announcement) == "" {
		return ""
	}
	return fmt.Sprintf(`<div class="banner">%s</div>`, escapeHTML(announcement))
}

func buildStatusPageComponentRows(components []StatusPageComponentState) string {
	if len(components) == 0 {
		return `<div class="empty">No public components are configured.</div>`
	}
	var b strings.Builder
	for _, comp := range components {
		sub := statusPageComponentSubtext(comp)
		fmt.Fprintf(&b, `<div class="row component"><div><div class="name">%s</div>%s</div><span class="pill %s">%s</span></div>`,
			escapeHTML(comp.Name),
			sub,
			escapeHTML(string(comp.Status)),
			escapeHTML(statusPageComponentLabel(comp.Status)),
		)
	}
	return b.String()
}

func statusPageComponentSubtext(comp StatusPageComponentState) string {
	parts := make([]string, 0, 4)
	if comp.Description != "" {
		parts = append(parts, comp.Description)
	}
	if comp.Uptime > 0 {
		parts = append(parts, fmt.Sprintf("%.2f%% uptime", comp.Uptime))
	}
	if comp.Latency > 0 {
		parts = append(parts, fmt.Sprintf("%d ms latency", comp.Latency))
	}
	if comp.LastChecked != "" {
		parts = append(parts, "checked "+formatPublicStatusTimestamp(comp.LastChecked))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(`<div class="sub">%s</div>`, escapeHTML(strings.Join(parts, " · ")))
}

func buildStatusPageIncidentRows(incidents []StatusPageIncident) string {
	if len(incidents) == 0 {
		return `<div class="empty">No incidents reported in the last 7 days.</div>`
	}
	var b strings.Builder
	for _, inc := range incidents {
		when := formatPublicStatusTime(inc.CreatedAt)
		status := statusPageIncidentLabel(inc.Status)
		if inc.ResolvedAt != nil {
			status = "Resolved " + inc.ResolvedAt.UTC().Format("Jan 2, 15:04")
		}
		fmt.Fprintf(&b, `<div class="row incident %s"><div><div class="name">%s</div><div class="sub">%s · %s</div></div><span class="pill %s">%s</span></div>`,
			escapeHTML(inc.Status),
			escapeHTML(inc.Title),
			escapeHTML(when),
			escapeHTML(inc.Severity),
			escapeHTML(statusPageIncidentClass(inc.Status)),
			escapeHTML(status),
		)
	}
	return b.String()
}

func buildStatusPageUptimeRows(days []UptimeDayEntry) string {
	if len(days) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="section"><div class="section-head"><h2>Uptime history</h2><span class="section-meta">Daily availability</span></div><div class="panel"><div class="uptime" aria-label="Uptime history">`)
	for _, day := range days {
		className := "bar"
		if day.Uptime < 95 {
			className += " bad"
		} else if day.Uptime < 99.9 {
			className += " warn"
		}
		fmt.Fprintf(&b, `<span class="%s" title="%s: %.2f%%"></span>`, className, escapeHTML(day.Date), day.Uptime)
	}
	b.WriteString(`</div></div></section>`)
	return b.String()
}

func statusPageStatusClass(indicator string) string {
	switch indicator {
	case "minor", "major", "critical":
		return indicator
	default:
		return "none"
	}
}

func statusPageStatusNote(indicator string) string {
	switch indicator {
	case "minor":
		return "Some components are degraded. We are monitoring the issue."
	case "major", "critical":
		return "One or more components are unavailable or severely degraded."
	default:
		return "All monitored components are reporting healthy checks."
	}
}

func statusPageComponentLabel(status StatusPageComponentStatus) string {
	switch status {
	case ComponentOperational:
		return "Operational"
	case ComponentDegraded:
		return "Degraded"
	case ComponentPartialOutage:
		return "Partial outage"
	case ComponentMajorOutage:
		return "Major outage"
	case ComponentUnderMaintenance:
		return "Maintenance"
	default:
		return string(status)
	}
}

func statusPageIncidentClass(status string) string {
	if status == "resolved" {
		return "operational"
	}
	return "major_outage"
}

func statusPageIncidentLabel(status string) string {
	switch status {
	case "open":
		return "Investigating"
	case "acknowledged":
		return "Monitoring"
	case "resolved":
		return "Resolved"
	default:
		return status
	}
}

func formatPublicStatusTimestamp(value string) string {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return formatPublicStatusTime(parsed)
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return formatPublicStatusTime(parsed)
	}
	return value
}

func formatPublicStatusTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("Jan 2, 15:04 UTC")
}

func escapeHTML(value string) string {
	return html.EscapeString(value)
}

func (h *StatusPageAPIHandler) buildPublicResponse(page *StatusPageConfig) StatusPageResponse {
	state := h.checkStore.Snapshot()

	// Build component states
	componentStates := make([]StatusPageComponentState, 0, len(page.Components))
	var hasMinor, hasMajor, hasCritical bool

	for _, comp := range page.Components {
		compState := h.resolveComponentState(comp, state)
		componentStates = append(componentStates, compState)

		switch compState.Status {
		case ComponentDegraded, ComponentPartialOutage:
			hasMinor = true
		case ComponentMajorOutage:
			hasMajor = true
		case ComponentUnderMaintenance:
			hasMinor = true
		}
	}

	// Overall status
	overall := OverallStatus{Indicator: "none", Description: "All Systems Operational"}
	if hasCritical || hasMajor {
		overall = OverallStatus{Indicator: "major", Description: "Major System Outage"}
	} else if hasMinor {
		overall = OverallStatus{Indicator: "minor", Description: "Minor Service Disruption"}
	}

	resp := StatusPageResponse{
		Page: StatusPageMeta{
			Name:         page.Name,
			Description:  page.Description,
			LogoURL:      page.LogoURL,
			Announcement: page.AnnouncementMsg,
		},
		Status:     overall,
		Components: componentStates,
	}

	// Include incidents if configured
	if page.ShowIncidents && h.incidentRepo != nil {
		resp.Incidents = h.getRecentIncidents(page)
	}

	// Include uptime if configured
	if page.ShowUptime {
		resp.UptimeData = h.computeUptimeHistory(page, state)
	}

	return resp
}

func (h *StatusPageAPIHandler) resolveComponentState(comp StatusPageComponent, state State) StatusPageComponentState {
	cs := StatusPageComponentState{
		ID:          comp.ID,
		Name:        comp.Name,
		Description: comp.Description,
		Group:       comp.Group,
	}

	// Find matching checks
	matchingChecks := h.matchComponentChecks(comp, state.Checks)
	if len(matchingChecks) == 0 {
		cs.Status = ComponentOperational
		return cs
	}

	// Check maintenance
	if h.maintenanceStore != nil {
		for _, check := range matchingChecks {
			if h.maintenanceStore.IsCheckInMaintenance(check) {
				cs.Status = ComponentUnderMaintenance
				return cs
			}
		}
	}

	// Get latest results for matching checks
	latestResults := make(map[string]CheckResult)
	for _, r := range state.Results {
		for _, check := range matchingChecks {
			if r.CheckID == check.ID {
				if existing, ok := latestResults[r.CheckID]; !ok || r.FinishedAt.After(existing.FinishedAt) {
					latestResults[r.CheckID] = r
				}
			}
		}
	}

	// Determine component status from check results
	total := len(matchingChecks)
	var healthy, unhealthy int
	var maxLatency int64

	for _, r := range latestResults {
		if r.Healthy {
			healthy++
		} else {
			unhealthy++
		}
		if r.DurationMs > maxLatency {
			maxLatency = r.DurationMs
		}
		cs.LastChecked = r.FinishedAt.Format(time.RFC3339)
	}

	cs.Latency = maxLatency

	if unhealthy == 0 {
		cs.Status = ComponentOperational
	} else if unhealthy == total {
		cs.Status = ComponentMajorOutage
	} else if float64(unhealthy)/float64(total) > 0.5 {
		cs.Status = ComponentPartialOutage
	} else {
		cs.Status = ComponentDegraded
	}

	// Compute uptime (from results)
	if total > 0 && len(latestResults) > 0 {
		cs.Uptime = float64(healthy) / float64(len(latestResults)) * 100
	}

	return cs
}

func (h *StatusPageAPIHandler) matchComponentChecks(comp StatusPageComponent, checks []CheckConfig) []CheckConfig {
	if len(comp.CheckIDs) == 0 && len(comp.Tags) == 0 && len(comp.Servers) == 0 {
		return nil
	}

	checkIDSet := make(map[string]bool)
	for _, id := range comp.CheckIDs {
		checkIDSet[id] = true
	}
	tagSet := make(map[string]bool)
	for _, t := range comp.Tags {
		tagSet[t] = true
	}
	serverSet := make(map[string]bool)
	for _, s := range comp.Servers {
		serverSet[s] = true
	}

	var matched []CheckConfig
	for _, c := range checks {
		if len(checkIDSet) > 0 && checkIDSet[c.ID] {
			matched = append(matched, c)
			continue
		}
		if len(serverSet) > 0 && serverSet[c.Server] {
			matched = append(matched, c)
			continue
		}
		if len(tagSet) > 0 && len(c.Tags) > 0 {
			for _, t := range c.Tags {
				if tagSet[t] {
					matched = append(matched, c)
					break
				}
			}
			continue
		}
	}
	return matched
}

func (h *StatusPageAPIHandler) getRecentIncidents(page *StatusPageConfig) []StatusPageIncident {
	allIncidents, err := h.incidentRepo.ListIncidents()
	if err != nil {
		return nil
	}

	// Collect check IDs from all components
	checkIDs := make(map[string]bool)
	state := h.checkStore.Snapshot()
	for _, comp := range page.Components {
		matched := h.matchComponentChecks(comp, state.Checks)
		for _, c := range matched {
			checkIDs[c.ID] = true
		}
	}

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour) // last 7 days
	var incidents []StatusPageIncident

	for _, inc := range allIncidents {
		if !checkIDs[inc.CheckID] {
			continue
		}
		if inc.StartedAt.Before(cutoff) {
			continue
		}

		spi := StatusPageIncident{
			ID:        inc.ID,
			Title:     publicStatusIncidentTitle(inc),
			Status:    inc.Status,
			Severity:  inc.Severity,
			CreatedAt: inc.StartedAt,
		}
		if inc.Status == "resolved" && inc.ResolvedAt != nil {
			spi.ResolvedAt = inc.ResolvedAt
		}
		incidents = append(incidents, spi)
	}

	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].CreatedAt.After(incidents[j].CreatedAt)
	})

	// Limit to 20 most recent
	if len(incidents) > 20 {
		incidents = incidents[:20]
	}
	return incidents
}

func publicStatusIncidentTitle(inc Incident) string {
	messageParts := parseAlertMessageParts(inc.Message)
	detail := strings.TrimSpace(messageParts["details"])
	if detail == "" && strings.EqualFold(messageParts["rule"], "High Latency") && messageParts["duration"] != "" {
		detail = fmt.Sprintf("response time exceeded threshold (%s)", messageParts["duration"])
	}
	if detail == "" {
		detail = strings.TrimSpace(inc.Message)
		filtered := make([]string, 0)
		for _, part := range strings.Split(detail, "|") {
			part = strings.TrimSpace(part)
			if part == "" || statusPageInternalMessagePart(part) {
				continue
			}
			filtered = append(filtered, part)
		}
		detail = strings.Join(filtered, " | ")
	}
	for _, marker := range []string{" | Duration:", " | Description:"} {
		if idx := strings.Index(detail, marker); idx >= 0 {
			detail = strings.TrimSpace(detail[:idx])
		}
	}
	detail = strings.Trim(detail, " |")
	detail = humanizePublicIncidentDetail(detail)
	if len(detail) > 180 {
		detail = strings.TrimSpace(detail[:177]) + "..."
	}
	checkName := strings.TrimSpace(inc.CheckName)
	if checkName == "" {
		checkName = strings.TrimSpace(inc.CheckID)
	}
	if detail == "" {
		return checkName
	}
	if checkName == "" {
		return detail
	}
	return fmt.Sprintf("%s: %s", checkName, detail)
}

func humanizePublicIncidentDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return detail
	}
	if strings.HasPrefix(detail, "Get \"") {
		if _, rest, ok := strings.Cut(detail, "\":"); ok {
			detail = strings.TrimSpace(rest)
		}
	}
	lower := strings.ToLower(detail)
	if strings.Contains(lower, "dial tcp: lookup ") && strings.Contains(lower, "no such host") {
		if host := extractBetween(detail, "lookup ", " on "); host != "" {
			return "could not resolve " + host
		}
		return "could not resolve monitored endpoint"
	}
	if strings.Contains(lower, "unexpected status code ") {
		code := strings.TrimSpace(detail[strings.LastIndex(lower, "unexpected status code ")+len("unexpected status code "):])
		if code != "" {
			return "unexpected HTTP status " + code
		}
	}
	if strings.HasPrefix(lower, "slow api response:") {
		value := strings.TrimSpace(detail[len("slow api response:"):])
		value = strings.ReplaceAll(value, "ms", " ms")
		return "slow API response (" + strings.TrimSpace(value) + ")"
	}
	return detail
}

func extractBetween(value, prefix, suffix string) string {
	start := strings.Index(value, prefix)
	if start < 0 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(value[start:], suffix)
	if end < 0 {
		return strings.TrimSpace(value[start:])
	}
	return strings.TrimSpace(value[start : start+end])
}

func parseAlertMessageParts(message string) map[string]string {
	parts := make(map[string]string)
	for _, part := range strings.Split(message, "|") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		parts[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return parts
}

func statusPageInternalMessagePart(part string) bool {
	key, _, ok := strings.Cut(part, ":")
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "rule", "check", "status", "description":
		return true
	default:
		return false
	}
}

func (h *StatusPageAPIHandler) computeUptimeHistory(page *StatusPageConfig, state State) []UptimeDayEntry {
	days := page.UptimeDays
	if days <= 0 {
		days = 90
	}
	if days > 365 {
		days = 365
	}

	// Simple uptime computation: for each day, count healthy vs total results
	now := time.Now().UTC()
	entries := make([]UptimeDayEntry, 0, days)

	// Collect all check IDs on this page
	checkIDs := make(map[string]bool)
	for _, comp := range page.Components {
		matched := h.matchComponentChecks(comp, state.Checks)
		for _, c := range matched {
			checkIDs[c.ID] = true
		}
	}

	// Group results by day
	dayResults := make(map[string]struct{ healthy, total int })
	for _, r := range state.Results {
		if !checkIDs[r.CheckID] {
			continue
		}
		day := r.FinishedAt.Format("2006-01-02")
		dr := dayResults[day]
		dr.total++
		if r.Healthy {
			dr.healthy++
		}
		dayResults[day] = dr
	}

	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		entry := UptimeDayEntry{Date: day, Uptime: 100.0}
		if dr, ok := dayResults[day]; ok && dr.total > 0 {
			entry.Uptime = float64(dr.healthy) / float64(dr.total) * 100
		}
		entries = append(entries, entry)
	}

	return entries
}
