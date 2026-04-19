# Load Test Report

## Phase
- Phase 7.2: Load test suite verification
- Date: 2026-04-18

## Test Setup
- Service command:
  - `cd backend && go run ./cmd/healthops`
- Load test command:
  - `cd backend && go run ./cmd/loadtest -scenario=query -duration=20s -checks=20 -queries=20 -workers=10 -memory-growth-max=400 -goroutine-limit=5000`

## Result
- Status: `PASS`
- Query throughput: `1965` total queries
- Success: `1965`
- Failures: `0`
- Latency:
  - `p50: 4ms`
  - `p95: 20ms`
  - `p99: 40ms`
- Goroutines:
  - `min: 1`
  - `max: 23`
  - `final: 5`

## Notes
- During closure work, load test implementation was fixed to close HTTP response bodies in query workers.
- Query and memory scenarios were switched to target local health endpoint instead of external `httpbin` for deterministic behavior.
- Prior goroutine growth issue was resolved after these fixes.

## Recommended Next Runs
1. Scheduler scenario: 5-10m with target check load.
2. Memory scenario: >=30m run on staging-sized workload.
3. Full combined run before production cut.

---

## Final Gate Rerun (2026-04-18)

### Query Scenario (Checklist Command)
- Command:
  - `cd backend && go run ./cmd/loadtest -scenario=query -duration=20s -checks=20 -queries=20 -workers=10 -memory-growth-max=400 -goroutine-limit=5000`
- Result: `PASS`
- Evidence:
  - Total queries: `1817`
  - Failures: `0`
  - Latency: `p50=1ms`, `p95=2ms`, `p99=4ms`
  - Goroutines: `max=21`, `final=3`

### Scheduler Scenario
- Command:
  - `cd backend && go run ./cmd/loadtest -scenario=scheduler -duration=5m -checks=100 -workers=50 -scheduler-lag-max=5s -memory-growth-max=400 -goroutine-limit=5000`
- Result: `FAIL`
- Evidence:
  - Reported threshold violation: `scheduler lag p95 (16m38.921784s) exceeds threshold (5s)`
  - Run exhibited long execution stalls; success/failure counters remained `0`.

### Memory Scenario (>=30m)
- Command:
  - `cd backend && go run ./cmd/loadtest -scenario=memory -duration=30m -checks=20 -workers=10 -memory-growth-max=400 -goroutine-limit=5000`
- Result: `INCONCLUSIVE / FAIL`
- Evidence:
  - Run showed severe timing stalls and did not complete with expected 30-minute cadence.
  - Process required manual termination; no trustworthy pass result for leak trend gate.

## Final Load Gate Verdict
- `NO-GO` for production based on failed scheduler threshold and incomplete 30m memory verification.
