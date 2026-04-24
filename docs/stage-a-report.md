# Stage A Hardening Report

Date: 2026-04-24
Branch: `refactor/enterprise-restructure`
Goal: produce real evidence of durability, security, and performance before staging soak.

## Summary

| Area | Status | Evidence |
|---|---|---|
| Chaos: panic mid-write | PASS | 100 iterations, 671 panics, zero corruption — see `store_chaos_test.go` |
| Chaos: process kill -9 | PASS | 5/5 iterations, all state.json valid JSON — `scripts/chaos/run-chaos.sh` |
| JSONL torn-line resilience | PASS | 5000 attempts, 491 panics, no torn lines on Load — `jsonl_chaos_test.go` |
| Disk-full / write failure | PASS | Previous state preserved when write fails — `store_diskfull_test.go` |
| Concurrent writers, no torn JSON | PASS | 1450 reads observed, 0 torn — `store_chaos_test.go` |
| gosec HIGH findings | 18 (all reviewed, accepted) | See "Security Scan" below |
| govulncheck CVEs | 0 | Clean dep tree |
| Fuzz tests | PASS | 4 targets, 30s each, ~1.5M execs total, 0 production panics (1 test-harness bug found and fixed) |
| Load test /healthz @1000 rps | PASS | p99 = 0.35ms, 29900 req in 30s, 1 error (0.003%) |
| Service binary RSS after load | ~24 MB | Stable across 100/500/1000 rps test |

## Chaos test details

### `TestChaosFileStoreSurvivesPanicMidMutator`
- 100 iterations × 20 concurrent goroutines per iteration
- Each goroutine has ~33% chance of panicking mid-mutator
- After every iteration: reload `FileStore`, assert state.json parses, assert no `.tmp` files left behind
- Result: **1329 successful writes, 671 panics survived, 0 corrupt state files** in 211ms total

### `scripts/chaos/run-chaos.sh --quick`
Real `kill -9` of the running binary mid-write, 5 iterations:
```
PASS[ 1/5]  sleep=0.58s  state_size=391B
PASS[ 2/5]  sleep=0.69s  state_size=391B
PASS[ 3/5]  sleep=0.70s  state_size=391B
PASS[ 4/5]  sleep=1.61s  state_size=391B
PASS[ 5/5]  sleep=1.50s  state_size=391B
```
Production `--full` runs 50 iterations (~3 minutes).

### `TestChaosAppendPanicsLeaveNoTornLines` (JSONL)
- 50 goroutines × 100 appends, ~10% panic mid-marshal
- Result: **4509 ok, 491 panics, no torn lines on Load** in 30ms

## Security scan

### gosec HIGH findings (18 total — all reviewed & accepted)

| Rule | Count | Files | Disposition |
|---|---|---|---|
| G404 (math/rand for crypto) | 3 | `cmd/loadtest/main.go` only | Accepted — load test, not security-sensitive |
| G703 (path traversal taint) | 7 | `store.go`, `users.go`, `notify/channel.go`, `spa.go`, `incident_repository.go` | Accepted — paths constructed from trusted config (`STATE_PATH`, `DATA_DIR`); no untrusted input flows into them |
| G704 (errors not handled) | 8 | mostly `defer Close()` patterns, ai/provider.go | Accepted — standard Go idiom; failed close on read-only handles is non-recoverable |

**One real fix made**: `alert_rules.go` — tightened `0o640` → `0o600` per gosec G306. Alert rules contain notification webhook secrets; no other UID needs read access.

### govulncheck
```
No vulnerabilities found.
```
Zero CVEs against the entire dependency tree.

## Fuzz test results (30s each)

| Target | Executions | New Interesting | Panics |
|---|---|---|---|
| `FuzzParseBasicAuth` | 640,009 | 71 | 0 |
| `FuzzConfigUnmarshal` | 55,408 | 75 | 0 |
| `FuzzValidateAndDecodeCheck` | 354,735 | 157 | 0 |
| `FuzzQueryIntRange` | 360,217 | 65 | 0 (after fix) |

**One bug found**: `FuzzQueryIntRange` initially panicked with input `" "` — root cause was the *test harness* calling `httptest.NewRequest("GET", "/?limit="+raw, nil)` without URL-escaping. Fixed by `url.QueryEscape(raw)`. Production code (`QueryIntRange`) is fine — it receives already-parsed query values from `net/http`, never raw URL strings.

## Load test baseline

Hardware: macOS, Apple Silicon, single laptop. Staging numbers will differ.

| Endpoint | RPS | Duration | Requests | Errors | p50 (ms) | p95 (ms) | p99 (ms) | max (ms) |
|---|---|---|---|---|---|---|---|---|
| `/healthz` | 100 | 30s | 2999 | 0 | 0.33 | 0.47 | 0.54 | 1.79 |
| `/healthz` | 500 | 30s | 14972 | 1 | 0.12 | 0.31 | 0.45 | 7.20 |
| `/healthz` | 1000 | 30s | 29900 | 1 | 0.13 | 0.23 | 0.35 | 6.67 |

`/healthz` is auth-exempt and therefore the cleanest measurement of HTTP middleware overhead. RSS after the run: ~24 MB. No goroutine leak observed.

**Authenticated endpoints** (`/api/v1/checks`, `/api/v1/summary`) require a JWT obtained via login; load testing them is intentionally deferred to staging where realistic auth flow can be exercised.

## What's still pending

| Item | Why deferred | Where |
|---|---|---|
| 7-day soak harness | Needs to be run against a Linux VM, not macOS dev box | Will be run during staging soak |
| Mongo flap test | Requires container orchestration to bring Mongo up/down on schedule | Staging concern |
| Authenticated endpoint load tests | Needs token automation in load test | Phase B |
| CI workflows for security/fuzz | Existing `ci.yml` covers tests; `security.yml` deferred to next iteration | Future |

## Conclusion

The FileStore + JSONL persistence layer is **demonstrably durable** under concurrent writes, panics, process kills, and write failures. Crash-consistency of `state.json` is not theoretical — it has been exercised with real `kill -9` and 100s of in-process panics.

The HTTP layer handles **1000 rps with sub-millisecond p99** on a laptop, with stable memory and zero leaked goroutines.

The dependency tree is **clean of known CVEs** and the public input parsers (auth header, config JSON, query params) **survived ~1.5M random fuzz inputs** without a single production panic.

This is **not** the same as "battle-tested in production for 3 months", but it is a solid pre-staging baseline. The next milestone is a Linux soak in a real environment.
