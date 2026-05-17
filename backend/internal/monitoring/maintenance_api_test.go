package monitoring

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func newMaintenanceTestMux(t *testing.T) (*MaintenanceAPIHandler, *http.ServeMux) {
	t.Helper()
	store := newTempMaintenanceStore(t)
	handler := NewMaintenanceAPIHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestMaintenanceAPICreateAndList(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	now := time.Now().UTC()
	payload := map[string]interface{}{
		"name":      "Deploy v2",
		"startTime": now.Add(-10 * time.Minute).Format(time.RFC3339),
		"endTime":   now.Add(50 * time.Minute).Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	// Create
	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d; want 201. Body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("expected ok=true")
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/v1/maintenance", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d; want 200", rec.Code)
	}

	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != 1 {
		t.Errorf("total = %.0f; want 1", total)
	}
}

func TestMaintenanceAPICreateMissingName(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	now := time.Now().UTC()
	payload := map[string]interface{}{
		"startTime": now.Format(time.RFC3339),
		"endTime":   now.Add(1 * time.Hour).Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
}

func TestMaintenanceAPICreateMissingTimes(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	payload := map[string]interface{}{
		"name": "Missing Times",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
}

func TestMaintenanceAPICreateEndBeforeStart(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	now := time.Now().UTC()
	payload := map[string]interface{}{
		"name":      "Bad Range",
		"startTime": now.Add(1 * time.Hour).Format(time.RFC3339),
		"endTime":   now.Format(time.RFC3339), // before start
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
}

func TestMaintenanceAPICreateInvalidJSON(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/maintenance", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
}

func TestMaintenanceAPIGetAndUpdate(t *testing.T) {
	store := newTempMaintenanceStore(t)
	handler := NewMaintenanceAPIHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	now := time.Now().UTC()
	store.Create(MaintenanceWindow{
		ID:        "mw-get",
		Name:      "Original",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})

	// Get
	req := httptest.NewRequest(http.MethodGet, "/api/v1/maintenance/mw-get", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d; want 200", rec.Code)
	}

	var got MaintenanceWindow
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Name != "Original" {
		t.Errorf("Name = %q; want 'Original'", got.Name)
	}

	// Update
	updatePayload := map[string]interface{}{
		"name":    "Updated",
		"enabled": true,
	}
	body, _ := json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/maintenance/mw-get", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d; want 200. Body: %s", rec.Code, rec.Body.String())
	}

	// Verify update
	req = httptest.NewRequest(http.MethodGet, "/api/v1/maintenance/mw-get", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Name != "Updated" {
		t.Errorf("after update Name = %q; want 'Updated'", got.Name)
	}
}

func TestMaintenanceAPIUpdateRejectsInvalidFinalRange(t *testing.T) {
	store := newTempMaintenanceStore(t)
	handler := NewMaintenanceAPIHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	start := time.Now().UTC()
	end := start.Add(time.Hour)
	if err := store.Create(MaintenanceWindow{
		ID:        "mw-range",
		Name:      "Range",
		StartTime: start,
		EndTime:   end,
		Enabled:   true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"endTime": start.Add(-time.Minute).Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/maintenance/mw-range", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d; want 400. Body: %s", rec.Code, rec.Body.String())
	}

	got, err := store.Get("mw-range")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.EndTime.Equal(end) {
		t.Fatalf("invalid update was persisted: endTime = %s; want %s", got.EndTime, end)
	}
}

func TestMaintenanceAPIGetNotFound(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/maintenance/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
}

func TestMaintenanceAPIUpdateNotFound(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	body, _ := json.Marshal(map[string]string{"name": "nope"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/maintenance/nonexistent", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
}

func TestMaintenanceAPIDelete(t *testing.T) {
	store := newTempMaintenanceStore(t)
	handler := NewMaintenanceAPIHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	now := time.Now().UTC()
	store.Create(MaintenanceWindow{
		ID:        "mw-del",
		Name:      "Delete Me",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/maintenance/mw-del", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d; want 200", rec.Code)
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/maintenance/mw-del", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("after delete status = %d; want 404", rec.Code)
	}
}

func TestMaintenanceAPIDeleteNotFound(t *testing.T) {
	_, mux := newMaintenanceTestMux(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/maintenance/nonexistent", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
}

func TestMaintenanceAPIListActive(t *testing.T) {
	store := newTempMaintenanceStore(t)
	handler := NewMaintenanceAPIHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	now := time.Now().UTC()

	// Active
	store.Create(MaintenanceWindow{
		ID:        "active",
		Name:      "Active",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})
	// Future (not active)
	store.Create(MaintenanceWindow{
		ID:        "future",
		Name:      "Future",
		StartTime: now.Add(1 * time.Hour),
		EndTime:   now.Add(2 * time.Hour),
		Enabled:   true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/maintenance/active", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != 1 {
		t.Errorf("active total = %.0f; want 1", total)
	}
}

func TestMaintenanceAPIUpdateInvalidJSON(t *testing.T) {
	store := newTempMaintenanceStore(t)
	handler := NewMaintenanceAPIHandler(store)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	now := time.Now().UTC()
	store.Create(MaintenanceWindow{
		ID:        "mw-bad-json",
		Name:      "Test",
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now.Add(1 * time.Hour),
		Enabled:   true,
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/maintenance/mw-bad-json", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", rec.Code)
	}
}

// Silence unused import for filepath (used by newTempMaintenanceStore helper).
var _ = filepath.Join
