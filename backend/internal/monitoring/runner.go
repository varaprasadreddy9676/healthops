package monitoring

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Runner struct {
	cfg               *Config
	store             Store
	client            *http.Client
	metrics           *MetricsCollector
	mysqlSampler      MySQLSampler
	mysqlRepo         MySQLMetricsRepository
	mysqlRuleEngine   *MySQLRuleEngine
	incidentManager   *IncidentManager
	outbox            NotificationOutboxRepository
	snapshotRepo      IncidentSnapshotRepository
	serverMetricsRepo *ServerMetricsRepository
	running           bool
	mu                sync.Mutex
}

func NewRunner(cfg *Config, store Store) *Runner {
	return &Runner{
		cfg:     cfg,
		store:   store,
		client:  &http.Client{},
		metrics: nil, // Will be set by SetMetricsCollector
	}
}

// resolveServer looks up a RemoteServer by ID from the config.
// Returns nil if serverId is empty or not found.
func (r *Runner) resolveServer(serverId string) *RemoteServer {
	if serverId == "" {
		return nil
	}
	for i := range r.cfg.Servers {
		if r.cfg.Servers[i].ID == serverId {
			return &r.cfg.Servers[i]
		}
	}
	return nil
}

// SetMetricsCollector sets the metrics collector for the runner
func (r *Runner) SetMetricsCollector(mc *MetricsCollector) {
	r.metrics = mc
}

func (r *Runner) RunOnce(ctx context.Context) (RunSummary, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return RunSummary{Skipped: true}, nil
	}
	r.running = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	startedAt := time.Now().UTC()
	state := r.store.Snapshot()
	checks := enabledChecks(state.Checks)
	results := make([]CheckResult, 0, len(checks))

	if len(checks) == 0 {
		finishedAt := time.Now().UTC()
		summary := buildSummary(state.Checks, nil, &finishedAt)
		if err := r.store.SetLastRun(finishedAt); err != nil {
			return RunSummary{}, err
		}
		return RunSummary{StartedAt: startedAt, FinishedAt: finishedAt, Results: nil, Summary: summary}, nil
	}

	jobs := make(chan CheckConfig)
	out := make(chan CheckResult)
	workerCount := r.cfg.Workers
	if workerCount > len(checks) {
		workerCount = len(checks)
	}
	if workerCount <= 0 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for check := range jobs {
				result := r.executeCheck(ctx, check)
				out <- result
			}
		}()
	}

	go func() {
		for _, check := range checks {
			jobs <- check
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()

	for result := range out {
		results = append(results, result)
	}

	finishedAt := time.Now().UTC()
	if err := r.persistResults(results, finishedAt); err != nil {
		return RunSummary{}, err
	}

	summary := buildSummary(state.Checks, results, &finishedAt)
	return RunSummary{
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Results:    results,
		Summary:    summary,
	}, nil
}

// RunCheck executes and persists a single check result.
func (r *Runner) RunCheck(ctx context.Context, check CheckConfig) (CheckResult, error) {
	result := r.executeCheck(ctx, check)
	finishedAt := result.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
		result.FinishedAt = finishedAt
	}
	if err := r.persistResults([]CheckResult{result}, finishedAt); err != nil {
		return CheckResult{}, err
	}
	return result, nil
}

func (r *Runner) persistResults(results []CheckResult, finishedAt time.Time) error {
	if err := r.store.AppendResults(results, r.cfg.RetentionDays); err != nil {
		return err
	}
	return r.store.SetLastRun(finishedAt)
}

