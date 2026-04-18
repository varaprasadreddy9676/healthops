package monitoring

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector collects and exposes Prometheus metrics for the health check service
type MetricsCollector struct {
	registry *prometheus.Registry

	// Check execution metrics
	checkRunsTotal     *prometheus.CounterVec
	checkFailuresTotal *prometheus.CounterVec
	checkDuration      *prometheus.HistogramVec

	// Incident metrics
	incidentsTotal *prometheus.CounterVec

	// Alert metrics
	alertDeliveriesTotal *prometheus.CounterVec

	// Scheduler metrics
	schedulerLag *prometheus.GaugeVec

	// HTTP request metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
}

// NewMetricsCollector creates a new metrics collector with all metrics registered
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		registry: prometheus.NewRegistry(),

		// Check runs counter: status can be "healthy", "warning", "critical", "error"
		checkRunsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "healthmon_check_runs_total",
				Help: "Total number of health check executions, labeled by status and check type",
			},
			[]string{"status", "type"},
		),

		// Check failures counter: labeled by check_id and check type
		checkFailuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "healthmon_check_failures_total",
				Help: "Total number of health check failures, labeled by check_id and check type",
			},
			[]string{"check_id", "type"},
		),

		// Check duration histogram: labeled by check_id
		checkDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "healthmon_check_duration_seconds",
				Help:    "Duration of health check execution in seconds, labeled by check_id",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"check_id"},
		),

		// Incidents counter: status can be "open", "acknowledged", "resolved"
		incidentsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "healthmon_incidents_total",
				Help: "Total number of incidents created, labeled by status",
			},
			[]string{"status"},
		),

		// Alert deliveries counter: channel can be "email", "webhook", etc; outcome can be "success", "failure"
		alertDeliveriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "healthmon_alert_deliveries_total",
				Help: "Total number of alert delivery attempts, labeled by channel and outcome",
			},
			[]string{"channel", "outcome"},
		),

		// Scheduler lag gauge: measures delay between expected and actual execution time
		schedulerLag: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "healthmon_scheduler_lag_seconds",
				Help: "Lag between scheduled check time and actual execution time in seconds",
			},
			[]string{}, // No labels for global lag
		),

		// HTTP requests counter: labeled by method, endpoint, and status code
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "healthmon_http_requests_total",
				Help: "Total number of HTTP requests, labeled by method, endpoint, and status code",
			},
			[]string{"method", "endpoint", "status"},
		),

		// HTTP request duration histogram: labeled by endpoint
		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "healthmon_http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds, labeled by endpoint",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"endpoint"},
		),
	}

	// Register metrics on an instance-local registry so tests/services can create
	// independent collectors without duplicate global registration panics.
	mc.registry.MustRegister(
		mc.checkRunsTotal,
		mc.checkFailuresTotal,
		mc.checkDuration,
		mc.incidentsTotal,
		mc.alertDeliveriesTotal,
		mc.schedulerLag,
		mc.httpRequestsTotal,
		mc.httpRequestDuration,
	)

	return mc
}

// RecordCheckRun records metrics after a check execution
func (mc *MetricsCollector) RecordCheckRun(checkID, checkType, status string, duration time.Duration) {
	if mc == nil {
		return
	}

	durationSeconds := duration.Seconds()
	mc.checkRunsTotal.WithLabelValues(status, checkType).Inc()
	mc.checkDuration.WithLabelValues(checkID).Observe(durationSeconds)

	// If status is not healthy, also record as failure
	if status != "healthy" {
		mc.checkFailuresTotal.WithLabelValues(checkID, checkType).Inc()
	}
}

// RecordIncident records incident creation or state changes
func (mc *MetricsCollector) RecordIncident(status string) {
	if mc == nil {
		return
	}
	mc.incidentsTotal.WithLabelValues(status).Inc()
}

// RecordAlertDelivery records alert delivery attempts
func (mc *MetricsCollector) RecordAlertDelivery(channel, outcome string) {
	if mc == nil {
		return
	}
	mc.alertDeliveriesTotal.WithLabelValues(channel, outcome).Inc()
}

// RecordSchedulerLag records the delay between scheduled and actual execution
func (mc *MetricsCollector) RecordSchedulerLag(lag time.Duration) {
	if mc == nil {
		return
	}
	mc.schedulerLag.WithLabelValues().Set(lag.Seconds())
}

// RecordHTTPRequest records HTTP request metrics
func (mc *MetricsCollector) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	if mc == nil {
		return
	}

	statusStr := strconv.Itoa(statusCode)
	durationSeconds := duration.Seconds()

	mc.httpRequestsTotal.WithLabelValues(method, endpoint, statusStr).Inc()
	mc.httpRequestDuration.WithLabelValues(endpoint).Observe(durationSeconds)
}

// Handler returns the Prometheus metrics HTTP handler
func (mc *MetricsCollector) Handler() http.Handler {
	if mc == nil || mc.registry == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(mc.registry, promhttp.HandlerOpts{})
}

// Unregister unregisters all metrics from the Prometheus registry
// This is primarily useful for tests
func (mc *MetricsCollector) Unregister() {
	if mc == nil || mc.registry == nil {
		return
	}
	mc.registry.Unregister(mc.checkRunsTotal)
	mc.registry.Unregister(mc.checkFailuresTotal)
	mc.registry.Unregister(mc.checkDuration)
	mc.registry.Unregister(mc.incidentsTotal)
	mc.registry.Unregister(mc.alertDeliveriesTotal)
	mc.registry.Unregister(mc.schedulerLag)
	mc.registry.Unregister(mc.httpRequestsTotal)
	mc.registry.Unregister(mc.httpRequestDuration)
}
