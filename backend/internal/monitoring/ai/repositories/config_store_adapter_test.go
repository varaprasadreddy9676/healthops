package repositories

import (
	"context"
	"sync"
	"testing"
	"time"

	"medics-health-check/backend/internal/monitoring/ai"
)

func TestMongoAIConfigStoreAdapterUpdateDoesNotDeadlock(t *testing.T) {
	repo := &fakeAIProviderRepository{providers: map[string]*AIProvider{}}
	adapter := &MongoAIConfigStoreAdapter{
		repo:          repo,
		serviceConfig: ai.DefaultAIServiceConfig(),
	}

	done := make(chan error, 1)
	go func() {
		done <- adapter.Update(func(cfg *ai.AIServiceConfig) error {
			cfg.Enabled = true
			cfg.Providers = append(cfg.Providers, ai.AIProviderConfig{
				ID:          "openrouter-free",
				Provider:    ai.AIProviderCustom,
				Name:        "OpenRouter Free Router",
				APIKey:      "test-key",
				BaseURL:     "https://openrouter.ai/api/v1",
				Model:       "openrouter/free",
				MaxTokens:   1200,
				Temperature: 0.2,
				Enabled:     true,
				IsDefault:   true,
			})
			return nil
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Update returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Update deadlocked while holding the adapter lock")
	}

	cfg := adapter.Get()
	if !cfg.Enabled {
		t.Fatal("expected enabled config to be cached after update")
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].ID != "openrouter-free" {
		t.Fatalf("unexpected provider ID %q", cfg.Providers[0].ID)
	}
}

type fakeAIProviderRepository struct {
	mu        sync.Mutex
	providers map[string]*AIProvider
}

func (r *fakeAIProviderRepository) List(context.Context) ([]*AIProvider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	providers := make([]*AIProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		copyProvider := *provider
		providers = append(providers, &copyProvider)
	}
	return providers, nil
}

func (r *fakeAIProviderRepository) Create(_ context.Context, provider *AIProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyProvider := *provider
	r.providers[provider.ID] = &copyProvider
	return nil
}

func (r *fakeAIProviderRepository) Update(_ context.Context, provider *AIProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyProvider := *provider
	r.providers[provider.ID] = &copyProvider
	return nil
}

func (r *fakeAIProviderRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.providers, id)
	return nil
}
