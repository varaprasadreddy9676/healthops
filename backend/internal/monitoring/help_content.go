package monitoring

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

//go:embed helpcontent/*.md
var helpContentFS embed.FS

// HelpTopic is a single help/documentation page.
type HelpTopic struct {
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary,omitempty"`
	Intent        string   `json:"intent,omitempty"`
	Category      string   `json:"category,omitempty"`
	Order         int      `json:"order,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	RelatedPaths  []string `json:"relatedPaths,omitempty"`
	RelatedTopics []string `json:"relatedTopics,omitempty"`
	Body          string   `json:"body,omitempty"`
}

// HelpTopicSummary is the lightweight projection used for list views.
type HelpTopicSummary struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary,omitempty"`
	Category     string   `json:"category,omitempty"`
	Order        int      `json:"order"`
	Icon         string   `json:"icon,omitempty"`
	RelatedPaths []string `json:"relatedPaths,omitempty"`
}

// HelpAPIHandler serves the embedded help content.
type HelpAPIHandler struct {
	topics  []HelpTopic
	bySlug  map[string]*HelpTopic
	once    sync.Once
	loadErr error
}

// NewHelpAPIHandler creates a help API handler with content loaded from the embedded FS.
func NewHelpAPIHandler() *HelpAPIHandler {
	h := &HelpAPIHandler{}
	h.load()
	return h
}

// RegisterRoutes registers help routes (public — no auth required).
func (h *HelpAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/help/topics", h.handleList)
	mux.HandleFunc("GET /api/v1/help/topics/{slug}", h.handleGet)
}

func (h *HelpAPIHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if h.loadErr != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   &APIError{Code: 500, Message: h.loadErr.Error()},
		})
		return
	}
	summaries := make([]HelpTopicSummary, 0, len(h.topics))
	for _, t := range h.topics {
		summaries = append(summaries, HelpTopicSummary{
			Slug:         t.Slug,
			Title:        t.Title,
			Summary:      t.Summary,
			Category:     t.Category,
			Order:        t.Order,
			Icon:         t.Icon,
			RelatedPaths: t.RelatedPaths,
		})
	}
	writeJSON(w, http.StatusOK, NewAPIResponse(summaries))
}

func (h *HelpAPIHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if h.loadErr != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   &APIError{Code: 500, Message: h.loadErr.Error()},
		})
		return
	}
	slug := r.PathValue("slug")
	topic, ok := h.bySlug[slug]
	if !ok {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   &APIError{Code: 404, Message: "help topic not found"},
		})
		return
	}
	writeJSON(w, http.StatusOK, NewAPIResponse(topic))
}

func (h *HelpAPIHandler) load() {
	h.once.Do(func() {
		entries, err := helpContentFS.ReadDir("helpcontent")
		if err != nil {
			h.loadErr = fmt.Errorf("read help content dir: %w", err)
			return
		}
		topics := make([]HelpTopic, 0, len(entries))
		bySlug := make(map[string]*HelpTopic, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			raw, err := helpContentFS.ReadFile("helpcontent/" + e.Name())
			if err != nil {
				h.loadErr = fmt.Errorf("read %s: %w", e.Name(), err)
				return
			}
			topic, err := parseHelpTopic(raw)
			if err != nil {
				h.loadErr = fmt.Errorf("parse %s: %w", e.Name(), err)
				return
			}
			if topic.Slug == "" {
				topic.Slug = strings.TrimSuffix(e.Name(), ".md")
			}
			topics = append(topics, topic)
		}
		sort.SliceStable(topics, func(i, j int) bool {
			if topics[i].Order != topics[j].Order {
				return topics[i].Order < topics[j].Order
			}
			return topics[i].Title < topics[j].Title
		})
		for i := range topics {
			bySlug[topics[i].Slug] = &topics[i]
		}
		h.topics = topics
		h.bySlug = bySlug
	})
}

// parseHelpTopic parses a markdown file with YAML-like frontmatter delimited by
// lines containing only "---". The frontmatter supports simple key: value pairs;
// values may be comma-separated for list fields.
func parseHelpTopic(raw []byte) (HelpTopic, error) {
	t := HelpTopic{}
	text := string(raw)
	// Strip leading BOM/whitespace then require leading "---" line.
	text = strings.TrimLeft(text, "\ufeff")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		// No frontmatter — treat entire file as body, derive title later.
		t.Body = text
		return t, nil
	}
	// Find closing fence
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return t, fmt.Errorf("frontmatter not closed")
	}
	for i := 1; i < end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, "\"'")
		switch key {
		case "slug":
			t.Slug = val
		case "title":
			t.Title = val
		case "summary":
			t.Summary = val
		case "intent":
			t.Intent = val
		case "category":
			t.Category = val
		case "order":
			var n int
			fmt.Sscanf(val, "%d", &n)
			t.Order = n
		case "icon":
			t.Icon = val
		case "relatedPaths":
			t.RelatedPaths = splitCSV(val)
		case "relatedTopics":
			t.RelatedTopics = splitCSV(val)
		}
	}
	if end+1 < len(lines) {
		t.Body = strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")
	}
	return t, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
