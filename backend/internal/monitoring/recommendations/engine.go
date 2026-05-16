package recommendations

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"time"

	"medics-health-check/backend/internal/monitoring"
)

// Thresholds for the engine analysis.
const (
	// A check is "stuck" if healthy but no results in this window.
	StuckThreshold = 5 * time.Minute

	// Minimum results needed to suggest threshold changes.
	MinResultsForThreshold = 20

	// If actual p95 is below this fraction of threshold, suggest tightening.
	TightenRatio = 0.5

	// If failures exceed this rate, suggest loosening.
	LooseFailureRate = 0.3

	// Minimum checks per server for adequate coverage.
	MinChecksPerServer = 2
)

// Engine analyzes telemetry data and generates recommendations.
type Engine struct {
	store        monitoring.Store
	incidentRepo monitoring.IncidentRepository
}

// NewEngine creates a recommendations engine.
func NewEngine(store monitoring.Store, incidentRepo monitoring.IncidentRepository) *Engine {
	return &Engine{store: store, incidentRepo: incidentRepo}
}

// Generate produces all recommendations from current state.
func (e *Engine) Generate() []Recommendation {
	state := e.store.Snapshot()
	incidents, _ := e.incidentRepo.ListIncidents()

	var recs []Recommendation
	recs = append(recs, e.thresholdSuggestions(state)...)
	recs = append(recs, e.coverageGaps(state)...)
	recs = append(recs, e.stuckDetection(state, incidents)...)

	// Sort by priority: high > medium > low
	sort.Slice(recs, func(i, j int) bool {
		return priorityWeight(recs[i].Priority) > priorityWeight(recs[j].Priority)
	})

	return recs
}

// thresholdSuggestions analyzes check results and suggests threshold adjustments.
func (e *Engine) thresholdSuggestions(state monitoring.State) []Recommendation {
	var recs []Recommendation

	// Index results by check ID
	resultsByCheck := make(map[string][]monitoring.CheckResult)
	for _, r := range state.Results {
		resultsByCheck[r.CheckID] = append(resultsByCheck[r.CheckID], r)
	}

	for _, check := range state.Checks {
		if check.Enabled != nil && !*check.Enabled {
			continue
		}
		if check.WarningThresholdMs <= 0 {
			continue
		}

		results := resultsByCheck[check.ID]
		if len(results) < MinResultsForThreshold {
			continue
		}

		// Compute p95 latency
		durations := make([]int64, 0, len(results))
		var failures int
		for _, r := range results {
			if r.DurationMs > 0 {
				durations = append(durations, r.DurationMs)
			}
			if !r.Healthy {
				failures++
			}
		}

		if len(durations) == 0 {
			continue
		}

		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p95 := percentile(durations, 0.95)
		p50 := percentile(durations, 0.50)
		threshold := int64(check.WarningThresholdMs)
		failureRate := float64(failures) / float64(len(results))

		// Suggest tightening if p95 is well below threshold
		if p95 < int64(float64(threshold)*TightenRatio) && failureRate < 0.05 {
			suggested := int64(math.Ceil(float64(p95) * 1.5))
			if suggested < p50*2 {
				suggested = p50 * 2
			}
			recs = append(recs, Recommendation{
				ID:       recID("threshold-tighten", check.ID),
				Category: CategoryThreshold,
				Priority: PriorityLow,
				Title:    fmt.Sprintf("Tighten threshold for %q", check.Name),
				Description: fmt.Sprintf(
					"Current threshold is %dms but p95 latency is only %dms. Consider lowering to %dms for earlier anomaly detection.",
					threshold, p95, suggested,
				),
				CheckID: check.ID,
				Server:  check.Server,
				Current: map[string]any{
					"warningThresholdMs": threshold,
					"p95Ms":              p95,
					"p50Ms":              p50,
				},
				Suggested: map[string]any{
					"warningThresholdMs": suggested,
				},
				Reason:    "p95 latency is significantly below current threshold",
				CreatedAt: time.Now(),
			})
		}

		// Suggest loosening if high failure rate due to threshold
		if failureRate > LooseFailureRate && p95 > threshold {
			suggested := int64(math.Ceil(float64(p95) * 1.2))
			recs = append(recs, Recommendation{
				ID:       recID("threshold-loosen", check.ID),
				Category: CategoryThreshold,
				Priority: PriorityMedium,
				Title:    fmt.Sprintf("Loosen threshold for %q", check.Name),
				Description: fmt.Sprintf(
					"Failure rate is %.0f%% with p95 at %dms (threshold %dms). Consider raising to %dms to reduce noise.",
					failureRate*100, p95, threshold, suggested,
				),
				CheckID: check.ID,
				Server:  check.Server,
				Current: map[string]any{
					"warningThresholdMs": threshold,
					"failureRate":        fmt.Sprintf("%.1f%%", failureRate*100),
					"p95Ms":              p95,
				},
				Suggested: map[string]any{
					"warningThresholdMs": suggested,
				},
				Reason:    "high failure rate suggests threshold is too aggressive",
				CreatedAt: time.Now(),
			})
		}
	}

	return recs
}

