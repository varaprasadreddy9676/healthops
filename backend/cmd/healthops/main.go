package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"medics-health-check/backend/internal/monitoring"
	"medics-health-check/backend/internal/monitoring/ai"
	airepositories "medics-health-check/backend/internal/monitoring/ai/repositories"
	"medics-health-check/backend/internal/monitoring/assistant"
	"medics-health-check/backend/internal/monitoring/automation"
	"medics-health-check/backend/internal/monitoring/evidence"
	"medics-health-check/backend/internal/monitoring/logs"
	"medics-health-check/backend/internal/monitoring/mysql"
	"medics-health-check/backend/internal/monitoring/notify"
	"medics-health-check/backend/internal/monitoring/rca"
	"medics-health-check/backend/internal/monitoring/recommendations"
	"medics-health-check/backend/internal/monitoring/repositories"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	logger := log.New(os.Stdout, "healthops ", log.LstdFlags|log.Lmicroseconds)

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		logger.Printf("Warning: .env file not found or error loading: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configPath := resolvePath("CONFIG_PATH", filepath.Join("backend", "config", "default.json"), filepath.Join("config", "default.json"))
	mongoURI := os.Getenv("MONGODB_URI")
	mongoDB := envOrDefault("MONGODB_DATABASE", "healthops")
	mongoPrefix := envOrDefault("MONGODB_COLLECTION_PREFIX", "healthops")
	storageBackend := strings.ToLower(envOrDefault("STORAGE_BACKEND", "file"))
	useMongoPhase0 := storageBackend == "mongo"
	if useMongoPhase0 && mongoURI == "" {
		logger.Fatalf("STORAGE_BACKEND=mongo requires MONGODB_URI to be set")
	}

	cfg, err := monitoring.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	var store monitoring.Store
	var hybridStore *monitoring.HybridStore
	var mongoStore *monitoring.MongoStore
	statePath := resolvePath("STATE_PATH", filepath.Join("backend", "data", "state.json"), filepath.Join("data", "state.json"))

	// Initialize MongoDB client early when Mongo is configured.
	// This single client is shared between the state store and all repositories.
	var mongoClient *mongo.Client
	if mongoURI != "" {
		clientOpts := options.Client().
			ApplyURI(mongoURI).
			SetServerSelectionTimeout(10 * time.Second).
			SetConnectTimeout(10 * time.Second).
			SetMaxPoolSize(100)

		var connErr error
		mongoClient, connErr = mongo.Connect(clientOpts)
		if connErr != nil {
			if useMongoPhase0 {
				logger.Fatalf("STORAGE_BACKEND=mongo but failed to connect to MongoDB: %v", connErr)
			}
			logger.Printf("Warning: Failed to connect to MongoDB: %v", connErr)
			mongoClient = nil
		} else {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if pingErr := mongoClient.Ping(pingCtx, nil); pingErr != nil {
				pingCancel()
				if useMongoPhase0 {
					_ = mongoClient.Disconnect(context.Background())
					logger.Fatalf("STORAGE_BACKEND=mongo but MongoDB ping failed: %v", pingErr)
				}
				logger.Printf("Warning: MongoDB ping failed: %v", pingErr)
				_ = mongoClient.Disconnect(context.Background())
				mongoClient = nil
			} else {
				pingCancel()
				logger.Printf("MongoDB connection established")
			}
		}
	}

	if useMongoPhase0 && mongoClient != nil {
		// Phase 0: MongoDB is the sole persistence layer for state.
		ms, err := monitoring.NewMongoStore(mongoClient, mongoDB, mongoPrefix, cfg.RetentionDays, cfg.Checks, logger)
		if err != nil {
			logger.Fatalf("init mongo store: %v", err)
		}
		mongoStore = ms
		store = ms
		logger.Printf("running with MongoDB as sole persistence (Phase 0 — no file fallback)")
	} else if mongoClient != nil {
		// Legacy: HybridStore with MongoDB as primary, local file as fallback
		hs, err := monitoring.NewHybridStore(statePath, cfg.Checks, mongoURI, mongoDB, mongoPrefix, cfg.RetentionDays, logger)
		if err != nil {
			logger.Fatalf("init hybrid store: %v", err)
		}
		hybridStore = hs
		store = hs
		logger.Printf("running with MongoDB as primary storage (local file fallback)")
	} else {
		// File-only mode
		store, err = monitoring.NewFileStore(statePath, cfg.Checks)
		if err != nil {
			logger.Fatalf("init file store: %v", err)
		}
		logger.Printf("running with local file persistence")
	}

	service := monitoring.NewService(cfg, store, logger)

	// Phase 0: /healthz must fail when MongoDB is unreachable
	if mongoStore != nil {
		service.SetMongoHealthCheck(func(ctx context.Context) error {
			return mongoClient.Ping(ctx, nil)
		})
	}

	// Warn about insecure auth configuration
	if !cfg.Auth.Enabled {
		logger.Printf("SECURITY WARNING: Authentication is disabled — set auth.enabled=true in config for production use")
	}

	// Initialize user store with MongoDB if available, otherwise file-based
	dataDir := resolvePath("DATA_DIR", filepath.Join("backend", "data"), "data")
	monitoring.InitJWTSecret(dataDir)

	if mongoClient != nil {
		mongoUserRepo, err := repositories.NewMongoUserRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Printf("Warning: Failed to init MongoDB user repository: %v", err)
			logger.Printf("Falling back to file-based user store")
			userStore, err := monitoring.NewUserStore(dataDir)
			if err != nil {
				logger.Printf("Warning: Failed to init file-based user store: %v", err)
			} else {
				service.SetUserStore(userStore)
				if userStore.IsUsingDefaultCredentials() {
					logger.Printf("WARNING: User management using default credentials — change immediately in production")
				} else {
					logger.Printf("User management initialized (file-based)")
				}
			}
		} else {
			bootstrapPassword := os.Getenv("HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD")
			bootstrapEmail := envOrDefault("HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL", "admin@healthops.local")
			bootstrapReset := envOrDefault("HEALTHOPS_BOOTSTRAP_ADMIN_RESET", "false") == "true"
			if bootstrapPassword != "" {
				bootstrapCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				changed, err := mongoUserRepo.BootstrapAdmin(bootstrapCtx, bootstrapPassword, bootstrapEmail, bootstrapReset)
				cancel()
				if err != nil {
					logger.Printf("Warning: Failed to bootstrap MongoDB admin user: %v", err)
				} else if changed {
					logger.Printf("MongoDB admin bootstrap applied")
				}
			}

			service.SetUserStore(repositories.NewUserStoreAdapter(mongoUserRepo))
			logger.Printf("User management initialized (MongoDB)")
		}
	} else {
		userStore, err := monitoring.NewUserStore(dataDir)
		if err != nil {
			logger.Printf("Warning: Failed to init user store: %v", err)
		} else {
			service.SetUserStore(userStore)
			if userStore.IsUsingDefaultCredentials() {
				logger.Printf("WARNING: User management using default credentials — change immediately in production")
			} else {
				logger.Printf("User management initialized")
			}
		}
	}

	// Initialize incident manager. STORAGE_BACKEND=mongo swaps in the
	// MongoDB-backed implementation so incidents survive process restarts.
	var incidentRepo monitoring.IncidentRepository
	var pruneIncidentRepo monitoring.Prunable
	if useMongoPhase0 && mongoClient != nil {
		mongoIncRepo, err := repositories.NewMongoIncidentRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo incident repository: %v", err)
		}
		incidentRepo = mongoIncRepo
		pruneIncidentRepo = mongoIncRepo
		logger.Printf("Incident repository: MongoDB (collection %s_incidents)", mongoPrefix)
	} else {
		memRepo := monitoring.NewMemoryIncidentRepository()
		incidentRepo = memRepo
		pruneIncidentRepo = memRepo
		if useMongoPhase0 {
			logger.Printf("WARNING: STORAGE_BACKEND=mongo requested but MongoDB unavailable — using in-memory incident repository")
		} else {
			logger.Printf("Incident repository: in-memory (set STORAGE_BACKEND=mongo to persist)")
		}
	}
	incidentManager := monitoring.NewIncidentManager(incidentRepo, logger)
	service.SetIncidentManager(incidentManager)

	// Start MongoDB health monitor — creates incidents when MongoDB goes down
	if hybridStore != nil && mongoURI != "" {
		stopMongoMonitor := make(chan struct{})
		go monitorMongoDB(hybridStore, incidentManager, logger, stopMongoMonitor)
		defer close(stopMongoMonitor)
	}

	// Initialize generic repositories (notification outbox, AI queue, snapshots).
	// outbox/aiQueue swap to MongoDB when STORAGE_BACKEND=mongo and a Mongo
	// client is available; otherwise they fall back to file-backed JSONL stores.
	var outbox monitoring.NotificationOutboxRepository
	var pruneOutbox monitoring.Prunable
	if useMongoPhase0 && mongoClient != nil {
		mongoOutbox, err := repositories.NewMongoNotificationOutbox(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo notification outbox: %v", err)
		}
		outbox = mongoOutbox
		pruneOutbox = mongoOutbox
		logger.Printf("Notification outbox: MongoDB (collection %s_notification_outbox)", mongoPrefix)
	} else {
		fileOutbox, err := notify.NewFileNotificationOutbox(filepath.Join(dataDir, "notification_outbox.jsonl"))
		if err != nil {
			logger.Printf("Warning: Failed to init notification outbox: %v", err)
		} else {
			outbox = fileOutbox
			pruneOutbox = fileOutbox
		}
	}

	// Initialize notification channel store and dispatcher
	var channelStore notify.ChannelStore
	mongoAvailableForChannels := (mongoStore != nil) || (hybridStore != nil && hybridStore.HasMongo() && !hybridStore.IsMongoDown())
	if mongoAvailableForChannels && mongoURI != "" {
		mongoChannelRepo, err := repositories.NewMongoChannelRepository(mongoURI, mongoDB, mongoPrefix, 5)
		if err != nil {
			logger.Printf("Warning: Failed to init MongoDB channel repository: %v", err)
			logger.Printf("Falling back to file-based channel store")
			channelStore, err = notify.NewNotificationChannelStore(dataDir)
			if err != nil {
				logger.Printf("Warning: Failed to init file-based channel store: %v", err)
			}
		} else {
			channelStore = repositories.NewChannelStoreAdapter(mongoChannelRepo)
			logger.Printf("MongoDB notification channel repository initialized")
		}
	} else {
		channelStore, err = notify.NewNotificationChannelStore(dataDir)
		if err != nil {
			logger.Printf("Warning: Failed to init notification channel store: %v", err)
		}
	}

	var notificationDispatcher *notify.NotificationDispatcher
	if channelStore != nil {
		notificationDispatcher = notify.NewNotificationDispatcher(channelStore, outbox, logger)
		defer notificationDispatcher.Stop() // flush pending notifications on shutdown
		// Set dashboard URL for email links
		addr := cfg.Server.Addr
		if addr == "" || addr == ":8080" {
			addr = "http://localhost:8080"
		}
		notificationDispatcher.SetDashboardURL(addr)
		notificationAPIHandler := notify.NewNotificationAPIHandler(channelStore, notificationDispatcher, cfg)
		service.SetNotifyRoutes(notificationAPIHandler)
		logger.Printf("Notification channels initialized")
	}

	var aiQueue monitoring.AIQueueRepository
	var pruneAIQueue monitoring.Prunable
	if useMongoPhase0 && mongoClient != nil {
		mongoQueue, err := airepositories.NewMongoAIQueue(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo ai queue: %v", err)
		}
		aiQueue = mongoQueue
		pruneAIQueue = mongoQueue
		logger.Printf("AI queue: MongoDB (collections %s_ai_queue, %s_ai_results)", mongoPrefix, mongoPrefix)
	} else {
		fileQueue, err := ai.NewFileAIQueue(dataDir)
		if err != nil {
			logger.Printf("Warning: Failed to init AI queue: %v", err)
		} else {
			aiQueue = fileQueue
			pruneAIQueue = fileQueue
		}
	}

	var snapshotRepo monitoring.IncidentSnapshotRepository
	var pruneSnapshotRepo monitoring.Prunable
	if useMongoPhase0 && mongoClient != nil {
		mongoSnap, err := repositories.NewMongoSnapshotRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo snapshot repository: %v", err)
		}
		snapshotRepo = mongoSnap
		pruneSnapshotRepo = mongoSnap
		logger.Printf("Snapshot repository: MongoDB (collection %s_incident_snapshots)", mongoPrefix)
	} else {
		fileSnap, err := monitoring.NewFileSnapshotRepository(filepath.Join(dataDir, "incident_snapshots.jsonl"))
		if err != nil {
			logger.Printf("Warning: Failed to init snapshot repository: %v", err)
		} else {
			snapshotRepo = fileSnap
			pruneSnapshotRepo = fileSnap
		}
	}

	// Initialize server metrics repository (for SSH process/metrics history)
	serverMetricsRepo := monitoring.NewServerMetricsRepository(dataDir)
	service.SetServerMetricsRepo(serverMetricsRepo)

	// Initialize MySQL-specific repositories if mysql checks exist
	hasMySQLChecks := false
	var mysqlRepo monitoring.MySQLMetricsRepository
	var pruneMySQLRepo monitoring.Prunable
	for _, check := range cfg.Checks {
		if check.Type == "mysql" {
			hasMySQLChecks = true
			break
		}
	}

	if hasMySQLChecks {
		if useMongoPhase0 && mongoClient != nil {
			mongoMySQL, err := repositories.NewMongoMySQLRepository(mongoClient, mongoDB, mongoPrefix)
			if err != nil {
				logger.Fatalf("init mongo mysql repository: %v", err)
			}
			mysqlRepo = mongoMySQL
			pruneMySQLRepo = mongoMySQL
			logger.Printf("MySQL repository: MongoDB (collections %s_mysql_samples, %s_mysql_deltas)", mongoPrefix, mongoPrefix)
		} else {
			repo, err := mysql.NewFileMySQLRepository(dataDir)
			if err != nil {
				logger.Fatalf("init mysql repository: %v", err)
			}
			mysqlRepo = repo
			pruneMySQLRepo = repo
		}

		sampler := mysql.NewLiveMySQLSampler()
		service.Runner().SetMySQLSampler(sampler)
		service.Runner().SetMySQLRepo(mysqlRepo)

		// Initialize MySQL rule engine with MongoDB if available
		var ruleEngine *monitoring.MySQLRuleEngine
		mongoAvailableForRules := (mongoStore != nil) || (hybridStore != nil && hybridStore.HasMongo() && !hybridStore.IsMongoDown())
		if mongoAvailableForRules && mongoURI != "" {
			alertRuleRepo, err := repositories.NewMongoAlertRuleRepository(mongoURI, mongoDB, mongoPrefix)
			if err != nil {
				logger.Printf("Warning: Failed to init MongoDB alert rule repository: %v", err)
				logger.Printf("Using default MySQL rules with file-based state")
				mysqlRules := monitoring.DefaultMySQLRules()
				ruleEngine, err = monitoring.NewMySQLRuleEngine(mysqlRules, dataDir)
				if err != nil {
					logger.Fatalf("init mysql rule engine: %v", err)
				}
			} else {
				logger.Printf("MongoDB alert rule repository initialized")
				// Seed default MySQL rules if empty
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				mysqlRules := monitoring.DefaultMySQLRules()
				// Check if we need to seed
				rules, err := alertRuleRepo.List(ctx)
				if err != nil || len(rules) == 0 {
					logger.Printf("Seeding default MySQL alert rules to MongoDB")
					for i := range mysqlRules {
						if err := alertRuleRepo.Create(ctx, &mysqlRules[i]); err != nil {
							logger.Printf("Warning: Failed to seed rule %s: %v", mysqlRules[i].ID, err)
						}
					}
				}
				// Load rules from MongoDB for the engine
				loadedRules, err := alertRuleRepo.List(ctx)
				if err != nil {
					logger.Printf("Warning: Failed to load rules from MongoDB: %v", err)
					logger.Printf("Using default MySQL rules")
					loadedRules = mysqlRules
				}
				ruleEngine, err = monitoring.NewMySQLRuleEngine(loadedRules, dataDir)
				if err != nil {
					logger.Fatalf("init mysql rule engine: %v", err)
				}
			}
		} else {
			mysqlRules := monitoring.DefaultMySQLRules()
			ruleEngine, err = monitoring.NewMySQLRuleEngine(mysqlRules, dataDir)
			if err != nil {
				logger.Fatalf("init mysql rule engine: %v", err)
			}
		}

		// Wire rule engine + incident manager + outbox + snapshots into runner
		service.Runner().SetMySQLRuleEngine(ruleEngine)
		service.Runner().SetIncidentManager(incidentManager)
		if outbox != nil {
			service.Runner().SetNotificationOutbox(outbox)
		}
		if snapshotRepo != nil {
			service.Runner().SetSnapshotRepo(snapshotRepo)
		}

		// Set up MySQL API handler
		var auditLogger *monitoring.AuditLogger // service creates its own; we pass nil and let it use the service's
		mysqlAPIHandler := mysql.NewMySQLAPIHandler(mysqlRepo, snapshotRepo, outbox, aiQueue, auditLogger, cfg)
		service.SetMySQLRoutes(mysqlAPIHandler)
		service.SetSnapshotRepo(snapshotRepo)

		logger.Printf("MySQL monitoring enabled")
	}

	// Initialize retention job
	retentionCfg := monitoring.DefaultRetentionConfig()
	retentionJob := monitoring.NewRetentionJob(retentionCfg, logger)
	if pruneSnapshotRepo != nil {
		retentionJob.Register("snapshots", pruneSnapshotRepo, retentionCfg.SnapshotRetentionDays)
	}
	if pruneMySQLRepo != nil {
		retentionJob.Register("mysql_metrics", pruneMySQLRepo, retentionCfg.SnapshotRetentionDays)
	}
	if pruneOutbox != nil {
		retentionJob.Register("notifications", pruneOutbox, retentionCfg.NotificationRetentionDays)
	}
	if pruneAIQueue != nil {
		retentionJob.Register("ai_queue", pruneAIQueue, retentionCfg.AIQueueRetentionDays)
	}
	// Prune resolved incidents to prevent unbounded memory/collection growth
	if pruneIncidentRepo != nil {
		retentionJob.Register("incidents", pruneIncidentRepo, retentionCfg.IncidentRetentionDays)
	}
	// Prune old server metric snapshots
	retentionJob.Register("server_metrics", serverMetricsRepo, retentionCfg.SnapshotRetentionDays)

	// Initialize BYOK AI service with MongoDB if available, otherwise file-based
	var aiConfigStore ai.AIConfigStoreInterface
	mongoAvailableForAI := (mongoStore != nil) || (hybridStore != nil && hybridStore.HasMongo() && !hybridStore.IsMongoDown())
	if mongoAvailableForAI && mongoURI != "" {
		mongoAIConfigRepo, err := airepositories.NewMongoAIConfigRepository(airepositories.MongoAIConfigRepositoryConfig{
			MongoURI:       mongoURI,
			DatabaseName:   mongoDB,
			CollectionName: mongoPrefix + "_ai_config",
			DataDir:        dataDir,
			RetentionDays:  cfg.RetentionDays,
		})
		if err != nil {
			logger.Printf("Warning: Failed to init MongoDB AI config repository: %v", err)
			logger.Printf("Falling back to file-based AI config store")
			aiConfigStore, err = ai.NewAIConfigStore(dataDir)
			if err != nil {
				logger.Printf("Warning: Failed to init file-based AI config store: %v", err)
			}
		} else {
			logger.Printf("MongoDB AI config repository initialized")
			aiConfigStore = airepositories.NewMongoAIConfigStoreAdapter(mongoAIConfigRepo)
		}
	} else {
		aiConfigStore, err = ai.NewAIConfigStore(dataDir)
		if err != nil {
			logger.Printf("Warning: Failed to init AI config store: %v", err)
		}
	}
	var aiService *ai.AIService
	if aiConfigStore != nil && aiQueue != nil {
		aiService = ai.NewAIService(aiConfigStore, aiQueue, incidentRepo, snapshotRepo, store, logger)
		aiService.StartWorker()
		defer aiService.StopWorker()

		aiAPIHandler := ai.NewAIAPIHandler(aiService, aiConfigStore, nil, cfg)
		if mysqlRepo != nil {
			aiAPIHandler.SetMySQLRepo(mysqlRepo)
		}
		service.SetAIRoutes(aiAPIHandler)

		// Wire auto-analysis + notifications: trigger when incidents are created
		incidentManager.SetOnIncidentCreated(func(incident monitoring.Incident) {
			if err := aiService.EnqueueIncidentAnalysis(incident.ID); err != nil {
				logger.Printf("AI: failed to enqueue analysis for incident %s: %v", incident.ID, err)
			}
			if notificationDispatcher != nil {
				channelIDs := lookupCheckChannelIDs(store, incident.CheckID)
				notificationDispatcher.NotifyIncident(incident, nil, channelIDs...)
			}
		})

		logger.Printf("BYOK AI service initialized (background worker active)")
	}

	// If AI not configured, still wire notification dispatch for incidents
	if aiConfigStore == nil || aiQueue == nil {
		if notificationDispatcher != nil {
			incidentManager.SetOnIncidentCreated(func(incident monitoring.Incident) {
				channelIDs := lookupCheckChannelIDs(store, incident.CheckID)
				notificationDispatcher.NotifyIncident(incident, nil, channelIDs...)
			})
		}
	}

	// Wire resolution notifications (always, regardless of AI config)
	if notificationDispatcher != nil {
		incidentManager.SetOnIncidentResolved(func(incident monitoring.Incident) {
			channelIDs := lookupCheckChannelIDs(store, incident.CheckID)
			notificationDispatcher.NotifyResolved(incident, nil, channelIDs...)
		})
	}

	// --- Log Intelligence ---
	logRepo, err := logs.NewFileRepository(dataDir)
	if err != nil {
		logger.Printf("Warning: Failed to init log repository: %v", err)
	} else {
		var logCategorizer *logs.Categorizer
		if aiConfigStore != nil {
			// Create a bridge provider for log categorization that uses the AI service
			logCategorizer = logs.NewCategorizer(logRepo, nil, logger) // Provider set later when AI is ready
		}
		logAPIHandler := logs.NewAPIHandler(logRepo, logCategorizer, logger)
		service.SetLogRoutes(logAPIHandler)
		logger.Printf("Log intelligence initialized (data dir: %s/logs)", dataDir)
	}

	// --- Root-Cause Analysis ---
	{
		signalSource := rca.NewStoreSignalSource(store)
		collector := rca.NewCollector(signalSource)

		var rcaProvider rca.AIProvider
		if aiService != nil {
			rcaProvider = rca.NewAIServiceBridge(aiService.CallProvider)
		}

		rcaAnalyzer, err := rca.NewAnalyzer(collector, rcaProvider, dataDir, logger)
		if err != nil {
			logger.Printf("Warning: Failed to init RCA analyzer: %v", err)
		} else {
			rcaAPIHandler := rca.NewAPIHandler(
				rcaAnalyzer,
				rca.IncidentLookup(incidentRepo),
				nil, // log families lookup — can be wired later
				logger,
			)
			service.SetRCARoutes(rcaAPIHandler)
			logger.Printf("Root-cause analysis initialized (data dir: %s)", dataDir)
		}
	}

	// --- Evidence Backbone & AI Incident Brief (Phase 1) ---
	{
		registry := evidence.NewRegistry()
		registry.Register(evidence.NewCheckProvider(store))
		if snapshotRepo != nil {
			registry.Register(evidence.NewMySQLSnapshotProvider(snapshotRepo))
		}
		if mysqlRepo != nil {
			registry.Register(evidence.NewMySQLProvider(mysqlRepo))
		}
		registry.Register(evidence.NewServerMetricsProvider(serverMetricsRepo, store))
		registry.Register(evidence.NewIncidentHistoryProvider(incidentRepo))

		// Audit provider — create an audit repository for read-only evidence collection
		auditRepo, auditErr := monitoring.NewFileAuditRepository(filepath.Join(dataDir, "audit.jsonl"))
		if auditErr != nil {
			logger.Printf("Warning: Failed to init audit repository for evidence: %v", auditErr)
		} else {
			registry.Register(evidence.NewAuditProvider(auditRepo))
		}

		contextBuilder := evidence.NewContextBuilder(registry, logger)
		briefGenerator := evidence.NewBriefGenerator(contextBuilder, incidentRepo, logger)

		// Wire AI provider if available
		if aiService != nil {
			briefGenerator.SetAICall(func(ctx context.Context, systemMsg, userMsg string) (string, error) {
				return aiService.CallProvider(ctx, systemMsg, userMsg)
			})
		}

		// MongoDB repositories for evidence persistence
		var briefRepo *evidence.BriefRepository
		var signalEventRepo *evidence.SignalEventRepository
		var incidentEventRepo *evidence.IncidentEventRepository
		if mongoClient != nil {
			var err error
			briefRepo, err = evidence.NewBriefRepository(mongoClient, mongoDB, mongoPrefix)
			if err != nil {
				logger.Printf("Warning: Failed to init evidence brief repository: %v", err)
			}
			signalEventRepo, err = evidence.NewSignalEventRepository(mongoClient, mongoDB, mongoPrefix)
			if err != nil {
				logger.Printf("Warning: Failed to init signal event repository: %v", err)
			}
			incidentEventRepo, err = evidence.NewIncidentEventRepository(mongoClient, mongoDB, mongoPrefix)
			if err != nil {
				logger.Printf("Warning: Failed to init incident event repository: %v", err)
			}
		}

		evidenceAPIHandler := evidence.NewAPIHandler(briefGenerator, briefRepo, incidentEventRepo, signalEventRepo)
		service.SetEvidenceRoutes(evidenceAPIHandler)
		logger.Printf("Evidence backbone initialized (%d providers registered)", len(registry.Categories()))
	}

	// --- Natural-Language Ops Assistant (Phase 4) ---
	{
		var assistantAICall assistant.AIProvider
		if aiService != nil {
			assistantAICall = aiService.CallProvider
		}
		assistantHandler := assistant.NewHandler(store, incidentRepo, assistantAICall, logger)
		service.SetAssistantRoutes(assistantHandler)
		logger.Printf("NL Ops Assistant initialized (AI available: %v)", assistantAICall != nil)
	}

	// --- Tuning & Recommendations (Phase 5) ---
	{
		var recsAICall recommendations.AIProvider
		if aiService != nil {
			recsAICall = aiService.CallProvider
		}
		recsHandler := recommendations.NewHandler(store, incidentRepo, recsAICall, logger)
		service.SetRecommendationRoutes(recsHandler)
		logger.Printf("Recommendations engine initialized (AI available: %v)", recsAICall != nil)
	}

	// --- Assisted Automation (Phase 6) ---
	{
		var autoAICall automation.AIProvider
		if aiService != nil {
			autoAICall = aiService.CallProvider
		}
		autoHandler := automation.NewHandler(store, incidentRepo, autoAICall, logger)
		service.SetAutomationRoutes(autoHandler)
		logger.Printf("Automation engine initialized (AI available: %v)", autoAICall != nil)
	}

	stopRetention := make(chan struct{})
	retentionJob.RunDaily(stopRetention)
	defer close(stopRetention)

	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("service stopped: %v", err)
	}
}

