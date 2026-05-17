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

	"health-ops/backend/internal/monitoring"
	"health-ops/backend/internal/monitoring/ai"
	airepositories "health-ops/backend/internal/monitoring/ai/repositories"
	"health-ops/backend/internal/monitoring/assistant"
	"health-ops/backend/internal/monitoring/automation"
	"health-ops/backend/internal/monitoring/cryptoutil"
	"health-ops/backend/internal/monitoring/evidence"
	"health-ops/backend/internal/monitoring/logs"
	"health-ops/backend/internal/monitoring/mysql"
	"health-ops/backend/internal/monitoring/notify"
	"health-ops/backend/internal/monitoring/rca"
	"health-ops/backend/internal/monitoring/recommendations"
	"health-ops/backend/internal/monitoring/remediation"
	"health-ops/backend/internal/monitoring/repositories"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	logger := log.New(os.Stdout, "healthops ", log.LstdFlags|log.Lmicroseconds)

	// Load .env file if it exists.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Printf("Warning: .env file not found or error loading: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configPath := resolvePath("CONFIG_PATH", filepath.Join("backend", "config", "default.json"), filepath.Join("config", "default.json"))
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		logger.Fatalf("MONGODB_URI is required")
	}
	mongoDB := envOrDefault("MONGODB_DATABASE", "healthops")
	mongoPrefix := envOrDefault("MONGODB_COLLECTION_PREFIX", "healthops")

	cfg, err := monitoring.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	var store monitoring.Store

	// Initialize MongoDB client — required for all persistence.
	clientOpts := options.Client().
		ApplyURI(mongoURI).
		SetServerSelectionTimeout(10 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetMaxPoolSize(100)

	mongoClient, connErr := mongo.Connect(clientOpts)
	if connErr != nil {
		logger.Fatalf("Failed to connect to MongoDB: %v", connErr)
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if pingErr := mongoClient.Ping(pingCtx, nil); pingErr != nil {
		pingCancel()
		_ = mongoClient.Disconnect(context.Background())
		logger.Fatalf("MongoDB ping failed: %v", pingErr)
	}
	pingCancel()
	logger.Printf("MongoDB connection established")

	// Initialize data dir and secrets encryption BEFORE MongoStore so seed checks
	// can have their plaintext passwords encrypted before being written to MongoDB.
	dataDir := resolvePath("DATA_DIR", filepath.Join("backend", "data"), "data")
	monitoring.InitJWTSecret(dataDir)
	if err := cryptoutil.Init(dataDir); err != nil {
		logger.Fatalf("init secrets encryption: %v", err)
	}

	mongoStore, err := monitoring.NewMongoStore(mongoClient, mongoDB, mongoPrefix, cfg.RetentionDays, cfg.Checks, logger)
	if err != nil {
		logger.Fatalf("init mongo store: %v", err)
	}
	store = mongoStore
	logger.Printf("MongoDB persistence initialized")

	service := monitoring.NewService(cfg, store, logger)

	// Set demo mode based on config file name
	if strings.Contains(configPath, "demo.json") {
		service.SetDemoMode(true)
		logger.Printf("Demo mode enabled")
	}

	// /healthz must fail when MongoDB is unreachable
	mongoHealthCheck := func(ctx context.Context) error {
		return mongoClient.Ping(ctx, nil)
	}
	service.SetMongoHealthCheck(mongoHealthCheck)
	service.SetDegradedHealthCheck(mongoHealthCheck)

	serverRepo, err := monitoring.NewMongoServerRepositoryFromClient(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init MongoDB server repository: %v", err)
	}
	serverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := serverRepo.SeedIfEmpty(serverCtx, cfg.Servers); err != nil {
		cancel()
		logger.Fatalf("seed MongoDB server repository: %v", err)
	}
	servers, srvErr := serverRepo.List(serverCtx)
	cancel()
	if srvErr != nil {
		logger.Fatalf("load MongoDB server repository: %v", srvErr)
	}
	cfg.Servers = servers
	service.SetServerRepo(serverRepo)
	logger.Printf("Server repository: MongoDB (collection %s_servers, %d servers)", mongoPrefix, len(servers))

	mongoUserRepo, err := repositories.NewMongoUserRepository(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init MongoDB user repository: %v", err)
	}
	bootstrapPassword := os.Getenv("HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD")
	bootstrapEmail := envOrDefault("HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL", "admin@healthops.local")
	bootstrapReset := envOrDefault("HEALTHOPS_BOOTSTRAP_ADMIN_RESET", "false") == "true"
	if bootstrapPassword != "" {
		bootstrapCtx, bCancel := context.WithTimeout(context.Background(), 5*time.Second)
		changed, err := mongoUserRepo.BootstrapAdmin(bootstrapCtx, bootstrapPassword, bootstrapEmail, bootstrapReset)
		bCancel()
		if err != nil {
			logger.Fatalf("bootstrap MongoDB admin user: %v", err)
		} else if changed {
			logger.Printf("MongoDB admin bootstrap applied")
		}
	} else {
		adminCtx, aCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := mongoUserRepo.FindByUsername(adminCtx, "admin")
		aCancel()
		if err != nil {
			logger.Fatalf("HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD is required on first startup when no MongoDB admin user exists")
		}
	}
	service.SetUserStore(repositories.NewUserStoreAdapter(mongoUserRepo))
	logger.Printf("User management initialized (MongoDB)")

	// Initialize incident manager (MongoDB-backed)
	var incidentRepo monitoring.IncidentRepository
	var pruneIncidentRepo monitoring.Prunable
	mongoIncRepo, err := repositories.NewMongoIncidentRepository(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo incident repository: %v", err)
	}
	incidentRepo = mongoIncRepo
	pruneIncidentRepo = mongoIncRepo
	logger.Printf("Incident repository: MongoDB (collection %s_incidents)", mongoPrefix)
	incidentManager := monitoring.NewIncidentManager(incidentRepo, logger)
	service.SetIncidentManager(incidentManager)

	// Forward-declare remediation engine so incident callbacks can reference it.
	// Actual initialization happens later in the "Auto-Remediation Engine" block.
	var remediationEngine *remediation.Engine

	// Initialize heartbeat API (unauthenticated ping endpoints for cron jobs)
	heartbeatAPI := monitoring.NewHeartbeatAPIHandler()
	service.SetHeartbeatAPI(heartbeatAPI)
	logger.Printf("Heartbeat monitoring initialized (ping endpoint: /api/v1/heartbeats/{token})")

	// Initialize embedded help/documentation API (public, no auth)
	helpAPI := monitoring.NewHelpAPIHandler()
	service.SetHelpAPI(helpAPI)
	logger.Printf("Help content initialized (browse at /api/v1/help/topics)")

	// Initialize maintenance windows (MongoDB)
	mongoMaint, err := repositories.NewMongoMaintenanceRepository(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo maintenance repository: %v", err)
	}
	service.SetMaintenanceStore(mongoMaint)
	logger.Printf("Maintenance windows: MongoDB (collection %s_maintenance_windows)", mongoPrefix)

	// Initialize custom dashboard builder (MongoDB)
	mongoDash, err := repositories.NewMongoCustomDashboardRepository(mongoClient.Database(mongoDB), mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo custom dashboard repository: %v", err)
	}
	service.SetCustomDashboardStore(mongoDash)
	logger.Printf("Custom dashboards: MongoDB (collection %s_custom_dashboards)", mongoPrefix)

	// Initialize status pages (MongoDB)
	mongoSP, err := repositories.NewMongoStatusPageRepository(mongoClient.Database(mongoDB), mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo status page repository: %v", err)
	}
	service.SetStatusPageStore(mongoSP)
	logger.Printf("Status pages: MongoDB (collection %s_status_pages)", mongoPrefix)

	// Initialize AI chat (provider wired later after AI service init)
	mongoChat, err := repositories.NewMongoChatRepository(mongoClient.Database(mongoDB), mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo chat repository: %v", err)
	}
	var chatStore monitoring.ChatRepository = mongoChat
	service.SetAIChatStore(chatStore, nil)
	logger.Printf("AI chat: MongoDB (collection %s_chat_conversations)", mongoPrefix)

	// Initialize notification outbox (MongoDB)
	var outbox monitoring.NotificationOutboxRepository
	var pruneOutbox monitoring.Prunable
	mongoOutbox, err := repositories.NewMongoNotificationOutbox(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo notification outbox: %v", err)
	}
	outbox = mongoOutbox
	pruneOutbox = mongoOutbox
	logger.Printf("Notification outbox: MongoDB (collection %s_notification_outbox)", mongoPrefix)

	// Initialize notification channel store and dispatcher (MongoDB)
	var channelStore notify.ChannelStore
	mongoChannelRepo, err := repositories.NewMongoChannelRepository(mongoURI, mongoDB, mongoPrefix, 5)
	if err != nil {
		logger.Fatalf("init MongoDB channel repository: %v", err)
	}
	channelStore = repositories.NewChannelStoreAdapter(mongoChannelRepo)
	logger.Printf("Notification channel repository: MongoDB")

	var notificationDispatcher *notify.NotificationDispatcher
	if channelStore != nil {
		notificationDispatcher = notify.NewNotificationDispatcher(channelStore, outbox, logger)
		defer notificationDispatcher.Stop() // flush pending notifications on shutdown
		// Set dashboard URL for email links.
		// HEALTHOPS_PUBLIC_URL takes precedence (set this in production).
		dashURL := os.Getenv("HEALTHOPS_PUBLIC_URL")
		if dashURL == "" {
			dashURL = cfg.Server.Addr
			if dashURL == "" || dashURL == ":8080" {
				dashURL = "http://localhost:8080"
			}
			logger.Printf("WARNING: HEALTHOPS_PUBLIC_URL is not set — email notification links will point to %s (unreachable from outside)", dashURL)
		}
		notificationDispatcher.SetDashboardURL(dashURL)
		notificationAPIHandler := notify.NewNotificationAPIHandler(channelStore, notificationDispatcher, cfg)
		service.SetNotifyRoutes(notificationAPIHandler)
		logger.Printf("Notification channels initialized")
	}

	// Initialize AI queue (MongoDB)
	var aiQueue monitoring.AIQueueRepository
	var pruneAIQueue monitoring.Prunable
	mongoQueue, err := airepositories.NewMongoAIQueue(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo ai queue: %v", err)
	}
	aiQueue = mongoQueue
	pruneAIQueue = mongoQueue
	logger.Printf("AI queue: MongoDB (collections %s_ai_queue, %s_ai_results)", mongoPrefix, mongoPrefix)

	// Initialize snapshot repository (MongoDB)
	var snapshotRepo monitoring.IncidentSnapshotRepository
	var pruneSnapshotRepo monitoring.Prunable
	mongoSnap, err := repositories.NewMongoSnapshotRepository(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo snapshot repository: %v", err)
	}
	snapshotRepo = mongoSnap
	pruneSnapshotRepo = mongoSnap
	logger.Printf("Snapshot repository: MongoDB (collection %s_incident_snapshots)", mongoPrefix)

	// Initialize server metrics repository (MongoDB)
	mongoSM, err := repositories.NewMongoServerMetricsRepository(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo server metrics repository: %v", err)
	}
	var serverMetricsRepo monitoring.ServerMetricsStore = mongoSM
	logger.Printf("Server metrics: MongoDB (collection %s_server_metrics)", mongoPrefix)
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
		mongoMySQL, err := repositories.NewMongoMySQLRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo mysql repository: %v", err)
		}
		mysqlRepo = mongoMySQL
		pruneMySQLRepo = mongoMySQL
		logger.Printf("MySQL repository: MongoDB (collections %s_mysql_samples, %s_mysql_deltas)", mongoPrefix, mongoPrefix)

		sampler := mysql.NewLiveMySQLSampler()
		service.Runner().SetMySQLSampler(sampler)
		service.Runner().SetMySQLRepo(mysqlRepo)

		// Initialize MySQL rule engine (MongoDB)
		var ruleEngine *monitoring.MySQLRuleEngine
		alertRuleRepo, err := repositories.NewMongoAlertRuleRepository(mongoURI, mongoDB, mongoPrefix)
		if err != nil {
			logger.Printf("Warning: Failed to init MongoDB alert rule repository: %v", err)
			mysqlRules := monitoring.DefaultMySQLRules()
			ruleEngine, err = monitoring.NewMySQLRuleEngine(mysqlRules, dataDir)
			if err != nil {
				logger.Fatalf("init mysql rule engine: %v", err)
			}
		} else {
			logger.Printf("MongoDB alert rule repository initialized")
			// Seed default MySQL rules if empty
			ruleCtx, ruleCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer ruleCancel()

			mysqlRules := monitoring.DefaultMySQLRules()
			rules, err := alertRuleRepo.List(ruleCtx)
			if err != nil || len(rules) == 0 {
				logger.Printf("Seeding default MySQL alert rules to MongoDB")
				for i := range mysqlRules {
					if err := alertRuleRepo.Create(ruleCtx, &mysqlRules[i]); err != nil {
						logger.Printf("Warning: Failed to seed rule %s: %v", mysqlRules[i].ID, err)
					}
				}
			}
			loadedRules, err := alertRuleRepo.List(ruleCtx)
			if err != nil {
				logger.Printf("Warning: Failed to load rules from MongoDB: %v", err)
				loadedRules = mysqlRules
			}
			ruleEngine, err = monitoring.NewMySQLRuleEngine(loadedRules, dataDir)
			if err != nil {
				logger.Fatalf("init mysql rule engine: %v", err)
			}
		}

		// Wire rule engine + incident manager + outbox + snapshots into runner
		service.Runner().SetMySQLRuleEngine(ruleEngine)
		service.Runner().SetIncidentManager(incidentManager)
		service.Runner().SetNotificationOutbox(outbox)
		service.Runner().SetSnapshotRepo(snapshotRepo)

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
	retentionJob.Register("snapshots", pruneSnapshotRepo, retentionCfg.SnapshotRetentionDays)
	if pruneMySQLRepo != nil {
		retentionJob.Register("mysql_metrics", pruneMySQLRepo, retentionCfg.SnapshotRetentionDays)
	}
	retentionJob.Register("notifications", pruneOutbox, retentionCfg.NotificationRetentionDays)
	retentionJob.Register("ai_queue", pruneAIQueue, retentionCfg.AIQueueRetentionDays)
	retentionJob.Register("incidents", pruneIncidentRepo, retentionCfg.IncidentRetentionDays)
	retentionJob.Register("server_metrics", serverMetricsRepo, retentionCfg.SnapshotRetentionDays)

	// Initialize BYOK AI service (MongoDB)
	var aiConfigStore ai.AIConfigStoreInterface
	mongoAIConfigRepo, err := airepositories.NewMongoAIConfigRepository(airepositories.MongoAIConfigRepositoryConfig{
		MongoURI:       mongoURI,
		DatabaseName:   mongoDB,
		CollectionName: mongoPrefix + "_ai_config",
		DataDir:        dataDir,
		RetentionDays:  cfg.RetentionDays,
	})
	if err != nil {
		logger.Fatalf("init MongoDB AI config repository: %v", err)
	}
	aiConfigStore = airepositories.NewMongoAIConfigStoreAdapter(mongoAIConfigRepo)
	logger.Printf("AI config repository: MongoDB")
	var aiService *ai.AIService
	if aiConfigStore != nil && aiQueue != nil {
		if err := bootstrapAIProviderFromEnv(aiConfigStore, logger); err != nil {
			logger.Printf("Warning: Failed to bootstrap AI provider from environment: %v", err)
		}

		aiService = ai.NewAIService(aiConfigStore, aiQueue, incidentRepo, snapshotRepo, store, logger)
		aiService.StartWorker()
		defer aiService.StopWorker()

		aiAPIHandler := ai.NewAIAPIHandler(aiService, aiConfigStore, nil, cfg)
		if mysqlRepo != nil {
			aiAPIHandler.SetMySQLRepo(mysqlRepo)
		}
		service.SetAIRoutes(aiAPIHandler)

		// Wire auto-analysis + notifications + remediation: trigger when incidents are created
		incidentManager.SetOnIncidentCreated(func(incident monitoring.Incident) {
			if err := aiService.EnqueueIncidentAnalysis(incident.ID); err != nil {
				logger.Printf("AI: failed to enqueue analysis for incident %s: %v", incident.ID, err)
			}
			if notificationDispatcher != nil {
				channelIDs := lookupCheckChannelIDs(store, incident.CheckID)
				notificationDispatcher.NotifyIncident(incident, nil, channelIDs...)
			}
			// Trigger auto-remediation if engine is wired and check has remediation config
			if remediationEngine != nil {
				triggerRemediation(remediationEngine, store, incident, logger)
			}
		})

		logger.Printf("BYOK AI service initialized (background worker active)")
	}

	// If AI not configured, still wire notification dispatch + remediation for incidents
	if aiConfigStore == nil || aiQueue == nil {
		if notificationDispatcher != nil {
			incidentManager.SetOnIncidentCreated(func(incident monitoring.Incident) {
				channelIDs := lookupCheckChannelIDs(store, incident.CheckID)
				notificationDispatcher.NotifyIncident(incident, nil, channelIDs...)
				if remediationEngine != nil {
					triggerRemediation(remediationEngine, store, incident, logger)
				}
			})
		} else {
			// No notifications, no AI — still try remediation
			incidentManager.SetOnIncidentCreated(func(incident monitoring.Incident) {
				if remediationEngine != nil {
					triggerRemediation(remediationEngine, store, incident, logger)
				}
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
	mongoLogRepo, err := logs.NewMongoRepository(mongoClient, mongoDB, mongoPrefix)
	if err != nil {
		logger.Fatalf("init mongo log repository: %v", err)
	}
	var logRepo logs.Repository = mongoLogRepo
	logger.Printf("Log intelligence: MongoDB")
	{
		var logCategorizer *logs.Categorizer
		if aiConfigStore != nil {
			var catProvider logs.AIProvider
			if aiService != nil {
				catProvider = logs.AIProviderFunc(aiService.CallProvider)
			}
			logCategorizer = logs.NewCategorizer(logRepo, catProvider, logger)
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

		var rcaRepo rca.ReportRepository
		mongoRCARepo, err := rca.NewMongoReportRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo rca repository: %v", err)
		}
		rcaRepo = mongoRCARepo
		logger.Printf("RCA reports: MongoDB")

		rcaAnalyzer, err := rca.NewAnalyzer(collector, rcaProvider, rcaRepo, logger)
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
			logger.Printf("Root-cause analysis initialized")
		}
	}

	// --- Evidence Backbone & AI Incident Brief ---
	{
		registry := evidence.NewRegistry()
		registry.Register(evidence.NewCheckProvider(store))
		registry.Register(evidence.NewMySQLSnapshotProvider(snapshotRepo))
		if mysqlRepo != nil {
			registry.Register(evidence.NewMySQLProvider(mysqlRepo))
		}
		registry.Register(evidence.NewServerMetricsProvider(serverMetricsRepo, store))
		registry.Register(evidence.NewIncidentHistoryProvider(incidentRepo))

		// Audit provider (MongoDB)
		mongoAuditRepo, err := repositories.NewMongoAuditRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Fatalf("init mongo audit repository: %v", err)
		}
		var auditRepo monitoring.AuditRepository = mongoAuditRepo
		logger.Printf("Audit repository: MongoDB (collection %s_audit)", mongoPrefix)
		registry.Register(evidence.NewAuditProvider(auditRepo))

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

	// --- Auto-Remediation Engine ---
	{
		remRepo, err := repositories.NewMongoRemediationRepository(mongoClient, mongoDB, mongoPrefix)
		if err != nil {
			logger.Printf("WARNING: remediation repository init failed: %v", err)
		} else {
			remediationEngine = remediation.NewEngine(remRepo, logger)

			// Wire AI provider for failure analysis
			if aiService != nil {
				remediationEngine.SetAIProvider(func(systemMsg, userMsg string) (string, error) {
					return aiService.CallProvider(context.Background(), systemMsg, userMsg)
				})
			}

			// Wire check resolver for manual remediation
			remediationEngine.SetCheckResolver(func(checkID string) (remediation.CheckInfo, error) {
				state := store.Snapshot()
				for _, c := range state.Checks {
					if c.ID == checkID {
						info := remediation.CheckInfo{
							CheckID:   c.ID,
							CheckName: c.Name,
							CheckType: c.Type,
							Target:    c.Target,
							ServerID:  c.ServerId,
						}
						if c.SSH != nil {
							info.SSH = &remediation.SSHConfig{
								Host:               c.SSH.Host,
								Port:               c.SSH.Port,
								User:               c.SSH.User,
								KeyPath:            c.SSH.KeyPath,
								KeyEnv:             c.SSH.KeyEnv,
								Password:           c.SSH.Password,
								PasswordEnc:        c.SSH.PasswordEnc,
								PasswordEnv:        c.SSH.PasswordEnv,
								HostKeyFingerprint: c.SSH.HostKeyFingerprint,
							}
						}
						if c.Remediation != nil {
							// Remediation-specific SSH target overrides the check's own SSH
							// (used when a non-SSH check needs to restart a service on a
							// remote server).
							if c.Remediation.SSH != nil && c.Remediation.SSH.Host != "" {
								info.SSH = sshConfigFromMonitoring(c.Remediation.SSH)
							}
							info.Ref = remediation.RemediationRef{
								ActionRef:                   c.Remediation.ActionRef,
								MaxAttempts:                 c.Remediation.MaxAttempts,
								CooldownSeconds:             c.Remediation.CooldownSeconds,
								ConsecutiveFailuresRequired: c.Remediation.ConsecutiveFailuresRequired,
								VerifyAfterSeconds:          c.Remediation.VerifyAfterSeconds,
								NotifyOnRemediation:         c.Remediation.NotifyOnRemediation,
								EscalateOnExhaustion:        c.Remediation.EscalateOnExhaustion,
							}
							info.InlineAction = buildInlineAction(c)
						}
						return info, nil
					}
				}
				return remediation.CheckInfo{}, errors.New("check not found")
			})

			remHandler := remediation.NewHandler(remediationEngine, logger)
			service.SetRemediationRoutes(remHandler)

			// Wire auto-resolve on successful remediation
			remediationEngine.SetOnSuccess(func(checkID, incidentID, actionName, attemptID string) {
				logger.Printf("[remediation] auto-resolving incident %s for check %s (action: %s)", incidentID, checkID, actionName)
				if err := incidentManager.ResolveIncident(incidentID, "auto-remediation"); err != nil {
					logger.Printf("[remediation] failed to auto-resolve incident %s: %v", incidentID, err)
				}
			})

			// Seed demo remediation actions if none exist
			existing, _ := remRepo.ListActions()
			if len(existing) == 0 {
				seedActions := []remediation.AllowedAction{
					{
						ID:             "restart-nginx",
						Name:           "Restart Nginx",
						Type:           remediation.ActionSSHCommand,
						Command:        "service nginx restart",
						TimeoutSeconds: 30,
						Risk:           remediation.RiskLow,
						Description:    "Restarts the Nginx web server via SSH",
					},
					{
						ID:             "restart-python-http",
						Name:           "Restart Python HTTP Server",
						Type:           remediation.ActionSSHCommand,
						Command:        "pkill -f 'http.server' && nohup python3 -m http.server 8000 &",
						TimeoutSeconds: 15,
						Risk:           remediation.RiskLow,
						Description:    "Kills and restarts the Python HTTP server",
					},
					{
						ID:             "restart-cron",
						Name:           "Restart Cron Daemon",
						Type:           remediation.ActionSSHCommand,
						Command:        "service cron restart",
						TimeoutSeconds: 15,
						Risk:           remediation.RiskLow,
						Description:    "Restarts the cron daemon via SSH",
					},
					{
						ID:             "restart-mysql",
						Name:           "Restart MySQL",
						Type:           remediation.ActionSSHCommand,
						Command:        "service mysql restart",
						TimeoutSeconds: 60,
						Risk:           remediation.RiskMedium,
						Description:    "Restarts the MySQL database server",
					},
				}
				for _, a := range seedActions {
					if err := remRepo.CreateAction(a); err != nil {
						logger.Printf("[remediation] failed to seed action %s: %v", a.ID, err)
					}
				}
				// Enable engine with dry-run=false for demo
				_ = remRepo.SaveConfig(remediation.GlobalConfig{
					Enabled:          true,
					DryRun:           false,
					MaxConcurrent:    2,
					OutputLimitBytes: 8192,
				})
				logger.Printf("Seeded %d demo remediation actions", len(seedActions))
			}

			logger.Printf("Remediation engine initialized (AI available: %v)", aiService != nil)
		}
	}

	// Wire AI provider into chat handler now that AI service is available
	if aiService != nil && chatStore != nil {
		service.SetAIChatStore(chatStore, monitoring.ChatProvider(aiService.CallProvider))
		logger.Printf("AI chat provider connected")
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

// triggerRemediation builds CheckInfo from the store and fires the remediation engine.
func triggerRemediation(engine *remediation.Engine, store monitoring.Store, incident monitoring.Incident, logger *log.Logger) {
	state := store.Snapshot()
	for _, c := range state.Checks {
		if c.ID != incident.CheckID {
			continue
		}
		if c.Remediation == nil {
			return // no remediation configured for this check
		}
		// Allow either inline (Type+Command set) or registry reference (ActionRef set)
		if c.Remediation.ActionRef == "" && c.Remediation.Type == "" {
			return
		}

		info := remediation.CheckInfo{
			CheckID:    c.ID,
			CheckName:  c.Name,
			CheckType:  c.Type,
			Target:     c.Target,
			ServerID:   c.ServerId,
			IncidentID: incident.ID,
			Ref: remediation.RemediationRef{
				ActionRef:                   c.Remediation.ActionRef,
				MaxAttempts:                 c.Remediation.MaxAttempts,
				CooldownSeconds:             c.Remediation.CooldownSeconds,
				ConsecutiveFailuresRequired: c.Remediation.ConsecutiveFailuresRequired,
				VerifyAfterSeconds:          c.Remediation.VerifyAfterSeconds,
				NotifyOnRemediation:         c.Remediation.NotifyOnRemediation,
				EscalateOnExhaustion:        c.Remediation.EscalateOnExhaustion,
			},
			InlineAction: buildInlineAction(c),
		}
		if c.SSH != nil {
			info.SSH = &remediation.SSHConfig{
				Host:               c.SSH.Host,
				Port:               c.SSH.Port,
				User:               c.SSH.User,
				KeyPath:            c.SSH.KeyPath,
				KeyEnv:             c.SSH.KeyEnv,
				Password:           c.SSH.Password,
				PasswordEnc:        c.SSH.PasswordEnc,
				PasswordEnv:        c.SSH.PasswordEnv,
				HostKeyFingerprint: c.SSH.HostKeyFingerprint,
			}
		}
		// Remediation-specific SSH target overrides the check's own SSH (used
		// when a non-SSH check needs to restart a service on a remote server).
		if c.Remediation != nil && c.Remediation.SSH != nil && c.Remediation.SSH.Host != "" {
			info.SSH = sshConfigFromMonitoring(c.Remediation.SSH)
		}

		engine.TryRemediate(info)
		return
	}
	logger.Printf("[remediation] check %s not found in store — skipping", incident.CheckID)
}

// sshConfigFromMonitoring converts a monitoring.SSHCheckConfig into the
// engine-facing remediation.SSHConfig (1:1 field copy).
func sshConfigFromMonitoring(s *monitoring.SSHCheckConfig) *remediation.SSHConfig {
	if s == nil {
		return nil
	}
	return &remediation.SSHConfig{
		Host:               s.Host,
		Port:               s.Port,
		User:               s.User,
		KeyPath:            s.KeyPath,
		KeyEnv:             s.KeyEnv,
		Password:           s.Password,
		PasswordEnc:        s.PasswordEnc,
		PasswordEnv:        s.PasswordEnv,
		HostKeyFingerprint: s.HostKeyFingerprint,
	}
}

// buildInlineAction constructs an AllowedAction from a check's inline
// remediation fields. Returns nil if the check uses the registry (ActionRef set)
// or has no inline action configured.
func buildInlineAction(c monitoring.CheckConfig) *remediation.AllowedAction {
	if c.Remediation == nil || c.Remediation.Type == "" {
		return nil
	}
	risk := remediation.RiskLevel(c.Remediation.Risk)
	if risk == "" {
		risk = remediation.RiskLow
	}
	timeout := c.Remediation.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	return &remediation.AllowedAction{
		ID:             "inline-" + c.ID,
		Name:           c.Name + " remediation",
		Type:           remediation.ActionType(c.Remediation.Type),
		Command:        c.Remediation.Command,
		URL:            c.Remediation.URL,
		Method:         c.Remediation.Method,
		Headers:        c.Remediation.Headers,
		TimeoutSeconds: timeout,
		Risk:           risk,
		Description:    c.Remediation.Description,
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

func bootstrapAIProviderFromEnv(store ai.AIConfigStoreInterface, logger *log.Logger) error {
	id := os.Getenv("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_ID")
	if id == "" {
		return nil
	}

	baseURL := os.Getenv("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_BASE_URL")
	model := os.Getenv("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_MODEL")
	if baseURL == "" || model == "" {
		return errors.New("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_BASE_URL and HEALTHOPS_BOOTSTRAP_AI_PROVIDER_MODEL are required")
	}

	providerType := ai.AIProviderType(envOrDefault("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_TYPE", string(ai.AIProviderCustom)))
	name := envOrDefault("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_NAME", id)
	apiKey := os.Getenv("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_API_KEY")
	reset := envOrDefault("HEALTHOPS_BOOTSTRAP_AI_PROVIDER_RESET", "false") == "true"
	now := time.Now().UTC()

	changed := false
	err := store.Update(func(cfg *ai.AIServiceConfig) error {
		cfg.Enabled = true
		cfg.AutoAnalyze = true
		if cfg.MaxConcurrent <= 0 {
			cfg.MaxConcurrent = 2
		}
		if cfg.TimeoutSeconds <= 0 {
			cfg.TimeoutSeconds = 30
		}
		if cfg.RetryCount < 0 {
			cfg.RetryCount = 0
		}
		if cfg.RetryDelayMs <= 0 {
			cfg.RetryDelayMs = 1000
		}

		provider := ai.AIProviderConfig{
			ID:          id,
			Provider:    providerType,
			Name:        name,
			APIKey:      apiKey,
			BaseURL:     baseURL,
			Model:       model,
			MaxTokens:   1200,
			Temperature: 0.2,
			Enabled:     true,
			IsDefault:   true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		for i := range cfg.Providers {
			cfg.Providers[i].IsDefault = false
			if cfg.Providers[i].ID != id {
				continue
			}
			if !reset {
				cfg.Providers[i].IsDefault = true
				return nil
			}
			provider.CreatedAt = cfg.Providers[i].CreatedAt
			if provider.CreatedAt.IsZero() {
				provider.CreatedAt = now
			}
			cfg.Providers[i] = provider
			changed = true
			return nil
		}

		cfg.Providers = append(cfg.Providers, provider)
		changed = true
		return nil
	})
	if err != nil {
		return err
	}
	if changed {
		logger.Printf("AI bootstrap provider applied: %s", id)
	}
	return nil
}
