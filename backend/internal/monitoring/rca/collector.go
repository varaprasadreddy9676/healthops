package rca

import (
	"math"
	"sort"
	"time"
)

// Collector gathers multi-signal context for root-cause analysis.
type Collector struct {
	signals SignalSource
}

// NewCollector creates a new signal collector.
func NewCollector(signals SignalSource) *Collector {
	return &Collector{signals: signals}
}

// CollectContext gathers all available signals for an incident within a time window.
func (c *Collector) CollectContext(incident IncidentRef, window time.Duration) CorrelationContext {
	windowStart := incident.StartedAt.Add(-window)
	windowEnd := time.Now().UTC()
	if windowEnd.Before(incident.StartedAt.Add(window)) {
		windowEnd = incident.StartedAt.Add(window)
	}

	// Gather check results for the incident's check
	checkResults := c.signals.RecentResults(incident.ID, 100)

	// Gather all results in the window for cross-correlation
	allResults := c.signals.AllRecentResults(windowStart, 500)

	// Build signal series from check results
	signals := c.buildSignalSeries(checkResults, allResults, windowStart, windowEnd)

	// Build timeline events
	events := c.buildTimeline(checkResults, allResults, incident)

	ctx := CorrelationContext{
		IncidentID:   incident.ID,
		CheckName:    incident.CheckName,
		Severity:     incident.Severity,
		Message:      incident.Message,
		StartedAt:    incident.StartedAt,
		Duration:     time.Since(incident.StartedAt).Round(time.Second).String(),
		Signals:      signals,
		RecentEvents: events,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
	}

	return ctx
}

// buildSignalSeries extracts time-series from check results.
func (c *Collector) buildSignalSeries(primary, all []CheckResultRef, start, end time.Time) []SignalSeries {
	// Group metrics by name+source+server
	type seriesKey struct {
		name, source, server string
	}
	groups := make(map[seriesKey][]SignalPoint)

	for _, r := range primary {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		// Always track latency
		if r.DurationMs > 0 {
			key := seriesKey{name: "latencyMs", source: r.Name, server: r.Server}
			groups[key] = append(groups[key], SignalPoint{
				Timestamp: r.Timestamp, Name: "latencyMs", Value: float64(r.DurationMs),
				Source: r.Name, Server: r.Server,
			})
		}
		// Track all metrics
		for name, val := range r.Metrics {
			key := seriesKey{name: name, source: r.Name, server: r.Server}
			groups[key] = append(groups[key], SignalPoint{
				Timestamp: r.Timestamp, Name: name, Value: val,
				Source: r.Name, Server: r.Server,
			})
		}
	}

	// Also extract signals from cross-correlated checks
	for _, r := range all {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		if r.DurationMs > 0 {
			key := seriesKey{name: "latencyMs", source: r.Name, server: r.Server}
			groups[key] = append(groups[key], SignalPoint{
				Timestamp: r.Timestamp, Name: "latencyMs", Value: float64(r.DurationMs),
				Source: r.Name, Server: r.Server,
			})
		}
		for name, val := range r.Metrics {
			key := seriesKey{name: name, source: r.Name, server: r.Server}
			groups[key] = append(groups[key], SignalPoint{
				Timestamp: r.Timestamp, Name: name, Value: val,
				Source: r.Name, Server: r.Server,
			})
		}
	}

	// Convert to SignalSeries with computed stats
	var series []SignalSeries
	for key, points := range groups {
		if len(points) < 2 {
			continue
		}
		sort.Slice(points, func(i, j int) bool {
			return points[i].Timestamp.Before(points[j].Timestamp)
		})

		s := SignalSeries{
			Name:   key.name,
			Source: key.source,
			Server: key.server,
			Points: points,
		}
		s.Min, s.Max, s.Avg = computeStats(points)
		s.Trend = detectTrend(points)
		series = append(series, s)
	}

	// Sort by relevance: spikes and rising trends first
	sort.Slice(series, func(i, j int) bool {
		return trendPriority(series[i].Trend) < trendPriority(series[j].Trend)
	})

	// Limit to top 20 most relevant series
	if len(series) > 20 {
		series = series[:20]
	}

	return series
}

// buildTimeline constructs a chronological list of notable events.
func (c *Collector) buildTimeline(primary, all []CheckResultRef, incident IncidentRef) []TimelineEvent {
	var events []TimelineEvent

	// Add incident start event
	events = append(events, TimelineEvent{
		Timestamp:   incident.StartedAt,
		Type:        "incident_open",
		Description: incident.Message,
		Severity:    incident.Severity,
		Source:      incident.CheckName,
	})

	// Detect status transitions in check results
	var prevStatus string
	for _, r := range primary {
		if prevStatus != "" && r.Status != prevStatus {
			evType := "check_fail"
			if r.Status == "healthy" {
				evType = "check_recover"
			}
			events = append(events, TimelineEvent{
				Timestamp:   r.Timestamp,
				Type:        evType,
				Description: r.Name + ": " + prevStatus + " -> " + r.Status,
				Source:      r.Name,
			})
		}
		prevStatus = r.Status
	}

	// Detect anomalies in all results (latency spikes, failures)
	for _, r := range all {
		if r.Status == "critical" {
			events = append(events, TimelineEvent{
				Timestamp:   r.Timestamp,
				Type:        "check_fail",
				Description: r.Name + " critical: " + r.Status,
				Severity:    "critical",
				Source:      r.Name,
			})
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Limit timeline
	if len(events) > 50 {
		events = events[:50]
	}

	return events
}

func computeStats(points []SignalPoint) (min, max, avg float64) {
	if len(points) == 0 {
		return 0, 0, 0
	}
	min = math.MaxFloat64
	max = -math.MaxFloat64
	sum := 0.0
	for _, p := range points {
		if p.Value < min {
			min = p.Value
		}
		if p.Value > max {
			max = p.Value
		}
		sum += p.Value
	}
	avg = sum / float64(len(points))
	return
}

func detectTrend(points []SignalPoint) string {
	if len(points) < 3 {
		return "stable"
	}

	// Compare first third average to last third average
	third := len(points) / 3
	if third == 0 {
		third = 1
	}

	var firstSum, lastSum float64
	for i := 0; i < third; i++ {
		firstSum += points[i].Value
	}
	for i := len(points) - third; i < len(points); i++ {
		lastSum += points[i].Value
	}

	firstAvg := firstSum / float64(third)
	lastAvg := lastSum / float64(third)

	if firstAvg == 0 {
		if lastAvg > 0 {
			return "spike"
		}
		return "stable"
	}

	change := (lastAvg - firstAvg) / firstAvg

	// Check for spike (single outlier in recent points)
	_, maxVal, avgVal := computeStats(points)
	if avgVal > 0 && maxVal/avgVal > 3.0 {
		return "spike"
	}

	switch {
	case change > 0.5:
		return "rising"
	case change < -0.3:
		return "falling"
	default:
		return "stable"
	}
}

func trendPriority(trend string) int {
	switch trend {
	case "spike":
		return 0
	case "rising":
		return 1
	case "falling":
		return 2
	default:
		return 3
	}
}
