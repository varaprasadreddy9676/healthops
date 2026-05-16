package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// AIProvider is an interface that the log categorizer uses to call AI.
type AIProvider interface {
	Complete(ctx context.Context, systemMsg, userMsg string) (string, error)
}

// Categorizer performs AI-assisted labeling of error families.
type Categorizer struct {
	mu       sync.Mutex
	repo     Repository
	provider AIProvider
	logger   *log.Logger
}

// NewCategorizer creates a log categorizer.
func NewCategorizer(repo Repository, provider AIProvider, logger *log.Logger) *Categorizer {
	return &Categorizer{
		repo:     repo,
		provider: provider,
		logger:   logger,
	}
}

const categorizerSystemPrompt = `You are an expert SRE categorizing error log patterns.
Given error message patterns, stack traces, and sample messages, assign each error family:
1. A category from this list: db_auth, timeout, thread_exhaustion, slow_query, network, app_bug, memory, config, permission, disk_io, unknown
2. A short AI summary (1-2 sentences explaining what this error means)
3. A severity assessment: critical, warning, or info

Respond in JSON format:
{
  "category": "<category>",
  "summary": "<1-2 sentence explanation>",
  "severity": "<critical|warning|info>"
}`

const categorizerUserTemplate = `Categorize this error family:

**Pattern**: %s
**Source**: %s
**Occurrence Count**: %d
**First Seen**: %s
**Last Seen**: %s
**Sample Messages**:
%s

Respond with JSON only.`

// CategorizeFamilies runs AI categorization on unlabeled families.
func (c *Categorizer) CategorizeFamilies(ctx context.Context, limit int) (int, error) {
	if c.provider == nil {
		return 0, fmt.Errorf("no AI provider configured")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	families, err := c.repo.ListFamilies("active", 100)
	if err != nil {
		return 0, fmt.Errorf("list families: %w", err)
	}

	categorized := 0
	for _, family := range families {
		if categorized >= limit {
			break
		}
		// Skip already categorized
		if family.Category != "" && family.Category != CategoryUnknown {
			continue
		}
		// Skip families with very few occurrences (noise)
		if family.OccurrenceCount < 2 {
			continue
		}

		if err := c.categorizeFamily(ctx, &family); err != nil {
			c.logger.Printf("logs/categorizer: failed to categorize family %s: %v", family.ID, err)
			continue
		}
		categorized++
	}

	return categorized, nil
}

// CategorizeFamily categorizes a single family.
func (c *Categorizer) CategorizeFamily(ctx context.Context, familyID string) error {
	if c.provider == nil {
		return fmt.Errorf("no AI provider configured")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	family, err := c.repo.GetFamily(familyID)
	if err != nil {
		return err
	}
	return c.categorizeFamily(ctx, family)
}

func (c *Categorizer) categorizeFamily(ctx context.Context, family *ErrorFamily) error {
	samples := strings.Join(family.SampleMessages, "\n- ")
	if samples == "" {
		samples = "(no samples)"
	}

	userMsg := fmt.Sprintf(categorizerUserTemplate,
		family.Pattern,
		family.Source,
		family.OccurrenceCount,
		family.FirstSeenAt.Format(time.RFC3339),
		family.LastSeenAt.Format(time.RFC3339),
		"- "+samples,
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := c.provider.Complete(ctx, categorizerSystemPrompt, userMsg)
	if err != nil {
		return fmt.Errorf("AI call failed: %w", err)
	}

	// Parse response
	var result struct {
		Category string `json:"category"`
		Summary  string `json:"summary"`
		Severity string `json:"severity"`
	}

	// Extract JSON from response (might be wrapped in markdown)
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return fmt.Errorf("parse AI response: %w", err)
	}

	// Validate category
	if !isValidCategory(result.Category) {
		result.Category = CategoryUnknown
	}

	family.Category = result.Category
	family.AISummary = result.Summary
	family.AILabel = result.Category
	if result.Severity != "" {
		family.Severity = result.Severity
	}

	return c.repo.UpdateFamily(*family)
}

func isValidCategory(cat string) bool {
	for _, c := range AllCategories() {
		if c == cat {
			return true
		}
	}
	return false
}

func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") {
		return text
	}
	// Try to extract from markdown code block
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		// Skip potential language tag on same line
		if nl := strings.Index(text[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	// Try to find JSON object
	if idx := strings.Index(text, "{"); idx != -1 {
		// Find matching closing brace
		depth := 0
		for i := idx; i < len(text); i++ {
			if text[i] == '{' {
				depth++
			} else if text[i] == '}' {
				depth--
				if depth == 0 {
					return text[idx : i+1]
				}
			}
		}
	}
	return text
}
