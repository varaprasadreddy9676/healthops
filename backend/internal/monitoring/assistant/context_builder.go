package assistant

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"medics-health-check/backend/internal/monitoring"
)

const (
	// DefaultIncidentLookback is how far back the assistant looks for incidents.
	DefaultIncidentLookback = 48 * time.Hour
	// MaxChecksInContext caps how many checks are sent to the AI.
	MaxChecksInContext = 50
	// MaxIncidentsInContext caps how many incidents are sent to the AI.
	MaxIncidentsInContext = 30
	// MaxHistoryMessages caps conversation history sent to the AI.
	MaxHistoryMessages = 10
)

// ContextBuilder gathers telemetry context for the AI assistant.
type ContextBuilder struct {
	store        monitoring.Store
	incidentRepo monitoring.IncidentRepository
}

// NewContextBuilder creates a telemetry context builder.
func NewContextBuilder(store monitoring.Store, incidentRepo monitoring.IncidentRepository) *ContextBuilder {
	return &ContextBuilder{
		store:        store,
		incidentRepo: incidentRepo,
	}
}

// Build gathers recent telemetry into a compact context string for the AI.
// lookback controls how far back to search for incidents (0 = default).
func (cb *ContextBuilder) Build(lookback time.Duration) (string, []Reference, error) {
	if lookback <= 0 {
		lookback = DefaultIncidentLookback
	}
	ctx := cb.gatherContext(lookback)
	refs := cb.extractReferences(ctx)

	data, err := json.Marshal(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("marshal context: %w", err)
	}
	return string(data), refs, nil
}

func (cb *ContextBuilder) gatherContext(lookback time.Duration) TelemetryContext {
	tc := TelemetryContext{}

	state := cb.store.Snapshot()

	// Index results by checkID — keep only latest per check
	latestResult := make(map[string]monitoring.CheckResult)
	recentFailures := make(map[string]int)
	for _, r := range state.Results {
		existing, ok := latestResult[r.CheckID]
		if !ok || r.FinishedAt.After(existing.FinishedAt) {
			latestResult[r.CheckID] = r
		}
		if r.Status != "healthy" {
			recentFailures[r.CheckID]++
		}
	}

	// Build server map and check summaries
	serverMap := make(map[string]*ServerSummary)

	for _, check := range state.Checks {
		cs := CheckSummary{
			ID:     check.ID,
			Name:   check.Name,
			Type:   check.Type,
			Server: check.Server,
		}

		if result, ok := latestResult[check.ID]; ok {
			cs.Status = result.Status
			cs.Latency = result.DurationMs
			cs.LastRun = result.FinishedAt.Format(time.RFC3339)
			if result.Status != "healthy" {
				cs.Message = result.Message
			}
			cs.Failures = recentFailures[check.ID]
		} else {
			cs.Status = "unknown"
		}

		tc.Checks = append(tc.Checks, cs)

		// Aggregate into server summary
		srv, ok := serverMap[check.Server]
		if !ok {
			srv = &ServerSummary{Name: check.Server}
			serverMap[check.Server] = srv
		}
		srv.TotalChecks++
		if cs.Status == "healthy" {
			srv.Healthy++
		} else if cs.Status != "unknown" {
			srv.Unhealthy++
		}
		if cs.Latency > srv.WorstLatency {
			srv.WorstLatency = cs.Latency
		}
	}

	for _, srv := range serverMap {
		tc.Servers = append(tc.Servers, *srv)
	}

	// Gather recent incidents
	if cb.incidentRepo != nil {
		incidents, _ := cb.incidentRepo.ListIncidents()
		cutoff := time.Now().Add(-lookback)

		for _, inc := range incidents {
			if inc.StartedAt.Before(cutoff) {
				continue
			}

			is := IncidentSummary{
				ID:        inc.ID,
				CheckID:   inc.CheckID,
				Status:    inc.Status,
				Severity:  inc.Severity,
				CreatedAt: inc.StartedAt.Format(time.RFC3339),
				Message:   inc.Message,
			}

			// Resolve check name + server
			is.CheckName = inc.CheckName
			for _, check := range state.Checks {
				if check.ID == inc.CheckID {
					is.Server = check.Server
					if is.CheckName == "" {
						is.CheckName = check.Name
					}
					break
				}
			}

			tc.Incidents = append(tc.Incidents, is)
		}
	}

	// Cap sizes to keep context manageable
	if len(tc.Checks) > MaxChecksInContext {
		tc.Checks = tc.Checks[:MaxChecksInContext]
	}
	if len(tc.Incidents) > MaxIncidentsInContext {
		tc.Incidents = tc.Incidents[:MaxIncidentsInContext]
	}

	// Prioritize unhealthy checks first
	sortChecks(tc.Checks)

	return tc
}

