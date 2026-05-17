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

func TestCustomDashboardStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCustomDashboardStore(dir + "/dashboards.json")
	if err != nil {
		t.Fatalf("NewCustomDashboardStore: %v", err)
	}

	d, err := store.Create(CustomDashboard{
		Name:        "Prod Overview",
		Description: "Production servers",
		Visibility:  "team",
		Servers:     []string{"web-1", "web-2"},
		Widgets: []DashboardWidget{
			{ID: "w1", Type: WidgetStatusGrid, Title: "Status", Position: WidgetPosition{X: 0, Y: 0, Width: 12, Height: 4}},
			{ID: "w2", Type: WidgetLatencyChart, Title: "Latency", Position: WidgetPosition{X: 0, Y: 4, Width: 6, Height: 4}},
		},
		RefreshSec: 30,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.ID == "" {
		t.Error("expected non-empty ID")
	}
	if d.Name != "Prod Overview" {
		t.Errorf("Name = %q; want 'Prod Overview'", d.Name)
	}
	if len(d.Widgets) != 2 {
		t.Errorf("len(Widgets) = %d; want 2", len(d.Widgets))
	}
	if d.Visibility != "team" {
		t.Errorf("Visibility = %q; want 'team'", d.Visibility)
	}

	// Get
	got, err := store.Get(d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Prod Overview" {
		t.Errorf("Get Name = %q; want 'Prod Overview'", got.Name)
	}
}

func TestCustomDashboardStore_CreateMissingName(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	_, err := store.Create(CustomDashboard{})
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("expected error about name, got: %v", err)
	}
}

func TestCustomDashboardStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")

	d, _ := store.Create(CustomDashboard{Name: "Test", Visibility: "private"})

	updated, err := store.Update(d.ID, CustomDashboard{
		Name:       "Updated",
		Visibility: "public",
		CheckIDs:   []string{"check-1", "check-2"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("Name = %q; want 'Updated'", updated.Name)
	}
	if updated.Visibility != "public" {
		t.Errorf("Visibility = %q; want 'public'", updated.Visibility)
	}
	if len(updated.CheckIDs) != 2 {
		t.Errorf("len(CheckIDs) = %d; want 2", len(updated.CheckIDs))
	}
}

func TestCustomDashboardStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")

	d, _ := store.Create(CustomDashboard{Name: "ToDelete"})
	if err := store.Delete(d.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Get(d.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCustomDashboardStore_DeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	err := store.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestCustomDashboardStore_Duplicate(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")

	original, _ := store.Create(CustomDashboard{
		Name:     "Original",
		Widgets:  []DashboardWidget{{ID: "w1", Type: WidgetSummary, Title: "Summary"}},
		CheckIDs: []string{"a", "b"},
	})

	dup, err := store.Duplicate(original.ID, "Copy of Original")
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup.ID == original.ID {
		t.Error("duplicate should have different ID")
	}
	if dup.Name != "Copy of Original" {
		t.Errorf("Name = %q; want 'Copy of Original'", dup.Name)
	}
	if len(dup.Widgets) != 1 {
		t.Errorf("Widgets not copied")
	}
	if len(dup.CheckIDs) != 2 {
		t.Errorf("CheckIDs not copied")
	}
}

func TestCustomDashboardStore_List(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")

	store.Create(CustomDashboard{Name: "A", Owner: "user1", Visibility: "private"})
	store.Create(CustomDashboard{Name: "B", Owner: "user2", Visibility: "public"})
	store.Create(CustomDashboard{Name: "C", Owner: "user1", Visibility: "team"})

	// List all
	all := store.List("")
	if len(all) != 3 {
		t.Errorf("List all: got %d; want 3", len(all))
	}

	// List for user1 (should see own + public + team)
	user1 := store.List("user1")
	if len(user1) != 3 {
		t.Errorf("List user1: got %d; want 3", len(user1))
	}
}

func TestCustomDashboardStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/dashboards.json"

	store1, _ := NewCustomDashboardStore(path)
	store1.Create(CustomDashboard{Name: "Persistent", Visibility: "private"})

	// Reload from disk
	store2, err := NewCustomDashboardStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	all := store2.List("")
	if len(all) != 1 || all[0].Name != "Persistent" {
		t.Errorf("persistence failed: got %+v", all)
	}
}

func TestCustomDashboardStore_DefaultVisibility(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	d, _ := store.Create(CustomDashboard{Name: "NoVis"})
	if d.Visibility != "private" {
		t.Errorf("default Visibility = %q; want 'private'", d.Visibility)
	}
}

// --- API Handler Tests ---

func TestCustomDashboardAPI_CreateAndList(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	fakeStore := &fakeStore{snapshot: State{}}
	handler := NewCustomDashboardAPIHandler(store, fakeStore, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create
	body := `{"name":"My Dashboard","visibility":"public","widgets":[{"id":"w1","type":"status_grid","title":"Grid","position":{"x":0,"y":0,"w":12,"h":4}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboards", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Create: status = %d; want 201. Body: %s", w.Code, w.Body.String())
	}

	var createResp struct {
		Data CustomDashboard `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Data.Name != "My Dashboard" {
		t.Errorf("Name = %q", createResp.Data.Name)
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/v1/dashboards", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List: status = %d", w.Code)
	}
}

func TestCustomDashboardAPI_GetAndUpdate(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	fakeStore := &fakeStore{snapshot: State{}}
	handler := NewCustomDashboardAPIHandler(store, fakeStore, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create first
	d, _ := store.Create(CustomDashboard{Name: "ToGet", Visibility: "private"})

	// Get
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards/"+d.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Get: status = %d", w.Code)
	}

	// Update
	body := `{"name":"Renamed","visibility":"team"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/dashboards/"+d.ID, strings.NewReader(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Update: status = %d; body: %s", w.Code, w.Body.String())
	}

	var updateResp struct {
		Data CustomDashboard `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &updateResp)
	if updateResp.Data.Name != "Renamed" {
		t.Errorf("Updated Name = %q; want 'Renamed'", updateResp.Data.Name)
	}
}