// lookupCheckChannelIDs returns the NotificationChannelIDs configured on a check.
func lookupCheckChannelIDs(store monitoring.Store, checkID string) []string {
	state := store.Snapshot()
	for _, c := range state.Checks {
		if c.ID == checkID {
			return c.NotificationChannelIDs
		}
	}
	return nil
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

// monitorMongoDB periodically pings MongoDB and creates/resolves incidents.
func monitorMongoDB(store *monitoring.HybridStore, im *monitoring.IncidentManager, logger *log.Logger, stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	const checkID = "internal-mongodb"
	const checkName = "MongoDB Primary Database"
	wasDown := store.IsMongoDown()

	// If already down at startup, create an incident immediately
	if wasDown {
		_ = im.ProcessAlert(checkID, checkName, "internal", "critical",
			"MongoDB is unreachable — operating in local file fallback mode",
			map[string]string{"component": "mongodb"},
		)
		logger.Printf("INCIDENT: MongoDB unreachable at startup — running on local file fallback")
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := store.PingMongo(ctx)
			cancel()

			isDown := err != nil
			if isDown && !wasDown {
				// MongoDB just went down — create incident
				_ = im.ProcessAlert(checkID, checkName, "internal", "critical",
					"MongoDB is unreachable — operating in local file fallback mode: "+err.Error(),
					map[string]string{"component": "mongodb"},
				)
				logger.Printf("INCIDENT: MongoDB connectivity lost: %v", err)
			} else if !isDown && wasDown {
				// MongoDB recovered — auto-resolve
				_ = im.AutoResolveOnRecovery(checkID)
				logger.Printf("RESOLVED: MongoDB connectivity restored")
			}
			wasDown = isDown
		}
	}
}
