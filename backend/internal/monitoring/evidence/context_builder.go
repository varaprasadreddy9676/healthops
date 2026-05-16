package evidence

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// DefaultEvidenceCap is the maximum number of evidence items included in a
// single AI context. Over the cap, items are summarized by deterministic
// rollups.
const DefaultEvidenceCap = 50

// ContextBuilder retrieves evidence from all registered providers and
// assembles it into a structured context for the AI Incident Brief.
type ContextBuilder struct {
	registry    *Registry
	evidenceCap int
	logger      *log.Logger
}

// NewContextBuilder creates a context builder with the given registry.
func NewContextBuilder(registry *Registry, logger *log.Logger) *ContextBuilder {
	return &ContextBuilder{
		registry:    registry,
		evidenceCap: DefaultEvidenceCap,
		logger:      logger,
	}
}

// SetEvidenceCap overrides the default evidence cap.
func (b *ContextBuilder) SetEvidenceCap(cap int) {
	if cap > 0 {
		b.evidenceCap = cap
	}
}

// CollectedEvidence holds the output of the context builder.
type CollectedEvidence struct {
	// Events is the bounded set of SignalEvents, sorted by timestamp.
	Events []SignalEvent
	// ByCategory groups events by provider category.
	ByCategory map[string][]SignalEvent
	// AvailableCategories lists categories that returned evidence.
	AvailableCategories []string
	// MissingCategories lists categories that returned no evidence.
	MissingCategories []string
	// WasCapped indicates if the evidence cap was applied.
	WasCapped bool
	// TotalBeforeCap is the count before capping.
	TotalBeforeCap int
}

// Collect gathers evidence from all registered providers for an incident.
func (b *ContextBuilder) Collect(ctx context.Context, incidentID string, window TimeWindow) (*CollectedEvidence, error) {
	providers := b.registry.Providers()

	result := &CollectedEvidence{
		ByCategory: make(map[string][]SignalEvent),
	}

	for _, p := range providers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		events, err := p.Collect(ctx, incidentID, window)
		if err != nil {
			b.logger.Printf("WARNING: evidence provider %q failed: %v", p.Category(), err)
			continue
		}

		if len(events) > 0 {
			result.ByCategory[p.Category()] = events
			result.Events = append(result.Events, events...)
			result.AvailableCategories = append(result.AvailableCategories, p.Category())
		} else {
			result.MissingCategories = append(result.MissingCategories, p.Category())
		}
	}

	// Sort all events by timestamp
	sort.Slice(result.Events, func(i, j int) bool {
		return result.Events[i].Timestamp.Before(result.Events[j].Timestamp)
	})

	// Apply evidence cap
	result.TotalBeforeCap = len(result.Events)
	if len(result.Events) > b.evidenceCap {
		result.WasCapped = true
		result.Events = capEvents(result.Events, b.evidenceCap)
	}

	return result, nil
}

// FormatForPrompt renders the collected evidence as a structured text block
// suitable for inclusion in an AI prompt.
func (b *ContextBuilder) FormatForPrompt(evidence *CollectedEvidence) string {
	if evidence == nil || len(evidence.Events) == 0 {
		return "No evidence collected."
	}

	var sb strings.Builder

	sb.WriteString("=== EVIDENCE START ===\n")
	sb.WriteString(fmt.Sprintf("Total evidence items: %d", len(evidence.Events)))
	if evidence.WasCapped {
		sb.WriteString(fmt.Sprintf(" (capped from %d)", evidence.TotalBeforeCap))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Categories with evidence: %s\n", strings.Join(evidence.AvailableCategories, ", ")))
	if len(evidence.MissingCategories) > 0 {
		sb.WriteString(fmt.Sprintf("Categories without evidence: %s\n", strings.Join(evidence.MissingCategories, ", ")))
	}
	sb.WriteString("\n")

	// Group by category for structured output
	for _, cat := range evidence.AvailableCategories {
		events := evidence.ByCategory[cat]
		sb.WriteString(fmt.Sprintf("--- %s (%d items) ---\n", strings.ToUpper(cat), len(events)))
		for _, e := range events {
			sb.WriteString(fmt.Sprintf("[%s] [%s] %s\n",
				e.Timestamp.Format(time.RFC3339), e.Severity, e.Message))
			// Include key attributes
			for k, v := range e.Attributes {
				if k == "payload" {
					continue // Skip large payloads in the summary line
				}
				sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== EVIDENCE END ===\n")
	return sb.String()
}

// capEvents reduces the event list to the cap by keeping the most relevant
// events: all critical/warning events plus recent events to fill the cap.
func capEvents(events []SignalEvent, cap int) []SignalEvent {
	// Separate critical/warning from info
	var important, info []SignalEvent
	for _, e := range events {
		if e.Severity == "critical" || e.Severity == "warning" {
			important = append(important, e)
		} else {
			info = append(info, e)
		}
	}

	// Always keep important events
	if len(important) >= cap {
		return important[:cap]
	}

	// Fill remaining slots with most recent info events
	remaining := cap - len(important)
	if remaining > len(info) {
		remaining = len(info)
	}
	// Take the most recent info events
	recentInfo := info[len(info)-remaining:]

	result := make([]SignalEvent, 0, len(important)+len(recentInfo))
	result = append(result, important...)
	result = append(result, recentInfo...)

	// Re-sort by timestamp
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}
