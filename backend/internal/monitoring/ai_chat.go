package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ChatProvider is the AI function used by the chat service.
type ChatProvider func(ctx context.Context, systemMsg, userMsg string) (string, error)

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"` // "user", "assistant", "system"
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // references, tokens, etc.
}

// ChatConversation is a persistent chat session.
type ChatConversation struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Owner     string        `json:"owner,omitempty"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Context   string        `json:"context,omitempty"` // "general", "incident:id", "check:id"
}

// ChatSuggestion is a quick-action suggestion.
type ChatSuggestion struct {
	Text     string `json:"text"`
	Category string `json:"category"` // "status", "debug", "explain", "action"
}

// ChatAskRequest is the request to send a message in the chat.
type ChatAskRequest struct {
	ConversationID string `json:"conversationId,omitempty"` // empty = new conversation
	Message        string `json:"message"`
	Context        string `json:"context,omitempty"` // e.g., "incident:inc-123"
}

// ChatAskResponse is the AI response.
type ChatAskResponse struct {
	ConversationID string           `json:"conversationId"`
	Message        ChatMessage      `json:"message"`
	Suggestions    []ChatSuggestion `json:"suggestions,omitempty"`
	References     []ChatReference  `json:"references,omitempty"`
	DurationMs     int64            `json:"durationMs"`
}

// ChatReference points to a related entity.
type ChatReference struct {
	Type string `json:"type"` // "check", "incident", "server", "metric"
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Compile-time check: ChatStore implements ChatRepository.
var _ ChatRepository = (*ChatStore)(nil)

// ChatStore manages persistent conversations.
type ChatStore struct {
	mu            sync.RWMutex
	conversations map[string]*ChatConversation
	filePath      string
	nextID        int
}

// NewChatStore creates or loads the chat store.
func NewChatStore(filePath string) (*ChatStore, error) {
	s := &ChatStore{
		conversations: make(map[string]*ChatConversation),
		filePath:      filePath,
		nextID:        1,
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load chat store: %w", err)
	}
	return s, nil
}

func (s *ChatStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var convos []ChatConversation
	if err := json.Unmarshal(data, &convos); err != nil {
		return err
	}
	for i := range convos {
		c := convos[i]
		s.conversations[c.ID] = &c
		s.nextID++
	}
	return nil
}

func (s *ChatStore) save() error {
	convos := make([]ChatConversation, 0, len(s.conversations))
	for _, c := range s.conversations {
		convos = append(convos, *c)
	}
	data, err := json.MarshalIndent(convos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// CreateConversation starts a new conversation.
func (s *ChatStore) CreateConversation(title, owner, ctx string) *ChatConversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	conv := &ChatConversation{
		ID:        fmt.Sprintf("chat-%d-%d", s.nextID, now.Unix()),
		Title:     title,
		Owner:     owner,
		Messages:  []ChatMessage{},
		CreatedAt: now,
		UpdatedAt: now,
		Context:   ctx,
	}
	s.nextID++
	s.conversations[conv.ID] = conv
	s.save()
	return conv
}

// GetConversation returns a conversation by ID.
func (s *ChatStore) GetConversation(id string) (*ChatConversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.conversations[id]
	if !ok {
		return nil, fmt.Errorf("conversation %q not found", id)
	}
	copy := *c
	return &copy, nil
}

// ListConversations returns all conversations for an owner.
func (s *ChatStore) ListConversations(owner string) []ChatConversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []ChatConversation
	for _, c := range s.conversations {
		if owner == "" || c.Owner == owner {
			// Return without full messages for listing
			summary := *c
			if len(summary.Messages) > 0 {
				summary.Messages = []ChatMessage{summary.Messages[len(summary.Messages)-1]}
			}
			result = append(result, summary)
		}
	}
	return result
}

// AddMessage adds a message to a conversation.
func (s *ChatStore) AddMessage(conversationID string, msg ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.conversations[conversationID]
	if !ok {
		return fmt.Errorf("conversation %q not found", conversationID)
	}
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now().UTC()

	// Auto-title from first user message
	if c.Title == "" && msg.Role == "user" {
		title := msg.Content
		if len(title) > 60 {
			title = title[:60] + "..."
		}
		c.Title = title
	}

	return s.save()
}

// DeleteConversation removes a conversation.
func (s *ChatStore) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.conversations[id]; !ok {
		return fmt.Errorf("conversation %q not found", id)
	}
	delete(s.conversations, id)
	return s.save()
}

// PruneOld removes conversations older than the given duration.
func (s *ChatStore) PruneOld(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().UTC().Add(-maxAge)
	count := 0
	for id, c := range s.conversations {
		if c.UpdatedAt.Before(cutoff) {
			delete(s.conversations, id)
			count++
		}
	}
	if count > 0 {
		s.save()
	}
	return count
}

// AIChatHandler handles the generic AI chat API.
type AIChatHandler struct {
	chatStore    ChatRepository
	checkStore   Store
	incidentRepo IncidentRepository
	aiProvider   ChatProvider
	maxMessages  int // max messages in context window
}

// NewAIChatHandler creates the AI chat handler.
func NewAIChatHandler(
	chatStore ChatRepository,
	checkStore Store,
	incidentRepo IncidentRepository,
	aiProvider ChatProvider,
) *AIChatHandler {
	return &AIChatHandler{
		chatStore:    chatStore,
		checkStore:   checkStore,
		incidentRepo: incidentRepo,
		aiProvider:   aiProvider,
		maxMessages:  20,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (h *AIChatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/chat/conversations", h.handleConversations)
	mux.HandleFunc("/api/v1/chat/conversations/", h.handleConversationByID)
	mux.HandleFunc("/api/v1/chat/ask", h.handleAsk)
	mux.HandleFunc("/api/v1/chat/suggestions", h.handleSuggestions)
}

func (h *AIChatHandler) handleConversations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Owner is derived from JWT claims, not from query params
		owner := ownerFromRequest(r)
		convos := h.chatStore.ListConversations(owner)
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(convos))
	case http.MethodPost:
		var req struct {
			Title   string `json:"title"`
			Context string `json:"context"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
			return
		}
		// Owner is derived from JWT claims
		owner := ownerFromRequest(r)
		conv := h.chatStore.CreateConversation(req.Title, owner, req.Context)
		WriteAPIResponse(w, http.StatusCreated, NewAPIResponse(conv))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AIChatHandler) handleConversationByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/chat/conversations/")
	id := strings.SplitN(path, "/", 2)[0]
	if id == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("conversation ID required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		conv, err := h.chatStore.GetConversation(id)
		if err != nil {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		// Ownership check: users can only access their own conversations
		owner := ownerFromRequest(r)
		if owner != "" && conv.Owner != "" && conv.Owner != owner {
			WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
			return
		}
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(conv))
	case http.MethodDelete:
		conv, err := h.chatStore.GetConversation(id)
		if err != nil {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		// Ownership check: users can only delete their own conversations
		owner := ownerFromRequest(r)
		if owner != "" && conv.Owner != "" && conv.Owner != owner {
			WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
			return
		}
		if err := h.chatStore.DeleteConversation(id); err != nil {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		WriteAPIResponse(w, http.StatusOK, NewAPIResponse(map[string]string{"deleted": id}))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AIChatHandler) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.aiProvider == nil {
		WriteAPIError(w, http.StatusServiceUnavailable, fmt.Errorf("AI provider not configured"))
		return
	}

	var req ChatAskRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	if req.Message == "" {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("message is required"))
		return
	}
	if len(req.Message) > 4000 {
		WriteAPIError(w, http.StatusBadRequest, fmt.Errorf("message must be under 4000 characters"))
		return
	}

	start := time.Now()
	owner := ownerFromRequest(r)

	// Get or create conversation
	var conv *ChatConversation
	if req.ConversationID != "" {
		var err error
		conv, err = h.chatStore.GetConversation(req.ConversationID)
		if err != nil {
			WriteAPIError(w, http.StatusNotFound, err)
			return
		}
		// Ownership check
		if owner != "" && conv.Owner != "" && conv.Owner != owner {
			WriteAPIError(w, http.StatusForbidden, fmt.Errorf("access denied"))
			return
		}
	} else {
		conv = h.chatStore.CreateConversation("", owner, req.Context)
	}

	// Add user message
	userMsg := ChatMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now().UTC(),
	}
	h.chatStore.AddMessage(conv.ID, userMsg)

	// Build system context
	systemPrompt := h.buildSystemPrompt(conv, req.Context)
	userPrompt := h.buildUserPrompt(conv, req.Message)

	// Call AI
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	answer, err := h.aiProvider(ctx, systemPrompt, userPrompt)
	if err != nil {
		WriteAPIError(w, http.StatusBadGateway, fmt.Errorf("AI provider error: %w", err))
		return
	}

	// Extract references from answer
	refs := h.extractReferences(answer)

	// Save assistant message
	assistantMsg := ChatMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Content:   answer,
		Timestamp: time.Now().UTC(),
		Metadata: map[string]interface{}{
			"references": refs,
		},
	}
	h.chatStore.AddMessage(conv.ID, assistantMsg)

	// Generate follow-up suggestions
	suggestions := h.generateSuggestions(answer, req.Context)

	resp := ChatAskResponse{
		ConversationID: conv.ID,
		Message:        assistantMsg,
		Suggestions:    suggestions,
		References:     refs,
		DurationMs:     time.Since(start).Milliseconds(),
	}

	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(resp))
}