// coverageGaps identifies servers or apps lacking sufficient monitoring.
func (e *Engine) coverageGaps(state monitoring.State) []Recommendation {
	var recs []Recommendation

	// Count enabled checks per server
	serverChecks := make(map[string][]string) // server → check types
	for _, check := range state.Checks {
		if check.Enabled != nil && !*check.Enabled {
			continue
		}
		if check.Server == "" {
			continue
		}
		serverChecks[check.Server] = append(serverChecks[check.Server], check.Type)
	}

	for server, types := range serverChecks {
		if len(types) < MinChecksPerServer {
			recs = append(recs, Recommendation{
				ID:       recID("coverage-low", server),
				Category: CategoryCoverage,
				Priority: PriorityMedium,
				Title:    fmt.Sprintf("Low monitoring coverage for %q", server),
				Description: fmt.Sprintf(
					"Server %q only has %d check(s). Consider adding more check types for comprehensive monitoring.",
					server, len(types),
				),
				Server: server,
				Current: map[string]any{
					"checkCount": len(types),
					"types":      types,
				},
				Reason:    "insufficient check diversity increases blind spots",
				CreatedAt: time.Now(),
			})
		}

		// Suggest log check if missing
		hasLog := false
		for _, t := range types {
			if t == "log" {
				hasLog = true
				break
			}
		}
		if !hasLog && len(types) >= 2 {
			recs = append(recs, Recommendation{
				ID:       recID("coverage-no-log", server),
				Category: CategoryCoverage,
				Priority: PriorityLow,
				Title:    fmt.Sprintf("No log monitoring for %q", server),
				Description: fmt.Sprintf(
					"Server %q has %d checks but no log freshness check. Log monitoring helps detect silent failures.",
					server, len(types),
				),
				Server: server,
				Suggested: map[string]any{
					"addCheckType": "log",
				},
				Reason:    "log checks detect silent failures other check types miss",
				CreatedAt: time.Now(),
			})
		}
	}

	return recs
}

// stuckDetection identifies checks that are healthy but appear stalled.
func (e *Engine) stuckDetection(state monitoring.State, incidents []monitoring.Incident) []Recommendation {
	var recs []Recommendation
	now := time.Now()

	// Index latest result per check
	latestResult := make(map[string]monitoring.CheckResult)
	for _, r := range state.Results {
		if existing, ok := latestResult[r.CheckID]; !ok || r.FinishedAt.After(existing.FinishedAt) {
			latestResult[r.CheckID] = r
		}
	}

	// Find open incidents per check
	openIncidents := make(map[string]bool)
	for _, inc := range incidents {
		if inc.Status == "open" {
			openIncidents[inc.CheckID] = true
		}
	}

	for _, check := range state.Checks {
		if check.Enabled != nil && !*check.Enabled {
			continue
		}

		latest, hasResult := latestResult[check.ID]
		if !hasResult {
			continue
		}

		// Skip if there's already an open incident
		if openIncidents[check.ID] {
			continue
		}

		// Detect "up but stuck": healthy status but no recent execution
		staleness := now.Sub(latest.FinishedAt)
		expectedInterval := time.Duration(check.IntervalSeconds) * time.Second
		if expectedInterval <= 0 {
			expectedInterval = 60 * time.Second
		}

		// Stuck if no result in >5x the expected interval
		stuckMultiplier := 5 * expectedInterval
		if stuckMultiplier < StuckThreshold {
			stuckMultiplier = StuckThreshold
		}

		if latest.Healthy && staleness > stuckMultiplier {
			recs = append(recs, Recommendation{
				ID:       recID("stuck", check.ID),
				Category: CategoryStuck,
				Priority: PriorityHigh,
				Title:    fmt.Sprintf("%q appears up but stuck", check.Name),
				Description: fmt.Sprintf(
					"Last result was %s ago (expected every %ds). The check reported healthy but hasn't run since. Possible scheduler issue or stalled service.",
					formatDuration(staleness), check.IntervalSeconds,
				),
				CheckID: check.ID,
				Server:  check.Server,
				Current: map[string]any{
					"lastResultAge":    staleness.String(),
					"lastStatus":       latest.Status,
					"expectedInterval": fmt.Sprintf("%ds", check.IntervalSeconds),
				},
				Reason:    "check is healthy but hasn't executed in an unexpectedly long time",
				CreatedAt: now,
			})
		}
	}

	return recs
}

// --- Helpers ---

func recID(prefix, key string) string {
	h := sha256.Sum256([]byte(prefix + ":" + key))
	return fmt.Sprintf("rec_%s_%x", prefix, h[:6])
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func priorityWeight(p Priority) int {
	switch p {
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}
