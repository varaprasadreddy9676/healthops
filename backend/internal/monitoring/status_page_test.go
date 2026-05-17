package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStatusPageStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStatusPageStore(dir + "/pages.json")
	if err != nil {
		t.Fatalf("NewStatusPageStore: %v", err)
	}

	page, err := store.Create(StatusPageConfig{
		Name:     "Acme Status",
		Slug:     "acme-status",
		IsPublic: true,
		Components: []StatusPageComponent{
			{ID: "c1", Name: "API", CheckIDs: []string{"api-health"}},
			{ID: "c2", Name: "Database", CheckIDs: []string{"db-health"}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if page.ID == "" {
		t.Error("expected non-empty ID")
	}
	if page.Slug != "acme-status" {
		t.Errorf("Slug = %q; want 'acme-status'", page.Slug)
	}
	if !page.IsPublic {
		t.Error("expected IsPublic = true")
	}
	if len(page.Components) != 2 {
		t.Errorf("len(Components) = %d; want 2", len(page.Components))
	}

	// Get
	got, err := store.Get(page.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Acme Status" {
		t.Errorf("Get Name = %q", got.Name)
	}
}

func TestStatusPageStore_SlugUniqueness(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	store.Create(StatusPageConfig{Name: "First", Slug: "my-slug"})
	_, err := store.Create(StatusPageConfig{Name: "Second", Slug: "my-slug"})
	if err == nil || !strings.Contains(err.Error(), "slug") {
		t.Errorf("expected slug uniqueness error, got: %v", err)
	}
}

func TestStatusPageStore_MissingFields(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	_, err := store.Create(StatusPageConfig{})
	if err == nil {
		t.Error("expected error for missing name")
	}

	_, err = store.Create(StatusPageConfig{Name: "No Slug"})
	if err == nil {
		t.Error("expected error for missing slug")
	}
}

func TestStatusPageStore_GetBySlug(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	store.Create(StatusPageConfig{Name: "Page", Slug: "test-page", IsPublic: true})

	got, err := store.GetBySlug("test-page")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.Name != "Page" {
		t.Errorf("Name = %q; want 'Page'", got.Name)
	}

	_, err = store.GetBySlug("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent slug")
	}
}

func TestStatusPageStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	page, _ := store.Create(StatusPageConfig{Name: "Old", Slug: "old-slug"})

	updated, err := store.Update(page.ID, StatusPageConfig{
		Name:     "New Name",
		Slug:     "new-slug",
		IsPublic: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("Name = %q; want 'New Name'", updated.Name)
	}
	if updated.Slug != "new-slug" {
		t.Errorf("Slug = %q; want 'new-slug'", updated.Slug)
	}
}

func TestStatusPageStore_UpdateSlugConflict(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	store.Create(StatusPageConfig{Name: "First", Slug: "slug-a"})
	second, _ := store.Create(StatusPageConfig{Name: "Second", Slug: "slug-b"})

	_, err := store.Update(second.ID, StatusPageConfig{Name: "Second", Slug: "slug-a"})
	if err == nil || !strings.Contains(err.Error(), "slug") {
		t.Errorf("expected slug conflict error, got: %v", err)
	}
}

func TestStatusPageStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	page, _ := store.Create(StatusPageConfig{Name: "Del", Slug: "del"})
	if err := store.Delete(page.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(page.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestStatusPageStore_DefaultUptimeDays(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	page, _ := store.Create(StatusPageConfig{Name: "P", Slug: "p"})
	if page.UptimeDays != 90 {
		t.Errorf("UptimeDays = %d; want 90", page.UptimeDays)
	}
}

func TestStatusPageStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pages.json"

	store1, _ := NewStatusPageStore(path)
	store1.Create(StatusPageConfig{Name: "Persist", Slug: "persist"})

	store2, _ := NewStatusPageStore(path)
	all := store2.List()
	if len(all) != 1 || all[0].Name != "Persist" {
		t.Errorf("persistence failed: %+v", all)
	}
}

// --- API Handler Tests ---

func TestStatusPageAPI_CRUD(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")
	fakeS := &fakeStore{snapshot: State{}}
	handler := NewStatusPageAPIHandler(store, fakeS, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create
	body := `{"name":"Status","slug":"status","isPublic":true,"components":[{"id":"c1","name":"Web","checkIds":["web-check"]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/status-pages", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create: status = %d; body: %s", w.Code, w.Body.String())
	}

	var createResp struct {
		Data StatusPageConfig `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	id := createResp.Data.ID

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/v1/status-pages", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("List: status = %d", w.Code)
	}

	// Get
	req = httptest.NewRequest(http.MethodGet, "/api/v1/status-pages/"+id, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Get: status = %d", w.Code)
	}

	// Update
	body = `{"name":"Updated Status","slug":"status","isPublic":false}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/status-pages/"+id, strings.NewReader(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Update: status = %d; body: %s", w.Code, w.Body.String())
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/status-pages/"+id, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Delete: status = %d", w.Code)
	}
}

func TestStatusPageAPI_PublicPage(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	enabled := true
	checks := []CheckConfig{
		{ID: "web-check", Name: "Web", Type: "api", Enabled: &enabled},
		{ID: "db-check", Name: "DB", Type: "tcp", Enabled: &enabled},
	}
	results := []CheckResult{
		{CheckID: "web-check", Status: "healthy", Healthy: true, StartedAt: time.Now()},
		{CheckID: "db-check", Status: "critical", Healthy: false, StartedAt: time.Now()},
	}
	fakeS := &fakeStore{snapshot: State{Checks: checks, Results: results}}
	handler := NewStatusPageAPIHandler(store, fakeS, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create a public page
	store.Create(StatusPageConfig{
		Name:     "Public Status",
		Slug:     "public",
		IsPublic: true,
		Components: []StatusPageComponent{
			{ID: "c1", Name: "Web", CheckIDs: []string{"web-check"}},
			{ID: "c2", Name: "DB", CheckIDs: []string{"db-check"}},
		},
		ShowIncidents: true,
		ShowUptime:    true,
	})

	// Access public page
	req := httptest.NewRequest(http.MethodGet, "/status/public", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Public page: status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data StatusPageResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.Page.Name != "Public Status" {
		t.Errorf("Name = %q", resp.Data.Page.Name)
	}
	if len(resp.Data.Components) != 2 {
		t.Errorf("Components = %d; want 2", len(resp.Data.Components))
	}
	// web-check is healthy, db-check is critical → overall should not be "none"
	if resp.Data.Status.Indicator == "none" {
		t.Error("overall status should not be 'none' when one check is critical")
	}
}

func TestStatusPageAPI_PublicPageHTML(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	enabled := true
	checks := []CheckConfig{
		{ID: "web-check", Name: "Web", Type: "api", Enabled: &enabled},
	}
	results := []CheckResult{
		{CheckID: "web-check", Status: "healthy", Healthy: true, StartedAt: time.Now(), FinishedAt: time.Now(), DurationMs: 42},
	}
	fakeS := &fakeStore{snapshot: State{Checks: checks, Results: results}}
	handler := NewStatusPageAPIHandler(store, fakeS, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	store.Create(StatusPageConfig{
		Name:            `<script>alert("x")</script>`,
		Slug:            "html",
		Description:     "Browser-rendered status page",
		IsPublic:        true,
		ShowIncidents:   true,
		ShowUptime:      true,
		AnnouncementMsg: `<b>maintenance</b>`,
		Components: []StatusPageComponent{
			{ID: "c1", Name: "Web", CheckIDs: []string{"web-check"}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/status/html", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Public page: status = %d; body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q; want text/html", got)
	}
	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "style-src 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy = %q; want inline styles allowed for rendered page", got)
	}
	body := w.Body.String()
	if strings.Contains(body, `"success":true`) {
		t.Fatalf("HTML response should not be API JSON: %s", body)
	}
	if strings.Contains(body, `<script>alert("x")</script>`) || strings.Contains(body, `<b>maintenance</b>`) {
		t.Fatalf("HTML response contains unescaped configured content: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "Operational") {
		t.Fatalf("HTML response missing escaped page name or component status: %s", body)
	}
}

func TestStatusPageAPI_PublicPageJSONWhenRequested(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")
	handler := NewStatusPageAPIHandler(store, &fakeStore{snapshot: State{}}, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	store.Create(StatusPageConfig{
		Name:     "JSON Status",
		Slug:     "json",
		IsPublic: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/status/json", nil)
	req.Header.Set("Accept", "application/json,text/html")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Public page: status = %d; body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q; want application/json", got)
	}
	var resp struct {
		Data StatusPageResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Data.Page.Name != "JSON Status" {
		t.Fatalf("Page.Name = %q; want JSON Status", resp.Data.Page.Name)
	}
}

func TestPublicStatusIncidentTitleHidesRuleNoise(t *testing.T) {
	title := publicStatusIncidentTitle(Incident{
		CheckName: "Checkout API Health",
		Message:   `Rule: Check Down | Check: Checkout API Health | Status: critical | Details: unexpected status code 503 | Duration: 1ms | Description: Fires when any health check reports unhealthy`,
	})
	if title != "Checkout API Health: unexpected HTTP status 503" {
		t.Fatalf("title = %q", title)
	}
}

func TestPublicStatusIncidentTitleExplainsHighLatencyWithoutRuleNoise(t *testing.T) {
	title := publicStatusIncidentTitle(Incident{
		CheckName: "MySQL Database Health",
		Message:   `Rule: High Latency | Check: MySQL Database Health | Status: healthy | Duration: 5018ms | Description: Fires when response time exceeds 3 seconds`,
	})
	if title != "MySQL Database Health: response time exceeded threshold (5018ms)" {
		t.Fatalf("title = %q", title)
	}
}

func TestPublicStatusIncidentTitleHumanizesNetworkDetails(t *testing.T) {
	title := publicStatusIncidentTitle(Incident{
		CheckName: "Checkout API Health",
		Message:   `Rule: Check Down | Check: Checkout API Health | Status: critical | Details: Get "http://demo-api:9100/health": dial tcp: lookup demo-api on 127.0.0.11:53: no such host | Duration: 3ms`,
	})
	if title != "Checkout API Health: could not resolve demo-api" {
		t.Fatalf("title = %q", title)
	}
}

func TestPublicStatusIncidentTitleHumanizesHTTPStatus(t *testing.T) {
	title := publicStatusIncidentTitle(Incident{
		CheckName: "Checkout API Health",
		Message:   `Rule: Check Down | Check: Checkout API Health | Status: critical | Details: unexpected status code 503 | Duration: 1ms`,
	})
	if title != "Checkout API Health: unexpected HTTP status 503" {
		t.Fatalf("title = %q", title)
	}
}

func TestStatusPageAPI_PublicPageNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")
	handler := NewStatusPageAPIHandler(store, &fakeStore{snapshot: State{}}, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/status/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
}

func TestStatusPageAPI_PrivatePageDenied(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")
	handler := NewStatusPageAPIHandler(store, &fakeStore{snapshot: State{}}, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	store.Create(StatusPageConfig{Name: "Private", Slug: "private", IsPublic: false})

	req := httptest.NewRequest(http.MethodGet, "/status/private", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404 for private page", w.Code)
	}
}

func TestStatusPageAPI_ComponentStateResolution(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")

	// Checks: all healthy
	enabled := true
	checks := []CheckConfig{
		{ID: "c1", Name: "Check1", Type: "api", Enabled: &enabled},
		{ID: "c2", Name: "Check2", Type: "api", Enabled: &enabled},
	}
	results := []CheckResult{
		{CheckID: "c1", Status: "healthy", Healthy: true, StartedAt: time.Now()},
		{CheckID: "c2", Status: "healthy", Healthy: true, StartedAt: time.Now()},
	}
	fakeS := &fakeStore{snapshot: State{Checks: checks, Results: results}}
	handler := NewStatusPageAPIHandler(store, fakeS, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	store.Create(StatusPageConfig{
		Name:     "All Green",
		Slug:     "green",
		IsPublic: true,
		Components: []StatusPageComponent{
			{ID: "comp1", Name: "All", CheckIDs: []string{"c1", "c2"}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/status/green", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var resp struct {
		Data StatusPageResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.Status.Indicator != "none" {
		t.Errorf("Status.Indicator = %q; want 'none'", resp.Data.Status.Indicator)
	}
	if len(resp.Data.Components) > 0 && resp.Data.Components[0].Status != ComponentOperational {
		t.Errorf("Component status = %q; want %q", resp.Data.Components[0].Status, ComponentOperational)
	}
}

func TestStatusPageAPI_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStatusPageStore(dir + "/pages.json")
	handler := NewStatusPageAPIHandler(store, &fakeStore{snapshot: State{}}, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/status-pages", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestStatusPageStore_BadFile(t *testing.T) {
	dir := t.TempDir()
	badFile := dir + "/bad.json"
	os.WriteFile(badFile, []byte("{invalid"), 0644)

	_, err := NewStatusPageStore(badFile)
	if err == nil {
		t.Error("expected error for invalid JSON file")
	}
}
