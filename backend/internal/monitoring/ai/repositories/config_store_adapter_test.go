package repositories

import (
	"context"
	"sync"
	"testing"
	"time"

	"health-ops/backend/internal/monitoring/ai"
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

func TestMongoAIConfigStoreAdapterPersistsServiceSettings(t *testing.T) {
	providers := &fakeAIProviderRepository{providers: map[string]*AIProvider{}}
	settings := &fakeAIServiceSettingsRepository{config: ai.DefaultAIServiceConfig()}

	adapter := &MongoAIConfigStoreAdapter{
		repo:          providers,
		settingsRepo:  settings,
		serviceConfig: ai.DefaultAIServiceConfig(),
	}

	if err := adapter.Update(func(cfg *ai.AIServiceConfig) error {
		cfg.Enabled = true
		cfg.AutoAnalyze = false
		cfg.TimeoutSeconds = 45
		cfg.RetryCount = 3
		cfg.RetryDelayMs = 1500
		cfg.DefaultPromptID = "custom-prompt"
		cfg.Prompts = append(cfg.Prompts, ai.AIPromptTemplate{
			ID:        "custom-prompt",
			Name:      "Custom Prompt",
			SystemMsg: "system",
			UserMsg:   "user",
			Version:   "v1",
		})
		cfg.Providers = append(cfg.Providers, ai.AIProviderConfig{
			ID:        "openrouter-live",
			Provider:  ai.AIProviderCustom,
			Name:      "OpenRouter Live",
			APIKey:    "test-key",
			BaseURL:   "https://openrouter.ai/api/v1",
			Model:     "openai/gpt-4o-mini",
			Enabled:   true,
			IsDefault: true,
		})
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	restarted := &MongoAIConfigStoreAdapter{
		repo:          providers,
		settingsRepo:  settings,
		serviceConfig: ai.DefaultAIServiceConfig(),
	}
	cfg := restarted.Get()
	if !cfg.Enabled {
		t.Fatal("expected enabled setting to survive adapter restart")
	}
	if cfg.AutoAnalyze {
		t.Fatal("expected autoAnalyze=false to survive adapter restart")
	}
	if cfg.TimeoutSeconds != 45 || cfg.RetryCount != 3 || cfg.RetryDelayMs != 1500 {
		t.Fatalf("settings were not persisted: timeout=%d retry=%d delay=%d", cfg.TimeoutSeconds, cfg.RetryCount, cfg.RetryDelayMs)
	}
	if cfg.DefaultPromptID != "custom-prompt" {
		t.Fatalf("expected default prompt to survive restart, got %q", cfg.DefaultPromptID)
	}
	if len(cfg.Prompts) < 3 {
		t.Fatalf("expected persisted prompts, got %d", len(cfg.Prompts))
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].ID != "openrouter-live" {
		t.Fatalf("expected provider to be loaded from provider repository, got %+v", cfg.Providers)
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

type fakeAIServiceSettingsRepository struct {
	mu     sync.Mutex
	config ai.AIServiceConfig
}

func (r *fakeAIServiceSettingsRepository) GetServiceConfig(context.Context) (ai.AIServiceConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return copyServiceConfig(r.config), nil
}

func (r *fakeAIServiceSettingsRepository) UpdateServiceConfig(_ context.Context, cfg ai.AIServiceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = copyServiceConfig(cfg)
	return nil
}
