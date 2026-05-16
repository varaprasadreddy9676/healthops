package evidence

import (
	"context"
	"sync"
)

// EvidenceProvider collects signals for a specific evidence category.
// Each phase registers new providers without modifying the context builder.
type EvidenceProvider interface {
	// Category returns the evidence category name (e.g. "checks", "mysql",
	// "server_metrics", "audit", "incident_history").
	Category() string

	// Collect gathers evidence for the given incident within the time window.
	// Returns SignalEvents that the context builder will include in the AI prompt.
	Collect(ctx context.Context, incidentID string, window TimeWindow) ([]SignalEvent, error)
}

// Registry holds all registered EvidenceProviders. It is safe for concurrent
// use.
type Registry struct {
	mu        sync.RWMutex
	providers []EvidenceProvider
}

// NewRegistry creates an empty evidence provider registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p EvidenceProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// Providers returns a snapshot of all registered providers.
func (r *Registry) Providers() []EvidenceProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EvidenceProvider, len(r.providers))
	copy(out, r.providers)
	return out
}

// Categories returns the names of all registered evidence categories.
func (r *Registry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cats := make([]string, len(r.providers))
	for i, p := range r.providers {
		cats[i] = p.Category()
	}
	return cats
}