func (h *AIChatHandler) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Context-aware suggestions based on current system state
	state := h.checkStore.Snapshot()
	suggestions := h.buildContextualSuggestions(state)
	WriteAPIResponse(w, http.StatusOK, NewAPIResponse(suggestions))
}

func (h *AIChatHandler) buildSystemPrompt(conv *ChatConversation, contextHint string) string {
	state := h.checkStore.Snapshot()

	var sb strings.Builder
	sb.WriteString("You are HealthOps AI Assistant — an expert in infrastructure monitoring, incident response, and system reliability.\n\n")
	sb.WriteString("## Capabilities\n")
	sb.WriteString("- Explain health check results and alert conditions\n")
	sb.WriteString("- Diagnose root causes of failures and suggest remediations\n")
	sb.WriteString("- Provide recommendations for monitoring improvements\n")
	sb.WriteString("- Help configure alert rules and check parameters\n")
	sb.WriteString("- Analyze incident patterns and suggest preventive measures\n")
	sb.WriteString("- Answer questions about system status and uptime\n\n")

	// Current system snapshot
	sb.WriteString("## Current System Status\n")
	summary := buildSummary(state.Checks, state.Results, &state.LastRunAt)
	sb.WriteString(fmt.Sprintf("- Total checks: %d\n", summary.TotalChecks))
	sb.WriteString(fmt.Sprintf("- Healthy: %d, Warning: %d, Critical: %d\n", summary.Healthy, summary.Warning, summary.Critical))

	// Add unhealthy checks detail
	unhealthyCount := 0
	for _, r := range state.Results {
		if r.Status != "healthy" && unhealthyCount < 10 {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s - %s\n", r.Status, r.CheckID, r.Name, r.Message))
			unhealthyCount++
		}
	}

	// Add recent incidents
	if h.incidentRepo != nil {
		incidents, _ := h.incidentRepo.ListIncidents()
		openCount := 0
		for _, inc := range incidents {
			if inc.Status == "open" && openCount < 5 {
				sb.WriteString(fmt.Sprintf("- OPEN INCIDENT: %s (%s) - %s\n", inc.CheckName, inc.Severity, inc.Message))
				openCount++
			}
		}
	}

	// Context-specific additions
	if contextHint != "" {
		sb.WriteString(fmt.Sprintf("\n## Focus Context: %s\n", contextHint))
		if strings.HasPrefix(contextHint, "incident:") {
			incID := strings.TrimPrefix(contextHint, "incident:")
			if h.incidentRepo != nil {
				inc, err := h.incidentRepo.GetIncident(incID)
				if err == nil {
					sb.WriteString(fmt.Sprintf("Discussing incident: %s\nCheck: %s\nSeverity: %s\nStatus: %s\nMessage: %s\n",
						inc.ID, inc.CheckName, inc.Severity, inc.Status, inc.Message))
				}
			}
		}
	}

	sb.WriteString("\n## Instructions\n")
	sb.WriteString("- Be concise and actionable in your responses\n")
	sb.WriteString("- When suggesting fixes, provide specific commands or configuration changes\n")
	sb.WriteString("- Reference specific checks, servers, or incidents by name when relevant\n")
	sb.WriteString("- If asked about something outside monitoring scope, politely redirect\n")
	sb.WriteString("- Use markdown formatting for readability\n")

	return sb.String()
}