func sortChecks(checks []CheckSummary) {
	// Simple bubble sort — unhealthy first, then by failures desc
	for i := range checks {
		for j := i + 1; j < len(checks); j++ {
			if checkPriority(checks[j]) > checkPriority(checks[i]) {
				checks[i], checks[j] = checks[j], checks[i]
			}
		}
	}
}

func checkPriority(c CheckSummary) int {
	score := c.Failures * 10
	if c.Status == "critical" {
		score += 100
	} else if c.Status == "warning" || c.Status == "degraded" {
		score += 50
	} else if c.Status == "unhealthy" {
		score += 80
	}
	return score
}

func (cb *ContextBuilder) extractReferences(ctx TelemetryContext) []Reference {
	var refs []Reference

	// Reference unhealthy checks and recent incidents
	for _, c := range ctx.Checks {
		if c.Status != "healthy" && c.Status != "unknown" {
			refs = append(refs, Reference{
				Type: "check",
				ID:   c.ID,
				Name: c.Name,
			})
		}
	}
	for _, inc := range ctx.Incidents {
		name := inc.CheckName
		if name == "" {
			name = inc.CheckID
		}
		refs = append(refs, Reference{
			Type: "incident",
			ID:   inc.ID,
			Name: fmt.Sprintf("%s (%s)", name, inc.Severity),
		})
	}

	// Deduplicate servers with issues
	seen := make(map[string]bool)
	for _, srv := range ctx.Servers {
		if srv.Unhealthy > 0 && !seen[srv.Name] {
			refs = append(refs, Reference{
				Type: "server",
				ID:   srv.Name,
				Name: srv.Name,
			})
			seen[srv.Name] = true
		}
	}

	return refs
}

// BuildPrompt creates the system and user messages for the AI call.
func (cb *ContextBuilder) BuildPrompt(question string, history []Message, telemetryJSON string) (systemMsg, userMsg string) {
	var sb strings.Builder

	sb.WriteString("You are HealthOps Assistant, an AI operations assistant embedded in a monitoring system.\n\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("- Answer ONLY from the telemetry data provided below. Never invent data.\n")
	sb.WriteString("- If the answer is not in the data, say so clearly.\n")
	sb.WriteString("- Be concise. Use bullet points for multi-item answers.\n")
	sb.WriteString("- When referencing checks or incidents, mention their names/IDs.\n")
	sb.WriteString("- For 'why' questions, correlate signals: timing, co-occurrence, cascading failures.\n")
	sb.WriteString("- Format your response in Markdown.\n")
	sb.WriteString("- Never reveal raw JSON. Summarize the data in natural language.\n\n")
	sb.WriteString("CURRENT SYSTEM TELEMETRY:\n```json\n")
	sb.WriteString(telemetryJSON)
	sb.WriteString("\n```\n")

	systemMsg = sb.String()

	// Build user message with optional conversation history
	var ub strings.Builder
	if len(history) > 0 {
		ub.WriteString("Previous conversation:\n")
		for _, msg := range history {
			ub.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
		}
		ub.WriteString("\n")
	}
	ub.WriteString("Question: ")
	ub.WriteString(question)

	userMsg = ub.String()
	return
}
