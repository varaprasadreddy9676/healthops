package automation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"health-ops/backend/internal/monitoring"
)

// AIProvider calls the configured AI model.
type AIProvider func(ctx context.Context, systemMsg, userMsg string) (string, error)

// ActionExpiry is how long an action stays pending before auto-expiring.
const ActionExpiry = 24 * time.Hour

// Engine generates and manages automation actions.
type Engine struct {
	store        monitoring.Store
	incidentRepo monitoring.IncidentRepository
	aiCall       AIProvider
	logger       *log.Logger
	mu           sync.RWMutex
	actions      []Action
	audit        []AuditEntry
}

// NewEngine creates an automation engine.
func NewEngine(store monitoring.Store, incidentRepo monitoring.IncidentRepository, aiCall AIProvider, logger *log.Logger) *Engine {
	if logger == nil {
		logger = log.Default()
	}
	return &Engine{
		store:        store,
		incidentRepo: incidentRepo,
		aiCall:       aiCall,
		logger:       logger,
		actions:      make([]Action, 0),
		audit:        make([]AuditEntry, 0),
	}
}

// Suggest asks AI to propose remediation actions for a given context.
func (e *Engine) Suggest(ctx context.Context, req SuggestRequest) ([]Action, error) {
	if e.aiCall == nil {
		return nil, fmt.Errorf("AI provider not configured")
	}

	// Build context for AI
	contextData := e.buildContext(req)

	systemMsg := `You are an infrastructure reliability engineer. Given the monitoring context, suggest specific remediation actions.
For each action, respond in JSON array format:
[{"type":"restart|drain_node|rotate_credential|inspect_queries|scale_up|clear_queue|custom","title":"short title","description":"what this does","risk":"low|medium|high|critical","command":"optional shell command","reason":"why this helps"}]
Rules:
- Only suggest actions that are safe and reversible when possible
- Always include risk level honestly
- Never suggest destructive actions without noting the risk as "critical"
- Maximum 3 suggestions per request
- Commands should be specific and executable`

	userMsg := fmt.Sprintf("Monitoring context:\n%s\n\nAdditional context: %s\n\nSuggest remediation actions.", contextData, req.Context)

	response, err := e.aiCall(ctx, systemMsg, userMsg)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	actions := e.parseAIResponse(response, req)
	if actions == nil {
		actions = []Action{}
	}

	// Store actions and audit. Reuse matching pending suggestions so repeated
	// generate clicks do not flood the operator queue with duplicates.
	e.mu.Lock()
	e.expirePendingLocked()
	existingPending := make(map[string]Action)
	for _, action := range e.actions {
		if action.Status == StatusPending {
			existingPending[actionDedupKey(action)] = action
		}
	}
	storedActions := make([]Action, 0, len(actions))
	for i := range actions {
		if existing, ok := existingPending[actionDedupKey(actions[i])]; ok {
			storedActions = append(storedActions, existing)
			continue
		}
		e.actions = append(e.actions, actions[i])
		existingPending[actionDedupKey(actions[i])] = actions[i]
		storedActions = append(storedActions, actions[i])
		e.audit = append(e.audit, AuditEntry{
			ID:        auditID(),
			ActionID:  actions[i].ID,
			Actor:     "ai",
			Event:     "suggested",
			Details:   actions[i].Reason,
			Timestamp: time.Now(),
		})
	}
	e.mu.Unlock()

	e.logger.Printf("[automation] AI suggested %d actions, %d queued", len(actions), len(storedActions))
	return storedActions, nil
}

