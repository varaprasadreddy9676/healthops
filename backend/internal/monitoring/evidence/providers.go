package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"health-ops/backend/internal/monitoring"
)

// --- Check Results Provider ---

// CheckProvider collects recent check results as SignalEvents.
type CheckProvider struct {
	store monitoring.Store
}

func NewCheckProvider(store monitoring.Store) *CheckProvider {
	return &CheckProvider{store: store}
}

func (p *CheckProvider) Category() string { return "checks" }

func (p *CheckProvider) Collect(_ context.Context, incidentID string, window TimeWindow) ([]SignalEvent, error) {
	state := p.store.Snapshot()

	// Find the check associated with this incident by scanning results
	var events []SignalEvent
	for _, r := range state.Results {
		if r.FinishedAt.Before(window.Start) || r.FinishedAt.After(window.End) {
			continue
		}
		sev := "info"
		if r.Status == "critical" || r.Status == "error" {
			sev = "critical"
		} else if r.Status == "warning" {
			sev = "warning"
		}

		// Find the check config for this result
		var checkName, checkType, server, app string
		for _, c := range state.Checks {
			if c.ID == r.CheckID {
				checkName = c.Name
				checkType = c.Type
				server = c.Server
				app = c.Application
				break
			}
		}

		msg := fmt.Sprintf("%s check '%s': %s", checkType, checkName, r.Status)
		if r.Message != "" {
			msg += " — " + r.Message
		}

		attrs := map[string]string{
			"checkId":   r.CheckID,
			"checkName": checkName,
			"checkType": checkType,
			"status":    r.Status,
		}
		if r.DurationMs > 0 {
			attrs["latencyMs"] = fmt.Sprintf("%d", r.DurationMs)
		}

		events = append(events, SignalEvent{
			ID:              signalID("check", r.CheckID, r.FinishedAt),
			TenantID:        "default",
			Type:            SignalTypeCheck,
			Timestamp:       r.FinishedAt,
			Severity:        sev,
			Service:         app,
			Environment:     "",
			Host:            server,
			Source:          "check_runner",
			Message:         msg,
			Attributes:      attrs,
			RedactionStatus: "clean",
		})
	}
	return events, nil
}

// --- MySQL Provider ---

// MySQLProvider collects MySQL samples and deltas as SignalEvents.
type MySQLProvider struct {
	repo monitoring.MySQLMetricsRepository
}

func NewMySQLProvider(repo monitoring.MySQLMetricsRepository) *MySQLProvider {
	return &MySQLProvider{repo: repo}
}

func (p *MySQLProvider) Category() string { return "mysql" }

func (p *MySQLProvider) Collect(_ context.Context, incidentID string, window TimeWindow) ([]SignalEvent, error) {
	if p.repo == nil {
		return nil, nil
	}

	// We need to find the check ID for this incident. Since we only have
	// incidentID, we gather recent samples across all checks. The context
	// builder will correlate by time window.
	// For now, get the latest sample and recent deltas from all checks by
	// querying with empty checkID (which won't work with the existing repo).
	// Instead, we use the snapshot repo which stores incident-specific evidence.
	return nil, nil // Handled by MySQLSnapshotProvider below
}

// MySQLSnapshotProvider uses the existing IncidentSnapshotRepository to
// collect pre-captured MySQL evidence.
type MySQLSnapshotProvider struct {
	repo monitoring.IncidentSnapshotRepository
}

func NewMySQLSnapshotProvider(repo monitoring.IncidentSnapshotRepository) *MySQLSnapshotProvider {
	return &MySQLSnapshotProvider{repo: repo}
}

func (p *MySQLSnapshotProvider) Category() string { return "mysql" }

