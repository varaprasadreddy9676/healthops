package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultTargetURL      = "http://localhost:8080"
	defaultCheckCount     = 100
	defaultDuration       = 5 * time.Minute
	defaultQueryLoad      = 100
	defaultInterval       = 30 * time.Second
	defaultWorkers        = 50
	defaultGoroutineLimit = 10000
)

var (
	targetURL       = flag.String("target", defaultTargetURL, "Health monitoring service URL")
	checkCount      = flag.Int("checks", defaultCheckCount, "Number of checks to create")
	duration        = flag.Duration("duration", defaultDuration, "Test duration")
	queryLoad       = flag.Int("queries", defaultQueryLoad, "Number of concurrent queries for query performance test")
	interval        = flag.Duration("interval", defaultInterval, "Check interval in seconds")
	workers         = flag.Int("workers", defaultWorkers, "Number of concurrent workers")
	scenario        = flag.String("scenario", "", "Specific scenario to run (scheduler, query, memory) or empty for all")
	verbose         = flag.Bool("verbose", false, "Verbose output")
	schedulerLagMax = flag.Duration("scheduler-lag-max", 5*time.Second, "Maximum acceptable scheduler lag")
	queryLatencyMax = flag.Duration("query-latency-max", 200*time.Millisecond, "Maximum acceptable query latency (p95)")
	memoryGrowthMax = flag.Float64("memory-growth-max", 10.0, "Maximum acceptable memory growth percentage")
	goroutineLimit  = flag.Int("goroutine-limit", defaultGoroutineLimit, "Maximum acceptable goroutine count")

	// Single-endpoint mode (CSV output). Activated when -url is set.
	singleURL = flag.String("url", "", "Single endpoint to load (enables CSV mode; overrides scenario flow)")
	rps       = flag.Int("rps", 100, "Target requests per second (single-endpoint mode)")
	method    = flag.String("method", "GET", "HTTP method for single-endpoint mode")
	csvHeader = flag.Bool("csv-header", true, "Print CSV header in single-endpoint mode")
	conns     = flag.Int("conns", 64, "Max concurrent in-flight requests in single-endpoint mode")
	bearer    = flag.String("bearer", "", "Optional bearer token sent as Authorization header")
)

type metrics struct {
	schedulerLag    []time.Duration
	checkDurations  []time.Duration
	queryLatencies  []time.Duration
	memorySamples   []memorySample
	goroutineCounts []int
	successCount    atomic.Int64
	failureCount    atomic.Int64
	totalChecks     atomic.Int64
	totalQueries    atomic.Int64
	startTime       time.Time
	lastCheckTime   atomic.Value
	mu              sync.Mutex
}

type memorySample struct {
	Timestamp  time.Time
	AllocMB    float64
	HeapMB     float64
	Goroutines int
}

type checkConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Server          string `json:"server,omitempty"`
	Application     string `json:"application,omitempty"`
	Target          string `json:"target,omitempty"`
	Port            int    `json:"port,omitempty"`
	ExpectedStatus  int    `json:"expectedStatus,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	Enabled         bool   `json:"enabled"`
}

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   string      `json:"error,omitempty"`
}

func main() {
	flag.Parse()

	if *verbose {
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	}

	// Single-endpoint CSV mode: when -url is provided, run a focused RPS
	// test against a single endpoint and emit a single CSV row.
	if *singleURL != "" {
		if err := runSingleEndpoint(*singleURL, *method, *rps, *duration, *conns, *bearer, *csvHeader); err != nil {
			log.Fatalf("single-endpoint mode failed: %v", err)
		}
		return
	}

	m := &metrics{
		schedulerLag:    make([]time.Duration, 0, 10000),
		checkDurations:  make([]time.Duration, 0, 10000),
		queryLatencies:  make([]time.Duration, 0, 10000),
		memorySamples:   make([]memorySample, 0, 720),
		goroutineCounts: make([]int, 0, 720),
		startTime:       time.Now(),
	}

	// Pre-warm the connection
	if err := waitForService(*targetURL); err != nil {
		log.Fatalf("Failed to connect to service: %v", err)
	}

	log.Printf("Starting load test against %s", *targetURL)
	log.Printf("Configuration: checks=%d duration=%s workers=%d interval=%s", *checkCount, *duration, *workers, *interval)

	var scenarios []string
	if *scenario == "" {
		scenarios = []string{"scheduler", "query", "memory"}
	} else {
		scenarios = []string{*scenario}
	}

	// Track initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	m.goroutineCounts = append(m.goroutineCounts, initialGoroutines)

	for _, scen := range scenarios {
		log.Printf("========================================")
		log.Printf("Running scenario: %s", scen)
		log.Printf("========================================")

		switch scen {
		case "scheduler":
			if err := runSchedulerScenario(m); err != nil {
				log.Fatalf("Scheduler scenario failed: %v", err)
			}
		case "query":
			if err := runQueryScenario(m); err != nil {
				log.Fatalf("Query scenario failed: %v", err)
			}
		case "memory":
			if err := runMemoryScenario(m); err != nil {
				log.Fatalf("Memory scenario failed: %v", err)
			}
		}
	}

	// Final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	m.goroutineCounts = append(m.goroutineCounts, finalGoroutines)

	// Print summary
	printSummary(m)

	// Exit with error if thresholds exceeded
	if err := checkThresholds(m, initialGoroutines, finalGoroutines); err != nil {
		log.Fatalf("Load test failed: %v", err)
	}

	log.Println("Load test passed all thresholds")
}

func waitForService(url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for service")
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", url+"/healthz", nil)
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

func runSchedulerScenario(m *metrics) error {
	log.Printf("Creating %d checks with %s interval...", *checkCount, *interval)

	// Create checks (only use api and tcp types to avoid validation issues)
	var checks []checkConfig
	for i := 0; i < *checkCount; i++ {
		checkType := "api"
		if i%2 == 1 {
			checkType = "tcp"
		}

		check := checkConfig{
			ID:              fmt.Sprintf("loadtest-check-%d", i),
			Name:            fmt.Sprintf("Load Test Check %d", i),
			Type:            checkType,
			Server:          fmt.Sprintf("server-%d", i%10),
			Application:     "loadtest",
			TimeoutSeconds:  5,
			IntervalSeconds: int(interval.Seconds()),
			Enabled:         true,
		}

		switch checkType {
		case "api":
			// Use localhost health endpoint for faster local testing
			check.Target = "http://localhost:8080/healthz"
			check.ExpectedStatus = 200
		case "tcp":
			// Check common local ports - need host for tcp checks
			check.Target = "localhost"
			check.Port = 8080 + (i % 10)
		}

		checks = append(checks, check)
	}

	// Create checks via API
	client := &http.Client{Timeout: 10 * time.Second}
	for _, check := range checks {
		body, _ := json.Marshal(check)
		req, err := http.NewRequest("POST", *targetURL+"/api/v1/checks", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to create check %s: %w", check.ID, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("unexpected status creating check %s: %d", check.ID, resp.StatusCode)
		}

		m.totalChecks.Add(1)
	}

	log.Printf("Successfully created %d checks", len(checks))

	// Monitor scheduler performance for the duration
	log.Printf("Monitoring scheduler performance for %s...", *duration)
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	monitorTicker := time.NewTicker(1 * time.Second)
	defer monitorTicker.Stop()

	expectedInterval := *interval
	tickCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("Scheduler scenario completed")
			return cleanupChecks(client)
		case <-ticker.C:
			tickCount++
			elapsed := time.Duration(tickCount) * 5 * time.Second

			// Check for scheduler lag
			resp, err := client.Get(*targetURL + "/api/v1/summary")
			if err != nil {
				log.Printf("Warning: Failed to get summary: %v", err)
				continue
			}

			var summaryResp apiResponse
			if err := json.NewDecoder(resp.Body).Decode(&summaryResp); err != nil {
				resp.Body.Close()
				log.Printf("Warning: Failed to decode summary: %v", err)
				continue
			}
			resp.Body.Close()

			// Calculate expected vs actual check times
			if summaryResp.Data != nil {
				data, _ := json.Marshal(summaryResp.Data)
				var summary struct {
					LastRunAt *string `json:"lastRunAt"`
				}
				if err := json.Unmarshal(data, &summary); err == nil && summary.LastRunAt != nil {
					lastRun, err := time.Parse(time.RFC3339Nano, *summary.LastRunAt)
					if err == nil {
						timeSinceLastRun := time.Since(lastRun)

						// Expected lag should be close to interval
						expectedTick := elapsed % expectedInterval
						lag := timeSinceLastRun - expectedTick
						if lag < 0 {
							lag = -lag
						}

						m.mu.Lock()
						m.schedulerLag = append(m.schedulerLag, lag)
						m.mu.Unlock()

						if *verbose && lag > *schedulerLagMax {
							log.Printf("Warning: High scheduler lag detected: %v (expected: ~%v)", lag, expectedInterval)
						}
					}
				}
			}

			// Sample memory and goroutines
			sampleMemory(m)

			log.Printf("Progress: %s elapsed | Checks: %d successes, %d failures | Goroutines: %d",
				elapsed.Round(time.Second),
				m.successCount.Load(),
				m.failureCount.Load(),
				runtime.NumGoroutine())

		case <-monitorTicker.C:
			// Track latest check execution time
			m.lastCheckTime.Store(time.Now())
		}
	}
}

func runQueryScenario(m *metrics) error {
	log.Printf("Creating historical data for query performance test...")

	// Create historical results by triggering runs
	client := &http.Client{Timeout: 30 * time.Second}

	// First create checks if needed
	for i := 0; i < 100; i++ {
		check := checkConfig{
			ID:              fmt.Sprintf("querytest-check-%d", i),
			Name:            fmt.Sprintf("Query Test Check %d", i),
			Type:            "api",
			Server:          fmt.Sprintf("server-%d", i%5),
			Application:     "querytest",
			Target:          *targetURL + "/healthz",
			ExpectedStatus:  200,
			TimeoutSeconds:  5,
			IntervalSeconds: 60,
			Enabled:         true,
		}

		body, _ := json.Marshal(check)
		req, _ := http.NewRequest("POST", *targetURL+"/api/v1/checks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Warning: Failed to create check: %v", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			log.Printf("Warning: Unexpected status creating check: %d", resp.StatusCode)
		}
	}

	// Generate some historical data by triggering runs
	log.Printf("Generating historical data...")
	for i := 0; i < 5; i++ {
		resp, err := http.Post(*targetURL+"/api/v1/runs", "application/json", bytes.NewReader([]byte("{}")))
		if err != nil {
			log.Printf("Warning: Failed to trigger run: %v", err)
			continue
		}
		resp.Body.Close()
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("Running %d concurrent queries for %s...", *queryLoad, *duration)

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// Launch concurrent query workers
	var wg sync.WaitGroup
	for i := 0; i < *queryLoad; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			ticker := time.NewTicker(time.Duration(100+rand.Intn(200)) * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Mix of different query types
					queryType := rand.Intn(4)
					var start time.Time
					var resp *http.Response
					var err error

					switch queryType {
					case 0:
						start = time.Now()
						resp, err = client.Get(*targetURL + "/api/v1/summary")
					case 1:
						start = time.Now()
						resp, err = client.Get(*targetURL + "/api/v1/dashboard/summary")
					case 2:
						start = time.Now()
						resp, err = client.Get(*targetURL + "/api/v1/dashboard/checks")
					case 3:
						start = time.Now()
						checkID := fmt.Sprintf("querytest-check-%d", rand.Intn(100))
						resp, err = client.Get(*targetURL + "/api/v1/results?checkId=" + checkID + "&days=7")
					}

					duration := time.Since(start)

					m.mu.Lock()
					m.queryLatencies = append(m.queryLatencies, duration)
					m.mu.Unlock()

					m.totalQueries.Add(1)

					if err != nil {
						m.failureCount.Add(1)
						if *verbose {
							log.Printf("Worker %d: Query failed: %v", workerID, err)
						}
					} else {
						if resp != nil {
							_ = resp.Body.Close()
						}
						m.successCount.Add(1)
					}
				}
			}
		}(i)
	}

	// Monitor progress
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			log.Printf("Query scenario completed")
			return cleanupChecks(client)
		case <-ticker.C:
			sampleMemory(m)

			log.Printf("Queries: %d total | Success: %d | Failed: %d | Goroutines: %d",
				m.totalQueries.Load(),
				m.successCount.Load(),
				m.failureCount.Load(),
				runtime.NumGoroutine())
		}
	}
}

func runMemoryScenario(m *metrics) error {
	log.Printf("Running memory stability test for %s...", *duration)

	if *duration < 10*time.Minute {
		log.Printf("Warning: Memory tests should run for at least 10 minutes for accurate results")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}

	// Create moderate number of checks
	checkCount := 50
	for i := 0; i < checkCount; i++ {
		check := checkConfig{
			ID:              fmt.Sprintf("memtest-check-%d", i),
			Name:            fmt.Sprintf("Memory Test Check %d", i),
			Type:            "api",
			Server:          fmt.Sprintf("server-%d", i%5),
			Application:     "memtest",
			Target:          *targetURL + "/healthz",
			ExpectedStatus:  200,
			TimeoutSeconds:  5,
			IntervalSeconds: 30,
			Enabled:         true,
		}

		body, _ := json.Marshal(check)
		req, _ := http.NewRequest("POST", *targetURL+"/api/v1/checks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Warning: Failed to create check: %v", err)
			continue
		}
		resp.Body.Close()
	}

	// Trigger periodic runs
	runTicker := time.NewTicker(30 * time.Second)
	defer runTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-runTicker.C:
				resp, err := http.Post(*targetURL+"/api/v1/runs", "application/json", bytes.NewReader([]byte("{}")))
				if err == nil {
					resp.Body.Close()
				}
			}
		}
	}()

	// Monitor memory every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	sampleCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("Memory scenario completed")
			return cleanupChecks(client)
		case <-ticker.C:
			sampleCount++
			sampleMemory(m)

			var mStats runtime.MemStats
			runtime.ReadMemStats(&mStats)

			allocMB := float64(mStats.Alloc) / 1024 / 1024
			heapMB := float64(mStats.HeapAlloc) / 1024 / 1024
			goroutines := runtime.NumGoroutine()

			log.Printf("Memory sample %d: Alloc=%.2fMB Heap=%.2fMB Goroutines=%d",
				sampleCount, allocMB, heapMB, goroutines)
		}
	}
}

func sampleMemory(m *metrics) {
	var mStats runtime.MemStats
	runtime.ReadMemStats(&mStats)

	sample := memorySample{
		Timestamp:  time.Now(),
		AllocMB:    float64(mStats.Alloc) / 1024 / 1024,
		HeapMB:     float64(mStats.HeapAlloc) / 1024 / 1024,
		Goroutines: runtime.NumGoroutine(),
	}

	m.mu.Lock()
	m.memorySamples = append(m.memorySamples, sample)
	m.goroutineCounts = append(m.goroutineCounts, sample.Goroutines)
	m.mu.Unlock()
}

func cleanupChecks(client *http.Client) error {
	log.Printf("Cleaning up test checks...")

	// Get all checks
	resp, err := client.Get(*targetURL + "/api/v1/checks")
	if err != nil {
		return fmt.Errorf("failed to get checks: %w", err)
	}
	defer resp.Body.Close()

	var checksResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&checksResp); err != nil {
		return fmt.Errorf("failed to decode checks: %w", err)
	}

	// Delete test checks
	if checksResp.Data != nil {
		data, _ := json.Marshal(checksResp.Data)
		var checks []checkConfig
		if err := json.Unmarshal(data, &checks); err == nil {
			deleted := 0
			for _, check := range checks {
				if len(check.ID) > 9 && (check.ID[:9] == "loadtest-" || check.ID[:10] == "querytest-" || check.ID[:8] == "memtest-") {
					req, _ := http.NewRequest("DELETE", *targetURL+"/api/v1/checks/"+check.ID, nil)
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						deleted++
					}
				}
			}
			log.Printf("Deleted %d test checks", deleted)
		}
	}

	return nil
}

func printSummary(m *metrics) {
	log.Printf("========================================")
	log.Printf("LOAD TEST SUMMARY")
	log.Printf("========================================")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Scheduler lag statistics
	if len(m.schedulerLag) > 0 {
		log.Printf("\nScheduler Lag:")
		printPercentiles(m.schedulerLag, "lag")
	}

	// Query latency statistics
	if len(m.queryLatencies) > 0 {
		log.Printf("\nQuery Latency:")
		printPercentiles(m.queryLatencies, "latency")
	}

	// Memory statistics
	if len(m.memorySamples) > 1 {
		log.Printf("\nMemory Usage:")
		firstSample := m.memorySamples[0]
		lastSample := m.memorySamples[len(m.memorySamples)-1]

		allocGrowth := ((lastSample.AllocMB - firstSample.AllocMB) / firstSample.AllocMB) * 100
		heapGrowth := ((lastSample.HeapMB - firstSample.HeapMB) / firstSample.HeapMB) * 100

		log.Printf("  Initial: Alloc=%.2fMB Heap=%.2fMB", firstSample.AllocMB, firstSample.HeapMB)
		log.Printf("  Final:   Alloc=%.2fMB Heap=%.2fMB", lastSample.AllocMB, lastSample.HeapMB)
		log.Printf("  Growth:  Alloc=%.2f%% Heap=%.2f%%", allocGrowth, heapGrowth)
	}

	// Goroutine statistics
	if len(m.goroutineCounts) > 0 {
		minGoroutines := m.goroutineCounts[0]
		maxGoroutines := m.goroutineCounts[0]
		for _, count := range m.goroutineCounts {
			if count < minGoroutines {
				minGoroutines = count
			}
			if count > maxGoroutines {
				maxGoroutines = count
			}
		}
		log.Printf("\nGoroutines:")
		log.Printf("  Min: %d", minGoroutines)
		log.Printf("  Max: %d", maxGoroutines)
		log.Printf("  Final: %d", m.goroutineCounts[len(m.goroutineCounts)-1])
	}

	// Overall statistics
	log.Printf("\nOverall Statistics:")
	log.Printf("  Total Checks: %d", m.totalChecks.Load())
	log.Printf("  Total Queries: %d", m.totalQueries.Load())
	log.Printf("  Successes: %d", m.successCount.Load())
	log.Printf("  Failures: %d", m.failureCount.Load())
	log.Printf("  Duration: %s", time.Since(m.startTime).Round(time.Millisecond))
}

func printPercentiles(durations []time.Duration, label string) {
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	p50 := durations[len(durations)*50/100]
	p95 := durations[len(durations)*95/100]
	p99 := durations[len(durations)*99/100]

	log.Printf("  p50: %v", p50.Round(time.Millisecond))
	log.Printf("  p95: %v", p95.Round(time.Millisecond))
	log.Printf("  p99: %v", p99.Round(time.Millisecond))
}

func checkThresholds(m *metrics, initialGoroutines, finalGoroutines int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []string

	// Check scheduler lag
	if len(m.schedulerLag) > 0 {
		sort.Slice(m.schedulerLag, func(i, j int) bool {
			return m.schedulerLag[i] < m.schedulerLag[j]
		})
		p95Lag := m.schedulerLag[len(m.schedulerLag)*95/100]
		if p95Lag > *schedulerLagMax {
			errors = append(errors, fmt.Sprintf("scheduler lag p95 (%v) exceeds threshold (%v)", p95Lag, *schedulerLagMax))
		}
	}

	// Check query latency
	if len(m.queryLatencies) > 0 {
		sort.Slice(m.queryLatencies, func(i, j int) bool {
			return m.queryLatencies[i] < m.queryLatencies[j]
		})
		p95Latency := m.queryLatencies[len(m.queryLatencies)*95/100]
		if p95Latency > *queryLatencyMax {
			errors = append(errors, fmt.Sprintf("query latency p95 (%v) exceeds threshold (%v)", p95Latency, *queryLatencyMax))
		}
	}

	// Check memory growth
	if len(m.memorySamples) > 10 {
		// Use last 10 samples for stability check
		stableSamples := m.memorySamples[len(m.memorySamples)-10:]
		firstStable := stableSamples[0]
		lastStable := stableSamples[len(stableSamples)-1]

		if firstStable.AllocMB > 0 {
			allocGrowth := ((lastStable.AllocMB - firstStable.AllocMB) / firstStable.AllocMB) * 100
			if allocGrowth > *memoryGrowthMax {
				errors = append(errors, fmt.Sprintf("memory growth (%.2f%%) exceeds threshold (%.2f%%)", allocGrowth, *memoryGrowthMax))
			}
		}
	}

	// Check goroutine count
	if finalGoroutines > *goroutineLimit {
		errors = append(errors, fmt.Sprintf("goroutine count (%d) exceeds limit (%d)", finalGoroutines, *goroutineLimit))
	}

	// Check for goroutine leaks
	goroutineGrowth := finalGoroutines - initialGoroutines
	if goroutineGrowth > 1000 {
		errors = append(errors, fmt.Sprintf("potential goroutine leak detected (%d growth)", goroutineGrowth))
	}

	if len(errors) > 0 {
		return fmt.Errorf("threshold violations:\n  - %s", joinErrors(errors))
	}

	return nil
}

func joinErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	result := errors[0]
	for i := 1; i < len(errors); i++ {
		result += "\n  - " + errors[i]
	}
	return result
}

// runSingleEndpoint hammers a single URL at a target RPS for the given
// duration and prints a single CSV row with p50/p95/p99/max latencies.
// Output format:
//
//	endpoint,rps_target,duration_s,requests,errors,p50_ms,p95_ms,p99_ms,max_ms
func runSingleEndpoint(url, httpMethod string, targetRPS int, dur time.Duration, maxConns int, bearerTok string, header bool) error {
	if targetRPS <= 0 {
		return fmt.Errorf("rps must be > 0")
	}
	if dur <= 0 {
		return fmt.Errorf("duration must be > 0")
	}
	if maxConns <= 0 {
		maxConns = 64
	}

	transport := &http.Transport{
		MaxIdleConns:        maxConns * 2,
		MaxIdleConnsPerHost: maxConns * 2,
		MaxConnsPerHost:     maxConns * 2,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	tickInterval := time.Second / time.Duration(targetRPS)
	if tickInterval <= 0 {
		tickInterval = time.Microsecond
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	sem := make(chan struct{}, maxConns)
	var (
		mu        sync.Mutex
		latencies = make([]time.Duration, 0, targetRPS*int(dur.Seconds())+1024)
		requests  atomic.Int64
		errCount  atomic.Int64
		wg        sync.WaitGroup
	)

	doRequest := func() {
		defer wg.Done()
		defer func() { <-sem }()

		req, err := http.NewRequestWithContext(ctx, httpMethod, url, nil)
		if err != nil {
			errCount.Add(1)
			return
		}
		if bearerTok != "" {
			req.Header.Set("Authorization", "Bearer "+bearerTok)
		}

		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)
		requests.Add(1)
		if err != nil {
			errCount.Add(1)
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			errCount.Add(1)
		}

		mu.Lock()
		latencies = append(latencies, elapsed)
		mu.Unlock()
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	startWall := time.Now()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			select {
			case sem <- struct{}{}:
				wg.Add(1)
				go doRequest()
			default:
				// Saturated: drop tick (counted neither as request nor error).
			}
		}
	}

	wg.Wait()
	actualDur := time.Since(startWall).Seconds()

	mu.Lock()
	defer mu.Unlock()

	p50ms, p95ms, p99ms, maxms := percentilesMs(latencies)

	if header {
		fmt.Println("endpoint,rps_target,duration_s,requests,errors,p50_ms,p95_ms,p99_ms,max_ms")
	}
	fmt.Printf("%s,%d,%.0f,%d,%d,%.2f,%.2f,%.2f,%.2f\n",
		url,
		targetRPS,
		actualDur,
		requests.Load(),
		errCount.Load(),
		p50ms, p95ms, p99ms, maxms,
	)
	return nil
}

// percentilesMs returns p50/p95/p99/max in milliseconds.
func percentilesMs(d []time.Duration) (p50, p95, p99, maxv float64) {
	if len(d) == 0 {
		return 0, 0, 0, 0
	}
	sorted := make([]time.Duration, len(d))
	copy(sorted, d)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := func(p float64) time.Duration {
		i := int(float64(len(sorted)-1) * p)
		if i < 0 {
			i = 0
		}
		if i >= len(sorted) {
			i = len(sorted) - 1
		}
		return sorted[i]
	}
	toMs := func(t time.Duration) float64 { return float64(t.Microseconds()) / 1000.0 }
	return toMs(idx(0.50)), toMs(idx(0.95)), toMs(idx(0.99)), toMs(sorted[len(sorted)-1])
}
