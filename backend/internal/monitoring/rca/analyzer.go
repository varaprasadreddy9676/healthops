package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"medics-health-check/backend/internal/util/jsonl"
)

// Analyzer performs AI-powered root cause analysis.
type Analyzer struct {
	mu        sync.RWMutex
	collector *Collector
	provider  AIProvider
	logger    *log.Logger
	dataDir   string
	reports   []RCAReport
}

// NewAnalyzer creates a new RCA analyzer.
func NewAnalyzer(collector *Collector, provider AIProvider, dataDir string, logger *log.Logger) (*Analyzer, error) {
	if logger == nil {
		logger = log.New(os.Stderr, "rca ", log.LstdFlags)
	}
	a := &Analyzer{
		collector: collector,
		provider:  provider,
		logger:    logger,
		dataDir:   dataDir,
	}

	// Load existing reports
	reports, err := jsonl.Load[RCAReport](a.reportsPath())
	if err != nil {
		return nil, fmt.Errorf("load rca reports: %w", err)
	}
	a.reports = reports

	return a, nil
}

func (a *Analyzer) reportsPath() string {
	return a.dataDir + "/rca_reports.jsonl"
}

// Analyze performs root-cause analysis for an incident.
func (a *Analyzer) Analyze(ctx context.Context, incident IncidentRef, logFamilies []ErrorFamilyRef) (*RCAReport, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("AI provider not configured")
	}

	// Collect multi-signal context (look back 15 minutes before incident)
	corrCtx := a.collector.CollectContext(incident, 15*time.Minute)
	corrCtx.ErrorFamilies = logFamilies

	report := RCAReport{
		ID:          fmt.Sprintf("rca-%s-%d", incident.ID, time.Now().UnixMilli()),
		IncidentID:  incident.ID,
		CreatedAt:   time.Now().UTC(),
		Status:      "pending",
		SignalCount: countSignals(corrCtx),
		WindowStart: corrCtx.WindowStart,
		WindowEnd:   corrCtx.WindowEnd,
		Timeline:    corrCtx.RecentEvents,
	}

	// Build AI prompt
	systemMsg := buildSystemPrompt()
	userMsg, err := buildUserPrompt(corrCtx)
	if err != nil {
		report.Status = "failed"
		report.Error = fmt.Sprintf("build prompt: %v", err)
		a.persist(report)
		return &report, err
	}

	// Call AI
	response, err := a.provider.Analyze(ctx, systemMsg, userMsg)
	if err != nil {
		report.Status = "failed"
		report.Error = fmt.Sprintf("AI call failed: %v", err)
		a.persist(report)
		return &report, err
	}

	// Parse response
	hypotheses, summary, err := parseRCAResponse(response)
	if err != nil {
		a.logger.Printf("RCA: failed to parse AI response, using raw: %v", err)
		report.Summary = response
		report.Status = "complete"
	} else {
		report.Hypotheses = hypotheses
		report.Summary = summary
		report.Status = "complete"
	}

	report.ProviderUsed = "default"
	a.persist(report)

	return &report, nil
}

// GetReport retrieves an RCA report by ID.
func (a *Analyzer) GetReport(id string) *RCAReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for i := range a.reports {
		if a.reports[i].ID == id {
			return &a.reports[i]
		}
	}
	return nil
}

// ReportsForIncident returns all RCA reports for an incident.
func (a *Analyzer) ReportsForIncident(incidentID string) []RCAReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []RCAReport
	for _, r := range a.reports {
		if r.IncidentID == incidentID {
			result = append(result, r)
		}
	}
	return result
}

// AllReports returns all RCA reports.
func (a *Analyzer) AllReports(limit int) []RCAReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.reports) {
		limit = len(a.reports)
	}

	// Return most recent first
	result := make([]RCAReport, limit)
	for i := 0; i < limit; i++ {
		result[i] = a.reports[len(a.reports)-1-i]
	}
	return result
}

func (a *Analyzer) persist(report RCAReport) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update existing or append
	found := false
	for i, r := range a.reports {
		if r.ID == report.ID {
			a.reports[i] = report
			found = true
			break
		}
	}
	if !found {
		a.reports = append(a.reports, report)
	}

	if err := jsonl.Rewrite(a.reportsPath(), a.reports); err != nil {
		a.logger.Printf("RCA: failed to persist report: %v", err)
	}
}

func countSignals(ctx CorrelationContext) int {
	count := 0
	for _, s := range ctx.Signals {
		count += len(s.Points)
	}
	return count
}