func TestCustomDashboardAPI_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	handler := NewCustomDashboardAPIHandler(store, &fakeStore{snapshot: State{}}, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	d, _ := store.Create(CustomDashboard{Name: "ToDelete"})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/dashboards/"+d.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Delete: status = %d", w.Code)
	}

	// Verify gone
	req = httptest.NewRequest(http.MethodGet, "/api/v1/dashboards/"+d.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Get after delete: status = %d; want 404", w.Code)
	}
}

func TestCustomDashboardAPI_Duplicate(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	handler := NewCustomDashboardAPIHandler(store, &fakeStore{snapshot: State{}}, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	d, _ := store.Create(CustomDashboard{Name: "Original", Widgets: []DashboardWidget{{ID: "w1", Type: WidgetText, Title: "Note"}}})

	body := `{"name":"Cloned"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboards/"+d.ID+"/duplicate", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Duplicate: status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCustomDashboardAPI_DuplicateAssignsCallerOwner(t *testing.T) {
	InitJWTSecret(t.TempDir())
	token, err := signJWT(JWTClaims{
		UserID:   "u2",
		Username: "new-owner",
		Role:     RoleAdmin,
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("signJWT: %v", err)
	}

	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	handler := NewCustomDashboardAPIHandler(store, &fakeStore{snapshot: State{}}, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	d, _ := store.Create(CustomDashboard{
		Name:       "Team Source",
		Owner:      "old-owner",
		Visibility: "team",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboards/"+d.ID+"/duplicate", strings.NewReader(`{"name":"Copy"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Duplicate: status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data CustomDashboard `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Owner != "new-owner" {
		t.Fatalf("duplicate owner = %q; want new-owner", resp.Data.Owner)
	}
}

func TestCustomDashboardAPI_GetData(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")

	checks := []CheckConfig{
		{ID: "api-1", Name: "API 1", Type: "api", Server: "web-1"},
		{ID: "api-2", Name: "API 2", Type: "api", Server: "web-2"},
		{ID: "db-1", Name: "DB 1", Type: "tcp", Server: "db-1"},
	}
	results := []CheckResult{
		{CheckID: "api-1", Status: "healthy", Healthy: true},
		{CheckID: "api-2", Status: "critical", Healthy: false},
	}

	fakeStore := &fakeStore{snapshot: State{Checks: checks, Results: results}}
	handler := NewCustomDashboardAPIHandler(store, fakeStore, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Dashboard filtered to web servers only
	d, _ := store.Create(CustomDashboard{Name: "Web Only", Servers: []string{"web-1", "web-2"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards/"+d.ID+"/data", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetData: status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data DashboardData `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Should only contain web-1 and web-2 checks, not db-1
	if len(resp.Data.Checks) != 2 {
		t.Errorf("Filtered checks: got %d; want 2", len(resp.Data.Checks))
	}
}

func TestCustomDashboardAPI_GetDataMasksCheckSecrets(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")

	checks := []CheckConfig{
		{
			ID:   "hb-1",
			Name: "Heartbeat",
			Type: "heartbeat",
			Heartbeat: &HeartbeatCheckConfig{
				Token:                   "heartbeat-token-secret",
				ExpectedIntervalSeconds: 60,
			},
			MySQL: &MySQLCheckConfig{Password: "mysql-secret"},
			SSH:   &SSHCheckConfig{Password: "ssh-secret"},
		},
	}
	fakeStore := &fakeStore{snapshot: State{Checks: checks}}
	handler := NewCustomDashboardAPIHandler(store, fakeStore, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	d, _ := store.Create(CustomDashboard{Name: "All Checks"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards/"+d.ID+"/data", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetData: status = %d; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, secret := range []string{"heartbeat-token-secret", "mysql-secret", "ssh-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("dashboard data leaked secret %q: %s", secret, body)
		}
	}
}

func TestCustomDashboardAPI_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	handler := NewCustomDashboardAPIHandler(store, &fakeStore{snapshot: State{}}, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboards", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestCustomDashboardAPI_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewCustomDashboardStore(dir + "/dashboards.json")
	handler := NewCustomDashboardAPIHandler(store, &fakeStore{snapshot: State{}}, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
}

func TestCustomDashboardAPI_BadFile(t *testing.T) {
	dir := t.TempDir()
	badFile := dir + "/bad.json"
	os.WriteFile(badFile, []byte("{invalid"), 0644)

	_, err := NewCustomDashboardStore(badFile)
	if err == nil {
		t.Error("expected error for invalid JSON file")
	}
}