func (p *MySQLSnapshotProvider) Collect(_ context.Context, incidentID string, _ TimeWindow) ([]SignalEvent, error) {
	if p.repo == nil {
		return nil, nil
	}

	snapshots, err := p.repo.GetSnapshots(incidentID)
	if err != nil {
		return nil, fmt.Errorf("get mysql snapshots: %w", err)
	}

	var events []SignalEvent
	for _, snap := range snapshots {
		// Parse payload to get summary info
		var payloadSummary string
		switch snap.SnapshotType {
		case "latest_sample":
			payloadSummary = summarizeMySQLSample(snap.PayloadJSON)
		case "recent_deltas":
			payloadSummary = "MySQL rate-of-change metrics"
		case "processlist":
			payloadSummary = "Active MySQL processes"
		case "statement_analysis":
			payloadSummary = "Top queries by execution time"
		default:
			payloadSummary = snap.SnapshotType
		}

		events = append(events, SignalEvent{
			ID:         signalID("mysql_snap", incidentID, snap.Timestamp),
			TenantID:   "default",
			Type:       SignalTypeMySQL,
			Timestamp:  snap.Timestamp,
			Severity:   "info",
			Source:     "mysql_collector",
			Message:    payloadSummary,
			IncidentID: incidentID,
			Attributes: map[string]string{
				"snapshotType": snap.SnapshotType,
				"payload":      truncate(snap.PayloadJSON, 2000),
			},
			RedactionStatus: "clean",
		})
	}
	return events, nil
}

// --- Server Metrics Provider ---

// ServerMetricsProvider collects recent server metric snapshots.
type ServerMetricsProvider struct {
	repo  monitoring.ServerMetricsStore
	store monitoring.Store
}

func NewServerMetricsProvider(repo monitoring.ServerMetricsStore, store monitoring.Store) *ServerMetricsProvider {
	return &ServerMetricsProvider{repo: repo, store: store}
}

func (p *ServerMetricsProvider) Category() string { return "server_metrics" }

func (p *ServerMetricsProvider) Collect(_ context.Context, incidentID string, window TimeWindow) ([]SignalEvent, error) {
	if p.repo == nil {
		return nil, nil
	}

	// Collect server metrics for all known servers in the window
	state := p.store.Snapshot()
	servers := make(map[string]bool)
	for _, c := range state.Checks {
		if c.Server != "" {
			servers[c.Server] = true
		}
	}

	var events []SignalEvent
	for serverID := range servers {
		snapshots, err := p.repo.GetSnapshots(serverID, window.Start, window.End)
		if err != nil {
			continue
		}
		for _, snap := range snapshots {
			sev := "info"
			if snap.CPUUsagePercent > 90 || snap.MemoryUsagePercent > 90 || snap.DiskUsagePercent > 90 {
				sev = "warning"
			}

			msg := fmt.Sprintf("Server %s: CPU %.1f%%, Mem %.1f%%, Disk %.1f%%",
				serverID, snap.CPUUsagePercent, snap.MemoryUsagePercent, snap.DiskUsagePercent)

			attrs := map[string]string{
				"serverId":    serverID,
				"cpuPercent":  fmt.Sprintf("%.1f", snap.CPUUsagePercent),
				"memPercent":  fmt.Sprintf("%.1f", snap.MemoryUsagePercent),
				"diskPercent": fmt.Sprintf("%.1f", snap.DiskUsagePercent),
				"loadAvg1":    fmt.Sprintf("%.2f", snap.LoadAvg1),
				"loadAvg5":    fmt.Sprintf("%.2f", snap.LoadAvg5),
			}

			events = append(events, SignalEvent{
				ID:              signalID("server", serverID, snap.Timestamp),
				TenantID:        "default",
				Type:            SignalTypeServer,
				Timestamp:       snap.Timestamp,
				Severity:        sev,
				Host:            serverID,
				Source:          "ssh_collector",
				Message:         msg,
				Attributes:      attrs,
				RedactionStatus: "clean",
			})
		}
	}
	return events, nil
}

// --- Audit Provider ---

// AuditProvider collects recent audit events related to the incident.
type AuditProvider struct {
	repo monitoring.AuditRepository
}

func NewAuditProvider(repo monitoring.AuditRepository) *AuditProvider {
	return &AuditProvider{repo: repo}
}

func (p *AuditProvider) Category() string { return "audit" }

