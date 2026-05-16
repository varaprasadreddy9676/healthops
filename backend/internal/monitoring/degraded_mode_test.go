package monitoring

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsProtectedEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/v1/checks", want: true},
		{path: "/api/v1/checks/check-1", want: true},
		{path: "/api/v1/config", want: true},
		{path: "/api/v1/config/runtime", want: true},
		{path: "/api/v1/servers", want: true},
		{path: "/api/v1/notification-channels", want: true},
		{path: "/api/v1/summary", want: false},
		{path: "/healthz", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			if got := isProtectedEndpoint(tc.path); got != tc.want {
				t.Fatalf("isProtectedEndpoint(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestServiceSetDegradedHealthCheckBlocksProtectedWrites(t *testing.T) {
	store := &fakeStore{}
	service := NewService(&Config{}, store, nil)
	service.SetDegradedHealthCheck(func(context.Context) error {
		return errors.New("mongo down")
	})

	service.degradedMode.runHealthCheck(context.Background())

	if service.degradedMode == nil {
		t.Fatal("expected degraded mode to be configured")
	}
	if !service.degradedMode.IsDegraded() {
		t.Fatal("expected service to be degraded after failing health check")
	}

	called := false
	handler := service.degradedMode.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/checks/api-1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("protected write reached downstream handler while degraded")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestDegradedModeAutoResolvesDatabaseIncidentOnRecovery(t *testing.T) {
	t.Parallel()

	healthy := false
	incidentMgr := &fakeDegradedIncidentManager{}
	dm := NewDegradedMode(func(context.Context) error {
		if !healthy {
			return errors.New("mongo down")
		}
		return nil
	}, incidentMgr, nil)

	dm.runHealthCheck(context.Background())
	if !dm.IsDegraded() {
		t.Fatal("expected degraded mode after failed health check")
	}
	if incidentMgr.createdCheckID != "database" {
		t.Fatalf("created check ID = %q, want database", incidentMgr.createdCheckID)
	}

	healthy = true
	dm.runHealthCheck(context.Background())
	if dm.IsDegraded() {
		t.Fatal("expected degraded mode to clear after recovered health check")
	}
	if incidentMgr.resolvedCheckID != "database" {
		t.Fatalf("resolved check ID = %q, want database", incidentMgr.resolvedCheckID)
	}
}

type fakeDegradedIncidentManager struct {
	createdCheckID  string
	resolvedCheckID string
}

func (f *fakeDegradedIncidentManager) ProcessAlert(checkID, checkName, checkType, severity, message string, metadata map[string]string) error {
	f.createdCheckID = checkID
	return nil
}

func (f *fakeDegradedIncidentManager) AutoResolveOnRecovery(checkID string) error {
	f.resolvedCheckID = checkID
	return nil
}