func (h *AIChatHandler) buildUserPrompt(conv *ChatConversation, currentMessage string) string {
	var sb strings.Builder

	// Include recent conversation history for context
	reloadedConv, err := h.chatStore.GetConversation(conv.ID)
	if err == nil && len(reloadedConv.Messages) > 1 {
		// Include last N messages as context
		msgs := reloadedConv.Messages
		start := 0
		if len(msgs) > h.maxMessages {
			start = len(msgs) - h.maxMessages
		}
		for i := start; i < len(msgs)-1; i++ { // exclude the just-added user message
			msg := msgs[i]
			sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
		}
	}

	sb.WriteString(fmt.Sprintf("[user]: %s", currentMessage))
	return sb.String()
}

func (h *AIChatHandler) extractReferences(answer string) []ChatReference {
	var refs []ChatReference
	state := h.checkStore.Snapshot()

	// Find check references in the answer
	for _, check := range state.Checks {
		if strings.Contains(answer, check.ID) || strings.Contains(answer, check.Name) {
			refs = append(refs, ChatReference{
				Type: "check",
				ID:   check.ID,
				Name: check.Name,
				URL:  fmt.Sprintf("/checks/%s", check.ID),
			})
		}
	}

	// Find incident references
	if h.incidentRepo != nil {
		incidents, _ := h.incidentRepo.ListIncidents()
		for _, inc := range incidents {
			if strings.Contains(answer, inc.ID) {
				refs = append(refs, ChatReference{
					Type: "incident",
					ID:   inc.ID,
					Name: inc.CheckName,
					URL:  fmt.Sprintf("/incidents/%s", inc.ID),
				})
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []ChatReference
	for _, ref := range refs {
		key := ref.Type + ":" + ref.ID
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ref)
		}
	}

	// Cap at 10 references
	if len(unique) > 10 {
		unique = unique[:10]
	}
	return unique
}

func (h *AIChatHandler) generateSuggestions(answer, contextHint string) []ChatSuggestion {
	suggestions := []ChatSuggestion{}

	// Context-aware suggestions
	if strings.Contains(answer, "critical") || strings.Contains(answer, "outage") {
		suggestions = append(suggestions, ChatSuggestion{
			Text:     "What's the root cause of this failure?",
			Category: "debug",
		})
		suggestions = append(suggestions, ChatSuggestion{
			Text:     "How can I prevent this from happening again?",
			Category: "action",
		})
	}

	if strings.Contains(answer, "latency") || strings.Contains(answer, "slow") {
		suggestions = append(suggestions, ChatSuggestion{
			Text:     "What are the performance trends over the last 24 hours?",
			Category: "status",
		})
	}

	if strings.Contains(contextHint, "incident") {
		suggestions = append(suggestions, ChatSuggestion{
			Text:     "What actions should I take to resolve this incident?",
			Category: "action",
		})
		suggestions = append(suggestions, ChatSuggestion{
			Text:     "Has this type of incident occurred before?",
			Category: "explain",
		})
	}

	// Default suggestions if none generated
	if len(suggestions) == 0 {
		suggestions = append(suggestions,
			ChatSuggestion{Text: "Show me the current system health overview", Category: "status"},
			ChatSuggestion{Text: "Are there any active incidents?", Category: "status"},
			ChatSuggestion{Text: "What checks are failing right now?", Category: "debug"},
		)
	}

	// Limit to 4
	if len(suggestions) > 4 {
		suggestions = suggestions[:4]
	}
	return suggestions
}

func (h *AIChatHandler) buildContextualSuggestions(state State) []ChatSuggestion {
	suggestions := []ChatSuggestion{}

	// Check for unhealthy systems
	unhealthyCount := 0
	for _, r := range state.Results {
		if r.Status == "critical" {
			unhealthyCount++
		}
	}

	if unhealthyCount > 0 {
		suggestions = append(suggestions, ChatSuggestion{
			Text:     fmt.Sprintf("Why are %d checks failing?", unhealthyCount),
			Category: "debug",
		})
		suggestions = append(suggestions, ChatSuggestion{
			Text:     "What's causing the current critical alerts?",
			Category: "debug",
		})
	}

	// Open incidents
	if h.incidentRepo != nil {
		incidents, _ := h.incidentRepo.ListIncidents()
		openCount := 0
		for _, inc := range incidents {
			if inc.Status == "open" {
				openCount++
			}
		}
		if openCount > 0 {
			suggestions = append(suggestions, ChatSuggestion{
				Text:     fmt.Sprintf("Tell me about the %d open incidents", openCount),
				Category: "status",
			})
		}
	}

	// General suggestions
	suggestions = append(suggestions,
		ChatSuggestion{Text: "What's the overall system health?", Category: "status"},
		ChatSuggestion{Text: "Which checks have the highest latency?", Category: "status"},
		ChatSuggestion{Text: "Suggest improvements for my monitoring setup", Category: "action"},
	)

	if len(suggestions) > 6 {
		suggestions = suggestions[:6]
	}
	return suggestions
}