func (p *AuditProvider) Collect(_ context.Context, incidentID string, window TimeWindow) ([]SignalEvent, error) {
	if p.repo == nil {
		return nil, nil
	}

	auditEvents, err := p.repo.ListEvents(monitoring.AuditFilter{
		StartTime: window.Start,
		EndTime:   window.End,
		Limit:     50,
	})
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}

	var events []SignalEvent
	for _, ae := range auditEvents {
		detailsJSON, _ := json.Marshal(ae.Details)
		events = append(events, SignalEvent{
			ID:        signalID("audit", ae.ID, ae.Timestamp),
			TenantID:  "default",
			Type:      SignalTypeAudit,
			Timestamp: ae.Timestamp,
			Severity:  "info",
			Source:    "audit_log",
			Message:   fmt.Sprintf("[%s] %s on %s/%s by %s", ae.Action, ae.Action, ae.Target, ae.TargetID, ae.Actor),
			Attributes: map[string]string{
				"action":   ae.Action,
				"actor":    ae.Actor,
				"target":   ae.Target,
				"targetId": ae.TargetID,
				"details":  truncate(string(detailsJSON), 500),
			},
			RedactionStatus: "clean",
		})
	}
	return events, nil
}

// --- Incident History Provider ---

// IncidentHistoryProvider collects recent incidents for pattern matching.
type IncidentHistoryProvider struct {
	repo monitoring.IncidentRepository
}

func NewIncidentHistoryProvider(repo monitoring.IncidentRepository) *IncidentHistoryProvider {
	return &IncidentHistoryProvider{repo: repo}
}

func (p *IncidentHistoryProvider) Category() string { return "incident_history" }

func (p *IncidentHistoryProvider) Collect(_ context.Context, incidentID string, window TimeWindow) ([]SignalEvent, error) {
	if p.repo == nil {
		return nil, nil
	}

	incidents, err := p.repo.ListIncidents()
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}

	var events []SignalEvent
	for _, inc := range incidents {
		// Skip the current incident
		if inc.ID == incidentID {
			continue
		}
		// Only include incidents from the time window (expanded to 7 days for history)
		historyStart := window.Start.Add(-7 * 24 * time.Hour)
		startedAt := time.Time{}
		if !inc.StartedAt.IsZero() {
			startedAt = inc.StartedAt
		}
		if startedAt.Before(historyStart) || startedAt.After(window.End) {
			continue
		}

		msg := fmt.Sprintf("Previous incident [%s] on check '%s': %s — %s",
			inc.Status, inc.CheckName, inc.Severity, inc.Message)

		attrs := map[string]string{
			"incidentId": inc.ID,
			"checkId":    inc.CheckID,
			"checkName":  inc.CheckName,
			"status":     inc.Status,
			"severity":   inc.Severity,
		}
		if inc.ResolvedAt != nil {
			duration := inc.ResolvedAt.Sub(startedAt)
			attrs["durationMin"] = fmt.Sprintf("%.1f", duration.Minutes())
		}

		events = append(events, SignalEvent{
			ID:              signalID("incident", inc.ID, startedAt),
			TenantID:        "default",
			Type:            SignalTypeCheck, // incidents originate from checks
			Timestamp:       startedAt,
			Severity:        inc.Severity,
			Source:          "incident_manager",
			Message:         msg,
			Attributes:      attrs,
			RedactionStatus: "clean",
		})
	}
	return events, nil
}

// --- Helpers ---

func signalID(prefix string, key string, ts time.Time) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", prefix, key, ts.UnixNano())))
	return fmt.Sprintf("sig_%x", h[:12])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func summarizeMySQLSample(payloadJSON string) string {
	var sample struct {
		Connections        int64   `json:"connections"`
		MaxConnections     int64   `json:"maxConnections"`
		MaxUsedConnections int64   `json:"maxUsedConnections"`
		ThreadsRunning     int64   `json:"threadsRunning"`
		ThreadsConnected   int64   `json:"threadsConnected"`
		SlowQueries        int64   `json:"slowQueries"`
		ConnectionUtil     float64 `json:"connectionUtilization"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &sample); err != nil {
		return "MySQL sample (parse error)"
	}
	return fmt.Sprintf("MySQL: %d/%d connections (%.1f%% util), %d threads running, %d slow queries",
		sample.ThreadsConnected, sample.MaxConnections, sample.ConnectionUtil*100,
		sample.ThreadsRunning, sample.SlowQueries)
}
