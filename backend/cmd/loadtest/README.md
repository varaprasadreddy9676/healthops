# Load Test Suite

Comprehensive load testing for the health monitoring service to validate performance, scalability, and resource management.

## Features

- **Scenario 1: Scheduler Throughput**
  - Creates 100 checks with 30-second intervals
  - Monitors scheduler lag (time drift from expected schedule)
  - Tracks goroutine count for leak detection
  - Runs for 5 minutes by default

- **Scenario 2: Query Performance**
  - Creates 500 checks with historical data
  - Executes 100 concurrent dashboard queries
  - Measures p50, p95, p99 latency
  - Tests all dashboard endpoints

- **Scenario 3: Memory Stability**
  - Long-running test (1+ hour recommended)
  - Monitors memory usage every 30 seconds
  - Detects memory leaks via growth analysis
  - Tracks goroutine count over time

## Usage

### Basic Usage

```bash
cd backend && go run ./cmd/loadtest
```

This runs all scenarios with default settings against `http://localhost:8080`.

### Run Specific Scenario

```bash
# Test scheduler performance only
cd backend && go run ./cmd/loadtest -scenario=scheduler

# Test query performance only
cd backend && go run ./cmd/loadtest -scenario=query

# Test memory stability only
cd backend && go run ./cmd/loadtest -scenario=memory
```

### Custom Configuration

```bash
# Custom target URL
cd backend && go run ./cmd/loadtest -target http://production.example.com

# Adjust test parameters
cd backend && go run ./cmd/loadtest -checks 200 -duration 10m -workers 100

# Set custom thresholds
cd backend && go run ./cmd/loadtest -scheduler-lag-max 10s -query-latency-max 500ms

# Verbose output
cd backend && go run ./cmd/loadtest -verbose
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-target` | `http://localhost:8080` | Health monitoring service URL |
| `-checks` | `100` | Number of checks to create |
| `-duration` | `5m` | Test duration |
| `-queries` | `100` | Concurrent queries for query test |
| `-interval` | `30s` | Check interval |
| `-workers` | `50` | Concurrent workers |
| `-scenario` | `(all)` | Specific scenario: scheduler, query, memory |
| `-verbose` | `false` | Enable verbose logging |
| `-scheduler-lag-max` | `5s` | Maximum acceptable scheduler lag |
| `-query-latency-max` | `200ms` | Maximum acceptable query latency (p95) |
| `-memory-growth-max` | `10.0` | Maximum memory growth percentage |
| `-goroutine-limit` | `10000` | Maximum acceptable goroutine count |

## Metrics Collected

### Scheduler Throughput
- **Scheduler Lag**: Time difference between expected and actual check execution
- **Goroutine Count**: Detects leaks if count grows continuously
- **Success/Failure Ratio**: Tracks check execution reliability

### Query Performance
- **Latency Percentiles**: p50, p95, p99 for all query types
- **Throughput**: Queries per second
- **Error Rate**: Failed queries percentage

### Memory Stability
- **Heap Allocation**: Current heap usage in MB
- **Total Allocation**: Cumulative allocations in MB
- **Growth Rate**: Percentage change over test duration
- **Goroutine Trends**: Count over time to detect leaks

## Exit Codes

- **0**: All thresholds passed
- **1**: One or more thresholds exceeded

## Examples

### Quick smoke test (1 minute)
```bash
cd backend && go run ./cmd/loadtest -duration 1m -checks 50
```

### Full production validation
```bash
cd backend && go run ./cmd/loadtest \
  -target http://prod.monitoring.example.com \
  -checks 500 \
  -duration 1h \
  -scenario memory \
  -scheduler-lag-max 10s \
  -query-latency-max 500ms
```

### CI/CD integration
```bash
# Run quick scheduler and query tests
cd backend && go run ./cmd/loadtest \
  -duration 2m \
  -scenario scheduler \
  && go run ./cmd/loadtest \
  -duration 2m \
  -scenario query
```

## Interpreting Results

### Good Results
```
Scheduler Lag:
  p50: 234ms
  p95: 567ms        ← Below 5s threshold ✓
  p99: 1.2s

Query Latency:
  p50: 45ms
  p95: 156ms       ← Below 200ms threshold ✓
  p99: 234ms

Memory Usage:
  Initial: Alloc=45.23MB Heap=30.12MB
  Final:   Alloc=47.89MB Heap=31.45MB
  Growth:  Alloc=5.88%  Heap=4.44%  ← Below 10% threshold ✓

Goroutines:
  Min: 245
  Max: 312
  Final: 267      ← Stable, no leak ✓
```

### Problematic Results
```
Scheduler Lag:
  p50: 2.3s
  p95: 8.7s       ← Exceeds 5s threshold ✗
  p99: 15.2s

Memory Usage:
  Initial: Alloc=45.23MB Heap=30.12MB
  Final:   Alloc=125.67MB Heap=98.34MB
  Growth:  Alloc=177.8%  Heap=226.6%  ← Exceeds 10% threshold ✗

Goroutines:
  Min: 245
  Max: 15234
  Final: 14892     ← Exceeds limit, potential leak ✗
```

## Troubleshooting

### Connection Refused
Ensure the health monitoring service is running:
```bash
cd backend && go run ./cmd/healthmon
```

### High Scheduler Lag
- Reduce check count or increase interval
- Check if system is CPU constrained
- Verify worker pool configuration

### Memory Growth Detected
- Run with `-scenario memory -duration 30m` for extended monitoring
- Check for goroutine leaks in `-verbose` output
- Profile the service with `pprof`

### Timeout Errors
- Increase `-timeout` flag value
- Check network latency to target service
- Verify service health endpoint responds quickly

## Integration with CI/CD

### GitHub Actions Example
```yaml
name: Load Tests

on: [push, pull_request]

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Start service
        run: |
          cd backend
          go run ./cmd/healthmon &
          sleep 10
      - name: Run load tests
        run: |
          cd backend
          go run ./cmd/loadtest -duration 2m -checks 100
```

### Jenkins Pipeline Example
```groovy
stage('Load Test') {
    steps {
        sh '''
            cd backend
            go run ./cmd/healthmon &
            sleep 10
            go run ./cmd/loadtest -duration 5m
        '''
    }
}
```

## Performance Baselines

Based on testing with the following hardware:
- CPU: 4 cores
- RAM: 8GB
- Storage: SSD

**Expected Performance:**
- Scheduler Lag p95: < 1s
- Query Latency p95: < 100ms
- Memory Growth: < 5% after stabilization
- Goroutines: < 500 (steady state)

Your baselines may vary based on hardware and configuration.