func (r *Runner) executeCheck(ctx context.Context, check CheckConfig) CheckResult {
	startedAt := time.Now().UTC()
	timeout := time.Duration(check.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := CheckResult{
		ID:          fmt.Sprintf("%s-%d", check.ID, startedAt.UnixNano()),
		CheckID:     check.ID,
		Name:        check.Name,
		Type:        check.Type,
		Server:      check.Server,
		Application: check.Application,
		StartedAt:   startedAt,
		Tags:        cloneTags(check.Tags),
		Metrics:     map[string]float64{},
	}

	exec, ok := LookupCheckExecutor(check.Type)
	if !ok {
		result.Status = "critical"
		result.Healthy = false
		result.Message = fmt.Sprintf("unsupported check type %q", check.Type)
	} else if err := exec.Execute(checkCtx, r, check, &result); err != nil {
		result.Status = "critical"
		result.Healthy = false
		result.Message = err.Error()
	}

	if result.Status == "" {
		result.Status = "healthy"
		result.Healthy = true
	}

	result.DurationMs = time.Since(startedAt).Milliseconds()
	result.FinishedAt = time.Now().UTC()

	// Record metrics if collector is available
	if r.metrics != nil {
		duration := time.Duration(result.DurationMs) * time.Millisecond
		r.metrics.RecordCheckRun(result.CheckID, result.Type, result.Status, duration)
	}

	return result
}

func enabledChecks(checks []CheckConfig) []CheckConfig {
	out := make([]CheckConfig, 0, len(checks))
	for _, check := range checks {
		if check.IsEnabled() {
			out = append(out, check)
		}
	}
	return out
}

func cloneTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, len(tags))
	copy(out, tags)
	return out
}

func processListCommand() (string, []string) {
	if runtime.GOOS == "darwin" {
		return "ps", []string{"-ax", "-o", "pid=,command="}
	}
	return "ps", []string{"-eo", "pid=,args="}
}

// SetMySQLSampler sets the MySQL sampler for the runner.
func (r *Runner) SetMySQLSampler(sampler MySQLSampler) {
	r.mysqlSampler = sampler
}

// SetMySQLRepo sets the MySQL repository for persisting samples.
func (r *Runner) SetMySQLRepo(repo MySQLMetricsRepository) {
	r.mysqlRepo = repo
}

// SetMySQLRuleEngine sets the MySQL rule engine for alert evaluation.
func (r *Runner) SetMySQLRuleEngine(engine *MySQLRuleEngine) {
	r.mysqlRuleEngine = engine
}

// SetIncidentManager sets the incident manager for rule-triggered incidents.
func (r *Runner) SetIncidentManager(im *IncidentManager) {
	r.incidentManager = im
}

// SetNotificationOutbox sets the notification outbox for alert delivery.
func (r *Runner) SetNotificationOutbox(outbox NotificationOutboxRepository) {
	r.outbox = outbox
}

// SetSnapshotRepo sets the snapshot repository for evidence capture.
func (r *Runner) SetSnapshotRepo(repo IncidentSnapshotRepository) {
	r.snapshotRepo = repo
}

func (r *Runner) SetServerMetricsRepo(repo *ServerMetricsRepository) {
	r.serverMetricsRepo = repo
}

