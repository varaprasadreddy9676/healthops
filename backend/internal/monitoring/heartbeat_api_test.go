package monitoring

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newHeartbeatTestMux(t *testing.T) (*HeartbeatAPIHandler, *http.ServeMux) {
	t.Helper()
	handler := NewHeartbeatAPIHandler()
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return handler, mux
}

func TestHeartbeatAPIPingGET(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	// Register a token
	token := GenerateHeartbeatToken()
	handler.store.Register(token, "check-ping-get")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeats/"+token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200. Body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Error("expected ok=true")
	}

	// Verify ping was recorded
	state, ok := handler.store.GetState(token)
	if !ok {
		t.Fatal("expected state for token")
	}
	if state.PingCount != 1 {
		t.Errorf("PingCount = %d; want 1", state.PingCount)
	}
}

func TestHeartbeatAPIPingPOSTWithBody(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	token := GenerateHeartbeatToken()
	handler.store.Register(token, "check-ping-post")

	payload := map[string]string{
		"status":  "fail",
		"message": "backup failed",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeats/"+token, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	state, _ := handler.store.GetState(token)
	if state.LastPing == nil {
		t.Fatal("expected LastPing to be set")
	}
	if state.LastPing.Status != "fail" {
		t.Errorf("Status = %q; want 'fail'", state.LastPing.Status)
	}
	if state.LastPing.Message != "backup failed" {
		t.Errorf("Message = %q; want 'backup failed'", state.LastPing.Message)
	}
}

func TestHeartbeatAPIPingWithQueryParams(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	token := GenerateHeartbeatToken()
	handler.store.Register(token, "check-ping-query")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeats/"+token+"?status=warn&msg=slow+backup", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	state, _ := handler.store.GetState(token)
	if state.LastPing.Status != "warn" {
		t.Errorf("Status = %q; want 'warn'", state.LastPing.Status)
	}
	if state.LastPing.Message != "slow backup" {
		t.Errorf("Message = %q; want 'slow backup'", state.LastPing.Message)
	}
}

func TestHeartbeatAPIPingHEAD(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	token := GenerateHeartbeatToken()
	handler.store.Register(token, "check-ping-head")

	req := httptest.NewRequest(http.MethodHead, "/api/v1/heartbeats/"+token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	state, _ := handler.store.GetState(token)
	if state.PingCount != 1 {
		t.Errorf("PingCount = %d; want 1 (HEAD should count as ping)", state.PingCount)
	}
}

func TestHeartbeatAPIPingUnknownToken(t *testing.T) {
	_, mux := newHeartbeatTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeats/unknown-token-xyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", rec.Code)
	}
}

func TestHeartbeatAPIList(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	token1 := GenerateHeartbeatToken()
	token2 := GenerateHeartbeatToken()
	handler.store.Register(token1, "check-list-1")
	handler.store.Register(token2, "check-list-2")

	// Ping one of them
	handler.store.RecordPing(HeartbeatPing{
		Token:    token1,
		PingedAt: time.Now().UTC(),
		Status:   "success",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total < 2 {
		t.Errorf("total = %.0f; want >= 2", total)
	}
}

func TestHeartbeatAPIMultiplePings(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	token := GenerateHeartbeatToken()
	handler.store.Register(token, "check-multi")

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeats/"+token, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("ping %d status = %d; want 200", i, rec.Code)
		}
	}

	state, _ := handler.store.GetState(token)
	if state.PingCount != 5 {
		t.Errorf("PingCount = %d; want 5", state.PingCount)
	}
}

func TestHeartbeatAPIIPExtraction(t *testing.T) {
	handler, mux := newHeartbeatTestMux(t)

	token := GenerateHeartbeatToken()
	handler.store.Register(token, "check-ip")

	// With X-Forwarded-For
	req := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeats/"+token, nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	state, _ := handler.store.GetState(token)
	if state.LastPing.IPAddress != "1.2.3.4" {
		t.Errorf("IPAddress = %q; want '1.2.3.4' (first XFF)", state.LastPing.IPAddress)
	}
}