// ListActions returns all actions, optionally filtered by status.
func (e *Engine) ListActions(status string) []Action {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.expirePendingLocked()

	if status == "" {
		result := make([]Action, len(e.actions))
		copy(result, e.actions)
		return result
	}

	var filtered []Action
	for _, a := range e.actions {
		if string(a.Status) == status {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// GetAction returns a single action by ID.
func (e *Engine) GetAction(id string) (Action, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, a := range e.actions {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}

// Approve marks an action as approved (human-in-the-loop).
func (e *Engine) Approve(id string, actor string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.actions {
		if e.actions[i].ID == id {
			if e.actions[i].Status != StatusPending {
				return fmt.Errorf("action %s is not pending (status: %s)", id, e.actions[i].Status)
			}
			now := time.Now()
			e.actions[i].Status = StatusApproved
			e.actions[i].ApprovedBy = actor
			e.actions[i].ApprovedAt = &now
			e.actions[i].Result = "approved in audit log; command not executed"

			e.audit = append(e.audit, AuditEntry{
				ID:        auditID(),
				ActionID:  id,
				Actor:     actor,
				Event:     "approved",
				Timestamp: now,
			})

			e.logger.Printf("[automation] action %s approved by %s", id, actor)
			return nil
		}
	}
	return fmt.Errorf("action %s not found", id)
}

// Reject marks an action as rejected.
func (e *Engine) Reject(id string, actor string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.actions {
		if e.actions[i].ID == id {
			if e.actions[i].Status != StatusPending {
				return fmt.Errorf("action %s is not pending (status: %s)", id, e.actions[i].Status)
			}
			now := time.Now()
			e.actions[i].Status = StatusRejected
			e.actions[i].RejectedBy = actor
			e.actions[i].RejectedAt = &now

			e.audit = append(e.audit, AuditEntry{
				ID:        auditID(),
				ActionID:  id,
				Actor:     actor,
				Event:     "rejected",
				Details:   reason,
				Timestamp: now,
			})

			e.logger.Printf("[automation] action %s rejected by %s: %s", id, actor, reason)
			return nil
		}
	}
	return fmt.Errorf("action %s not found", id)
}

// AuditLog returns the full audit trail.
func (e *Engine) AuditLog() []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]AuditEntry, len(e.audit))
	copy(result, e.audit)
	return result
}

// buildContext gathers relevant monitoring data for AI.
func (e *Engine) buildContext(req SuggestRequest) string {
	var sb strings.Builder

	state := e.store.Snapshot()
	checkByID := make(map[string]monitoring.CheckConfig, len(state.Checks))
	for _, c := range state.Checks {
		checkByID[c.ID] = c
	}
	latestByCheck := make(map[string]monitoring.CheckResult)
	for _, r := range state.Results {
		if existing, ok := latestByCheck[r.CheckID]; !ok || r.FinishedAt.After(existing.FinishedAt) {
			latestByCheck[r.CheckID] = r
		}
	}

	// Include relevant check info
	if req.CheckID != "" {
		for _, c := range state.Checks {
			if c.ID == req.CheckID {
				sb.WriteString(fmt.Sprintf("Check: %s (type=%s, server=%s, target=%s)\n", c.Name, c.Type, c.Server, c.Target))
				break
			}
		}
		// Recent results for this check
		var recent []monitoring.CheckResult
		for _, r := range state.Results {
			if r.CheckID == req.CheckID {
				recent = append(recent, r)
			}
		}
		if len(recent) > 5 {
			recent = recent[len(recent)-5:]
		}
		for _, r := range recent {
			sb.WriteString(fmt.Sprintf("  Result: status=%s healthy=%v duration=%dms at=%s msg=%s\n",
				r.Status, r.Healthy, r.DurationMs, r.FinishedAt.Format(time.RFC3339), r.Message))
		}
	} else {
		var unhealthy []monitoring.CheckResult
		for _, r := range latestByCheck {
			if !r.Healthy {
				unhealthy = append(unhealthy, r)
			}
		}
		sort.Slice(unhealthy, func(i, j int) bool {
			return unhealthy[i].FinishedAt.After(unhealthy[j].FinishedAt)
		})
		if len(unhealthy) > 0 {
			sb.WriteString("Current unhealthy checks:\n")
			for i, r := range unhealthy {
				if i >= 5 {
					break
				}
				check := checkByID[r.CheckID]
				name := r.Name
				if name == "" {
					name = check.Name
				}
				checkType := r.Type
				if checkType == "" {
					checkType = check.Type
				}
				server := r.Server
				if server == "" {
					server = check.Server
				}
				sb.WriteString(fmt.Sprintf("  - %s (type=%s, server=%s, target=%s): status=%s duration=%dms at=%s msg=%s\n",
					name, checkType, server, check.Target, r.Status, r.DurationMs, r.FinishedAt.Format(time.RFC3339), r.Message))
			}
		}
	}

	// Include incident info
	if req.IncidentID != "" && e.incidentRepo != nil {
		inc, err := e.incidentRepo.GetIncident(req.IncidentID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("\nIncident: %s (severity=%s, status=%s, started=%s)\n",
				inc.CheckName, inc.Severity, inc.Status, inc.StartedAt.Format(time.RFC3339)))
			sb.WriteString(fmt.Sprintf("  Message: %s\n", inc.Message))
		}
	} else if e.incidentRepo != nil {
		incidents, err := e.incidentRepo.ListIncidents()
		if err == nil {
			var open []monitoring.Incident
			for _, inc := range incidents {
				if inc.Status != "resolved" {
					open = append(open, inc)
				}
			}
			sort.Slice(open, func(i, j int) bool {
				return open[i].UpdatedAt.After(open[j].UpdatedAt)
			})
			if len(open) > 0 {
				sb.WriteString("\nOpen incidents:\n")
				for i, inc := range open {
					if i >= 5 {
						break
					}
					sb.WriteString(fmt.Sprintf("  - %s (severity=%s, status=%s, updated=%s)\n",
						inc.CheckName, inc.Severity, inc.Status, inc.UpdatedAt.Format(time.RFC3339)))
					sb.WriteString(fmt.Sprintf("    Message: %s\n", inc.Message))
				}
			}
		}
	}

	// General system summary
	var healthyCount, unhealthyCount int
	for _, r := range latestByCheck {
		if r.Healthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}
	sb.WriteString(fmt.Sprintf("\nSystem: %d checks total, %d healthy, %d unhealthy\n", len(state.Checks), healthyCount, unhealthyCount))

	return sb.String()
}