func (r *Runner) runMySQL(ctx context.Context, check CheckConfig, result *CheckResult) error {
	if r.mysqlSampler == nil {
		return fmt.Errorf("mysql sampler not configured")
	}
	if check.MySQL == nil {
		return fmt.Errorf("mysql config block is required")
	}

	sample, err := r.mysqlSampler.Collect(ctx, check)
	if err != nil {
		return err
	}

	result.Metrics["connections"] = float64(sample.Connections)
	result.Metrics["maxConnections"] = float64(sample.MaxConnections)
	result.Metrics["threadsRunning"] = float64(sample.ThreadsRunning)
	result.Metrics["threadsConnected"] = float64(sample.ThreadsConnected)
	result.Metrics["abortedConnects"] = float64(sample.AbortedConnects)
	result.Metrics["slowQueries"] = float64(sample.SlowQueries)
	result.Metrics["questionsPerSec"] = sample.QuestionsPerSec
	result.Metrics["uptimeSeconds"] = float64(sample.UptimeSeconds)
	result.Metrics["innodbRowLockWaits"] = float64(sample.InnoDBRowLockWaits)
	result.Metrics["createdTmpDiskTables"] = float64(sample.CreatedTmpDiskTables)
	result.Metrics["createdTmpTables"] = float64(sample.CreatedTmpTables)
	result.Metrics["threadsCreated"] = float64(sample.ThreadsCreated)
	result.Metrics["maxUsedConnections"] = float64(sample.MaxUsedConnections)

	if sample.MaxConnections > 0 {
		utilPct := float64(sample.Connections) / float64(sample.MaxConnections) * 100
		result.Metrics["connectionUtilPct"] = utilPct
	}

	// Persist sample and compute delta
	var delta *MySQLDelta
	if r.mysqlRepo != nil {
		sampleID, err := r.mysqlRepo.AppendSample(sample)
		if err != nil {
			log.Printf("warning: failed to persist mysql sample: %v", err)
		} else {
			if d, err := r.mysqlRepo.ComputeAndAppendDelta(sampleID); err != nil {
				if !strings.Contains(err.Error(), "no previous sample") {
					log.Printf("warning: failed to compute mysql delta: %v", err)
				}
			} else {
				delta = &d
			}
		}
	}

	// Evaluate MySQL rules and process incidents
	if r.mysqlRuleEngine != nil && r.incidentManager != nil {
		evalResults := r.mysqlRuleEngine.Evaluate(check.ID, sample, delta)
		for _, er := range evalResults {
			if er.Action == "open" {
				metadata := map[string]string{
					"ruleCode": er.RuleCode,
					"checkId":  er.CheckID,
				}
				if err := r.incidentManager.ProcessAlert(
					er.CheckID,
					check.Name,
					"mysql",
					er.Severity,
					er.Message,
					metadata,
				); err != nil {
					log.Printf("warning: failed to create incident for rule %s: %v", er.RuleCode, err)
				} else {
					// Record incident ID in rule engine state
					if incident, findErr := r.incidentManager.repo.FindOpenIncident(er.CheckID); findErr == nil && incident.ID != "" {
						r.mysqlRuleEngine.SetOpenIncidentID(er.RuleCode, er.CheckID, incident.ID)

						// Enqueue notification
						if r.outbox != nil {
							notifEvt := NotificationEvent{
								NotificationID: fmt.Sprintf("notif-%s-%d", incident.ID, time.Now().UnixNano()),
								IncidentID:     incident.ID,
								Channel:        "default",
								PayloadJSON:    fmt.Sprintf(`{"severity":"%s","rule":"%s","message":"%s","checkId":"%s"}`, er.Severity, er.RuleCode, er.Message, er.CheckID),
								Status:         "pending",
								CreatedAt:      time.Now().UTC(),
							}
							if err := r.outbox.Enqueue(notifEvt); err != nil {
								log.Printf("warning: failed to enqueue notification: %v", err)
							}
						}

						// Capture evidence snapshots
						if r.snapshotRepo != nil && r.mysqlRepo != nil {
							evidenceCollector := NewMySQLEvidenceCollector(r.mysqlRepo)
							snaps := evidenceCollector.CaptureEvidence(ctx, incident.ID, check.ID, nil)
							if err := r.snapshotRepo.SaveSnapshots(incident.ID, snaps); err != nil {
								log.Printf("warning: failed to save evidence snapshots: %v", err)
							}
						}
					}
				}
			} else if er.Action == "close" && er.IncidentID != "" {
				if err := r.incidentManager.ResolveIncident(er.IncidentID, "system-auto-recovery"); err != nil {
					log.Printf("warning: failed to auto-resolve incident %s: %v", er.IncidentID, err)
				}
			}
		}
	}

	return nil
}

func (r *Runner) runSSH(ctx context.Context, check CheckConfig, result *CheckResult) error {
	if check.SSH == nil {
		return fmt.Errorf("ssh config block is required")
	}

	timeout := time.Duration(check.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	metrics, err := collectSSHMetrics(check.SSH, timeout)
	if err != nil {
		return err
	}

	for k, v := range metrics.toMap() {
		result.Metrics[k] = v
	}

	status, messages := buildSSHStatus(metrics, check)
	result.Status = status
	result.Healthy = status == "healthy"
	result.Message = strings.Join(messages, "; ")

	// Save server metrics snapshot for historical tracking
	if r.serverMetricsRepo != nil && check.SSH != nil {
		serverID := check.ServerId
		if serverID == "" {
			serverID = check.Server
		}
		if serverID == "" {
			serverID = check.SSH.Host
		}
		snap := SnapshotFromMetrics(serverID, metrics)
		_ = r.serverMetricsRepo.Save(snap)
	}

	return nil
}
