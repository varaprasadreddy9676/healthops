package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"medics-health-check/backend/internal/monitoring"
)

func main() {
	logger := log.New(os.Stdout, "healthmon ", log.LstdFlags|log.Lmicroseconds)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configPath := resolvePath("CONFIG_PATH", filepath.Join("backend", "config", "default.json"), filepath.Join("config", "default.json"))
	mongoURI := os.Getenv("MONGODB_URI")
	mongoDB := envOrDefault("MONGODB_DATABASE", "healthmon")
	mongoPrefix := envOrDefault("MONGODB_COLLECTION_PREFIX", "healthmon")

	cfg, err := monitoring.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	var store monitoring.Store
	statePath := resolvePath("STATE_PATH", filepath.Join("backend", "data", "state.json"), filepath.Join("data", "state.json"))

	if mongoURI != "" {
		// Use HybridStore with MongoDB mirror
		store, err = monitoring.NewHybridStore(statePath, cfg.Checks, mongoURI, mongoDB, mongoPrefix, cfg.RetentionDays, logger)
		if err != nil {
			logger.Fatalf("init hybrid store: %v", err)
		}
		logger.Printf("running with hybrid storage (local file + MongoDB mirror)")
	} else {
		// Use only local file store
		store, err = monitoring.NewFileStore(statePath, cfg.Checks)
		if err != nil {
			logger.Fatalf("init file store: %v", err)
		}
		logger.Printf("running with local file persistence")
	}

	service := monitoring.NewService(cfg, store, logger)
	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("service stopped: %v", err)
	}
}

func resolvePath(envKey string, candidates ...string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
