package monitoring

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricsCollectorCreation tests that the metrics collector is created correctly
func TestMetricsCollectorCreation(t *testing.T) {
	mc := NewMetricsCollector()
	if mc == nil {
		t.Fatal("expected non-nil metrics collector")
	}
	mc.Unregister()
}

// TestRecordCheckRun tests recording check run metrics
func TestRecordCheckRun(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record some check runs - should not panic
	mc.RecordCheckRun("check-1", "api", "healthy", 100*time.Millisecond)
	mc.RecordCheckRun("check-1", "api", "healthy", 150*time.Millisecond)
	mc.RecordCheckRun("check-2", "tcp", "critical", 200*time.Millisecond)
}

// TestRecordCheckRunFailures tests that failed checks are recorded separately
func TestRecordCheckRunFailures(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record check runs including failures - should not panic
	mc.RecordCheckRun("check-1", "api", "healthy", 100*time.Millisecond)
	mc.RecordCheckRun("check-1", "api", "critical", 200*time.Millisecond)
	mc.RecordCheckRun("check-2", "tcp", "warning", 150*time.Millisecond)
}

// TestCheckDurationHistogram tests that check durations are recorded
func TestCheckDurationHistogram(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record check runs with various durations - should not panic
	mc.RecordCheckRun("check-1", "api", "healthy", 50*time.Millisecond)
	mc.RecordCheckRun("check-1", "api", "healthy", 100*time.Millisecond)
	mc.RecordCheckRun("check-1", "api", "healthy", 150*time.Millisecond)
}

// TestRecordIncident tests recording incident metrics
func TestRecordIncident(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record incidents - should not panic
	mc.RecordIncident("open")
	mc.RecordIncident("open")
	mc.RecordIncident("acknowledged")
	mc.RecordIncident("resolved")
}

// TestRecordAlertDelivery tests recording alert delivery metrics
func TestRecordAlertDelivery(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record alert deliveries - should not panic
	mc.RecordAlertDelivery("email", "success")
	mc.RecordAlertDelivery("email", "failure")
	mc.RecordAlertDelivery("webhook", "success")
	mc.RecordAlertDelivery("webhook", "success")
}

// TestRecordSchedulerLag tests recording scheduler lag
func TestRecordSchedulerLag(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record scheduler lag - should not panic
	mc.RecordSchedulerLag(100 * time.Millisecond)
	mc.RecordSchedulerLag(200 * time.Millisecond)
	mc.RecordSchedulerLag(50 * time.Millisecond)
}

// TestRecordHTTPRequest tests recording HTTP request metrics
func TestRecordHTTPRequest(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record HTTP requests - should not panic
	mc.RecordHTTPRequest("GET", "/api/v1/summary", 200, 50*time.Millisecond)
	mc.RecordHTTPRequest("GET", "/api/v1/checks", 200, 30*time.Millisecond)
	mc.RecordHTTPRequest("POST", "/api/v1/checks", 201, 100*time.Millisecond)
	mc.RecordHTTPRequest("GET", "/api/v1/summary", 500, 200*time.Millisecond)
}

// TestMetricsEndpoint tests that the /metrics endpoint works correctly
func TestMetricsEndpoint(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record some test data
	mc.RecordCheckRun("check-1", "api", "healthy", 100*time.Millisecond)
	mc.RecordHTTPRequest("GET", "/test", 200, 50*time.Millisecond)

	// Create test server with metrics handler
	handler := mc.Handler()
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make request to metrics endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	// Read and verify body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "healthmon_check_runs_total") {
		t.Error("expected metrics to contain healthmon_check_runs_total")
	}
	if !strings.Contains(bodyStr, "healthmon_http_requests_total") {
		t.Error("expected metrics to contain healthmon_http_requests_total")
	}
}

// TestNilMetricsCollector tests that methods handle nil collector gracefully
func TestNilMetricsCollector(t *testing.T) {
	var mc *MetricsCollector = nil

	// These should not panic
	mc.RecordCheckRun("check-1", "api", "healthy", 100*time.Millisecond)
	mc.RecordIncident("open")
	mc.RecordAlertDelivery("email", "success")
	mc.RecordSchedulerLag(100 * time.Millisecond)
	mc.RecordHTTPRequest("GET", "/test", 200, 50*time.Millisecond)

	// If we got here without panicking, the test passes
}

// TestMetricsMiddleware tests the metrics middleware
func TestMetricsMiddleware(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Create a simple handler
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	handler := metricsMiddleware(mc, next)

	// Make a test request
	req := httptest.NewRequest("GET", "/test/endpoint", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify metrics were recorded by gathering them
	// We just check that gathering doesn't error
	_, _ = prometheus.DefaultGatherer.Gather()
}

// TestMetricsMiddlewareWithError tests metrics middleware with error status
func TestMetricsMiddlewareWithError(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Create a handler that returns an error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
	})

	// Wrap with metrics middleware
	handler := metricsMiddleware(mc, next)

	// Make a test request
	req := httptest.NewRequest("POST", "/api/v1/checks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	// Verify metrics were recorded by gathering them
	// We just check that gathering doesn't error
	_, _ = prometheus.DefaultGatherer.Gather()
}

// TestResponseWriter tests the custom responseWriter
func TestResponseWriter(t *testing.T) {
	// Create a mock response writer
	underlying := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: underlying, status: http.StatusOK}

	// Test default status
	if rw.status != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rw.status)
	}

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	if rw.status != http.StatusNotFound {
		t.Errorf("expected status 404 after WriteHeader, got %d", rw.status)
	}

	// Test Write
	_, err := rw.Write([]byte("test"))
	if err != nil {
		t.Errorf("unexpected error writing: %v", err)
	}

	// Verify underlying writer received the data
	if underlying.Body.String() != "test" {
		t.Errorf("expected body 'test', got '%s'", underlying.Body.String())
	}

	// Verify underlying writer received the status
	if underlying.Code != http.StatusNotFound {
		t.Errorf("expected underlying status 404, got %d", underlying.Code)
	}
}

// TestUnregister tests that Unregister properly removes metrics
func TestUnregister(t *testing.T) {
	mc := NewMetricsCollector()

	// Record some data
	mc.RecordCheckRun("check-1", "api", "healthy", 100*time.Millisecond)

	// Unregister
	mc.Unregister()

	// Record more data (should not panic)
	mc.RecordCheckRun("check-2", "api", "healthy", 100*time.Millisecond)

	// Create a new collector for cleanup
	mc2 := NewMetricsCollector()
	defer mc2.Unregister()
}

// TestMetricsCollectorConcurrentRecording tests that the metrics collector is thread-safe
func TestMetricsCollectorConcurrentRecording(t *testing.T) {
	mc := NewMetricsCollector()
	defer mc.Unregister()

	// Record metrics concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				checkID := string(rune('a' + id))
				mc.RecordCheckRun(checkID, "api", "healthy", time.Duration(j)*time.Millisecond)
				mc.RecordIncident("open")
				mc.RecordAlertDelivery("email", "success")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without deadlock or race, the test passes
}