// parseAIResponse converts the AI response into Action structs.
func (e *Engine) parseAIResponse(response string, req SuggestRequest) []Action {
	// Extract JSON from response (might have markdown wrapping)
	jsonStr := response
	if idx := strings.Index(response, "["); idx >= 0 {
		if end := strings.LastIndex(response, "]"); end > idx {
			jsonStr = response[idx : end+1]
		}
	}

	type aiAction struct {
		Type        string `json:"type"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Risk        string `json:"risk"`
		Command     string `json:"command"`
		Reason      string `json:"reason"`
	}

	var parsed []aiAction
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		e.logger.Printf("[automation] failed to parse AI response: %v", err)
		return nil
	}

	now := time.Now()
	var actions []Action
	for _, p := range parsed {
		if len(actions) >= 3 {
			break
		}
		actions = append(actions, Action{
			ID:          actionID(p.Type, p.Title),
			Type:        ActionType(p.Type),
			Title:       p.Title,
			Description: p.Description,
			Risk:        RiskLevel(p.Risk),
			CheckID:     req.CheckID,
			Server:      "",
			IncidentID:  req.IncidentID,
			Command:     p.Command,
			Reason:      p.Reason,
			Status:      StatusPending,
			CreatedAt:   now,
			ExpiresAt:   now.Add(ActionExpiry),
		})
	}
	return actions
}

// expirePendingLocked expires actions past their expiry time. Must be called with at least RLock held.
func (e *Engine) expirePendingLocked() {
	now := time.Now()
	for i := range e.actions {
		if e.actions[i].Status == StatusPending && now.After(e.actions[i].ExpiresAt) {
			e.actions[i].Status = StatusExpired
		}
	}
}

func actionID(actionType, title string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", actionType, title, time.Now().UnixNano())))
	return fmt.Sprintf("act_%x", h[:8])
}

func actionDedupKey(action Action) string {
	return string(action.Type) + "\x00" + strings.ToLower(strings.TrimSpace(action.Title))
}

func auditID() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("audit:%d", time.Now().UnixNano())))
	return fmt.Sprintf("aud_%x", h[:8])
}
