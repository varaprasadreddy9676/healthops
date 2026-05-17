package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChatStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewChatStore(dir + "/chat.json")
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}

	conv := store.CreateConversation("Test Conversation", "user1", "")
	if conv.ID == "" {
		t.Error("expected non-empty ID")
	}
	if conv.Title != "Test Conversation" {
		t.Errorf("Title = %q; want 'Test Conversation'", conv.Title)
	}
	if len(conv.Messages) != 0 {
		t.Errorf("len(Messages) = %d; want 0", len(conv.Messages))
	}

	got, err := store.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Test Conversation" {
		t.Errorf("Get Title = %q", got.Title)
	}
}

func TestChatStore_AddMessage(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	conv := store.CreateConversation("With Messages", "", "")
	err := store.AddMessage(conv.ID, ChatMessage{
		Role:    "user",
		Content: "Hello, how are my servers?",
	})
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	got, _ := store.GetConversation(conv.ID)
	if len(got.Messages) != 1 {
		t.Fatalf("Messages count = %d; want 1", len(got.Messages))
	}
	if got.Messages[0].Role != "user" {
		t.Errorf("Role = %q; want 'user'", got.Messages[0].Role)
	}
}

func TestChatStore_AddMessageToNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	err := store.AddMessage("nonexistent", ChatMessage{Role: "user", Content: "hi"})
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

func TestChatStore_AutoTitle(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	conv := store.CreateConversation("", "", "")
	store.AddMessage(conv.ID, ChatMessage{Role: "user", Content: "What is the status of my API?"})

	got, _ := store.GetConversation(conv.ID)
	if got.Title == "" {
		t.Error("expected auto-generated title from first user message")
	}
}

func TestChatStore_List(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	store.CreateConversation("First", "user1", "")
	store.CreateConversation("Second", "user2", "")
	store.CreateConversation("Third", "user1", "")

	all := store.ListConversations("")
	if len(all) != 3 {
		t.Errorf("List all: count = %d; want 3", len(all))
	}

	user1 := store.ListConversations("user1")
	if len(user1) != 2 {
		t.Errorf("List user1: count = %d; want 2", len(user1))
	}
}

func TestChatStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	conv := store.CreateConversation("To Delete", "", "")
	if err := store.DeleteConversation(conv.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.GetConversation(conv.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestChatStore_DeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	err := store.DeleteConversation("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestChatStore_PruneOld(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	conv := store.CreateConversation("Old Conversation", "", "")
	store.mu.Lock()
	if c, ok := store.conversations[conv.ID]; ok {
		c.UpdatedAt = time.Now().Add(-31 * 24 * time.Hour)
	}
	store.mu.Unlock()

	store.CreateConversation("Recent Conversation", "", "")

	pruned := store.PruneOld(30 * 24 * time.Hour)
	if pruned != 1 {
		t.Errorf("Pruned = %d; want 1", pruned)
	}

	list := store.ListConversations("")
	if len(list) != 1 {
		t.Errorf("After prune: %d conversations; want 1", len(list))
	}
}

func TestChatStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/chat.json"

	store1, _ := NewChatStore(path)
	conv := store1.CreateConversation("Persistent", "", "")
	store1.AddMessage(conv.ID, ChatMessage{Role: "user", Content: "msg"})

	store2, _ := NewChatStore(path)
	list := store2.ListConversations("")
	if len(list) != 1 || list[0].Title != "Persistent" {
		t.Errorf("persistence: %+v", list)
	}

	got, _ := store2.GetConversation(conv.ID)
	if len(got.Messages) != 1 {
		t.Errorf("messages not persisted: %d", len(got.Messages))
	}
}

func TestChatStore_BadFile(t *testing.T) {
	dir := t.TempDir()
	badFile := dir + "/bad.json"
	os.WriteFile(badFile, []byte("{bad"), 0644)

	_, err := NewChatStore(badFile)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- AI Chat Handler Tests ---

func TestAIChatHandler_CreateConversation(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"title":"New Chat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create: status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAIChatHandler_ListConversations(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	store.CreateConversation("Chat A", "", "")
	store.CreateConversation("Chat B", "", "")
	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("List: status = %d", w.Code)
	}
}

func TestAIChatHandler_GetConversation(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	conv := store.CreateConversation("My Chat", "", "")
	store.AddMessage(conv.ID, ChatMessage{Role: "user", Content: "Hello"})

	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+conv.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Get: status = %d", w.Code)
	}
}

func TestAIChatHandler_DeleteConversation(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	conv := store.CreateConversation("Delete Me", "", "")

	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/conversations/"+conv.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Delete: status = %d", w.Code)
	}
}

func TestAIChatHandler_AskWithoutProvider(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	conv := store.CreateConversation("Ask Chat", "", "")

	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"conversationId":"` + conv.ID + `","message":"What is the status?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/ask", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Ask without provider: status = %d; want 503. Body: %s", w.Code, w.Body.String())
	}
}

func TestAIChatHandler_AskWithProvider(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	conv := store.CreateConversation("AI Chat", "", "")

	enabled := true
	checks := []CheckConfig{
		{ID: "web", Name: "Web Server", Type: "api", Enabled: &enabled},
	}
	results := []CheckResult{
		{CheckID: "web", Status: "healthy", Healthy: true, StartedAt: time.Now()},
	}
	fakeS := &fakeStore{snapshot: State{Checks: checks, Results: results}}

	fakeProvider := ChatProvider(func(ctx context.Context, systemMsg, userMsg string) (string, error) {
		return "All systems are operational. Your web server is healthy.", nil
	})

	handler := NewAIChatHandler(store, fakeS, nil, fakeProvider)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := `{"conversationId":"` + conv.ID + `","message":"How are my servers?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/ask", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Ask: status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data ChatAskResponse `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Message.Content == "" {
		t.Error("expected non-empty response")
	}
}

func TestAIChatHandler_Suggestions(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")

	enabled := true
	checks := []CheckConfig{
		{ID: "api", Name: "API", Type: "api", Enabled: &enabled},
	}
	results := []CheckResult{
		{CheckID: "api", Status: "critical", Healthy: false, StartedAt: time.Now()},
	}
	fakeS := &fakeStore{snapshot: State{Checks: checks, Results: results}}

	handler := NewAIChatHandler(store, fakeS, nil, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/suggestions", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Suggestions: status = %d", w.Code)
	}

	var resp struct {
		Data []ChatSuggestion `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) == 0 {
		t.Error("expected at least one suggestion")
	}
}

func TestAIChatHandler_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	// Need a provider so the handler doesn't short-circuit with 503
	fakeProvider := ChatProvider(func(ctx context.Context, systemMsg, userMsg string) (string, error) {
		return "", nil
	})
	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, fakeProvider)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/ask", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestAIChatHandler_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewChatStore(dir + "/chat.json")
	handler := NewAIChatHandler(store, &fakeStore{snapshot: State{}}, nil, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
}