func buildSystemPrompt() string {
	return `You are an expert Site Reliability Engineer performing root-cause analysis on a production incident.

You will receive correlated multi-signal telemetry data from a monitoring system including:
- Time-series metrics (latency, error rates, resource utilization)
- Timeline of events (check failures, recoveries, status changes)
- Related log error families (clustered similar errors)
- Incident metadata (severity, duration, affected services)

Your task is to analyze all signals together and produce a ROOT CAUSE ANALYSIS with:
1. Multiple ranked hypotheses (not just one answer)
2. Confidence scores (0.0 to 1.0) based on evidence strength
3. Supporting evidence for each hypothesis
4. Actionable remediation suggestions

IMPORTANT:
- Rank by confidence, not severity
- Never claim 100% confidence without definitive proof
- Identify correlations across different signal types
- Note when signals contradict each other
- If evidence is insufficient, say so explicitly

Respond in JSON format:
{
  "summary": "1-2 sentence overview of the incident",
  "hypotheses": [
    {
      "rank": 1,
      "title": "Short hypothesis title",
      "description": "Detailed explanation of the hypothesis",
      "confidence": 0.85,
      "category": "resource|network|application|database|config",
      "evidence": ["signal1 shows X", "signal2 correlates with Y"],
      "suggestion": "Specific remediation step"
    }
  ]
}`
}

func buildUserPrompt(ctx CorrelationContext) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Incident\n"))
	sb.WriteString(fmt.Sprintf("- Check: %s\n", ctx.CheckName))
	sb.WriteString(fmt.Sprintf("- Severity: %s\n", ctx.Severity))
	sb.WriteString(fmt.Sprintf("- Message: %s\n", ctx.Message))
	sb.WriteString(fmt.Sprintf("- Started: %s\n", ctx.StartedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- Duration: %s\n", ctx.Duration))
	sb.WriteString(fmt.Sprintf("- Window: %s to %s\n\n", ctx.WindowStart.Format(time.RFC3339), ctx.WindowEnd.Format(time.RFC3339)))

	// Signals
	sb.WriteString("## Correlated Signals\n")
	for _, s := range ctx.Signals {
		sb.WriteString(fmt.Sprintf("\n### %s (source: %s", s.Name, s.Source))
		if s.Server != "" {
			sb.WriteString(fmt.Sprintf(", server: %s", s.Server))
		}
		sb.WriteString(fmt.Sprintf(")\n"))
		sb.WriteString(fmt.Sprintf("- Trend: %s\n", s.Trend))
		sb.WriteString(fmt.Sprintf("- Min: %.2f, Max: %.2f, Avg: %.2f\n", s.Min, s.Max, s.Avg))

		// Include last 10 data points for context
		points := s.Points
		if len(points) > 10 {
			points = points[len(points)-10:]
		}
		sb.WriteString("- Recent values: ")
		for i, p := range points {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%.1f@%s", p.Value, p.Timestamp.Format("15:04:05")))
		}
		sb.WriteString("\n")
	}

	// Error families
	if len(ctx.ErrorFamilies) > 0 {
		sb.WriteString("\n## Related Error Families\n")
		for _, f := range ctx.ErrorFamilies {
			sb.WriteString(fmt.Sprintf("- [%s] %s (%d occurrences, last: %s)\n",
				f.Category, f.Pattern, f.OccurrenceCount, f.LastSeenAt))
		}
	}

	// Timeline
	if len(ctx.RecentEvents) > 0 {
		sb.WriteString("\n## Event Timeline\n")
		for _, e := range ctx.RecentEvents {
			sb.WriteString(fmt.Sprintf("- %s [%s] %s", e.Timestamp.Format("15:04:05"), e.Type, e.Description))
			if e.Severity != "" {
				sb.WriteString(fmt.Sprintf(" (severity: %s)", e.Severity))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func parseRCAResponse(raw string) ([]RCAHypothesis, string, error) {
	// Extract JSON from response (handle markdown code blocks)
	jsonStr := extractJSON(raw)

	var result struct {
		Summary    string          `json:"summary"`
		Hypotheses []RCAHypothesis `json:"hypotheses"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, "", fmt.Errorf("parse RCA JSON: %w", err)
	}

	// Ensure ranks are set
	for i := range result.Hypotheses {
		if result.Hypotheses[i].Rank == 0 {
			result.Hypotheses[i].Rank = i + 1
		}
	}

	return result.Hypotheses, result.Summary, nil
}

func extractJSON(s string) string {
	// Try to find JSON block in markdown code fence
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		start := idx + 3
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Try to find raw JSON object
	if idx := strings.Index(s, "{"); idx >= 0 {
		if end := strings.LastIndex(s, "}"); end > idx {
			return s[idx : end+1]
		}
	}
	return s
}
