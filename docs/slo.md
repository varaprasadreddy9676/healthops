# HealthOps Service Level Objectives

**Audience:** SRE owners of the HealthOps deployment; consumers who depend on
its API or alerting.

This document defines what HealthOps promises and what it does not. SLIs are
measured from the Prometheus metrics emitted by the service itself
(`backend/internal/monitoring/metrics.go`) and from the reverse-proxy access
log. Targets reflect the service's classification as an internal
infrastructure tool, not a customer-facing product.

For deployment guidance see [deployment.md](deployment.md). For backup
expectations see [backups.md](backups.md). For incident playbooks see
[runbook.md](runbook.md).

---

## 1. Service classification

- **Tier:** internal infrastructure / observability tool.
- **Audience:** the on-call team and a small number of operators.
- **Coverage:** business-hours SRE attention; out-of-hours pages only when
  HealthOps itself is part of the critical path of another paged incident.
- **Topology:** single instance per environment, no multi-region active-active.

Targets below are deliberately modest. Tightening them requires either
horizontal redundancy or moving HealthOps into a higher-tier classification.

---

## 2. SLIs and SLOs

All percentages are computed over the rolling window stated. Each SLI is
phrased as "good events / valid events".

### SLI 1 — API availability

**What it measures:** the fraction of API requests that did not return a
server error.

**Metric source:** `healthops_http_requests_total{endpoint, status}` from the
service. The proxy access log is an acceptable secondary source.

**Formula (PromQL, 30-day window):**

```promql
sum(rate(healthops_http_requests_total{status!~"5.."}[30d]))
/
sum(rate(healthops_http_requests_total[30d]))
```

**SLO:** 99.5% over 30 days.
**Error budget:** 0.5% = ~3h 36m of fully-failing API per 30-day window.

Rationale: a single-instance service behind a single reverse proxy cannot
credibly promise more than two-nines plus. Routine restarts (deploys, OS
patching) and the occasional Mongo blip eat into the budget; pad accordingly.

### SLI 2 — API latency

**What it measures:** the fraction of API requests served fast enough for the
UI to feel responsive.

**Metric source:** `healthops_http_request_duration_seconds` histogram.

**Formula (PromQL, 5-minute window — proportion under 300ms):**

```promql
sum(rate(healthops_http_request_duration_seconds_bucket{le="0.3"}[5m]))
/
sum(rate(healthops_http_request_duration_seconds_count[5m]))
```

**SLO:** at least 95% of requests served in <300ms over each 5-minute window.
**Error budget:** 5% slow requests per window.

Note on bucket boundaries: the histogram buckets defined in `metrics.go` are
`[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]`. The
buckets that bracket 300ms are `le="0.25"` and `le="0.5"`. `le="0.3"` will
linearly interpolate between them; if you need an exact bucket boundary use
`le="0.5"` and accept a slightly looser SLI.

If you also want a p99 expression for dashboarding (not for the SLI):

```promql
histogram_quantile(
  0.99,
  sum(rate(healthops_http_request_duration_seconds_bucket[5m])) by (le)
)
```

### SLI 3 — Check freshness

**What it measures:** whether checks are actually running on schedule. A
HealthOps that returns 200 to its API while its scheduler is wedged is worse
than one that is honestly down.

**Metric source:** `healthops_check_runs_total` (counter, increments per
execution) and `healthops_scheduler_lag_seconds` (gauge). A check is
"fresh" if its last run finished within `2 × intervalSeconds` ago.

**Implementation:** Compute freshness from `/api/v1/dashboard`, which exposes
each check's `lastRunAt`, or maintain a recording rule that emits a
`healthops_check_fresh` gauge per `check_id`:

```promql
# Pseudo-rule: 1 if the check has incremented in the last 2 minutes
# (assuming a 60s default interval — adjust per check)
clamp_max(
  increase(healthops_check_runs_total[2m]),
  1
)
```

Then:

```promql
avg(healthops_check_fresh)
```

A coarser proxy that needs no recording rule is scheduler lag. If the
scheduler is keeping up, freshness is healthy:

```promql
healthops_scheduler_lag_seconds < 10
```

**SLO:** 99% of enabled checks fresh in any 5-minute window.
**Error budget:** 1% stale checks at any moment (i.e., for 100 checks, at
most 1 may be stale at any time).

---

## 3. Burn-rate alerts

Burn-rate alerting pages when the error budget is consumed faster than the
SLO permits. Use the standard fast-burn / slow-burn pair so that fast outages
page immediately and slow regressions surface as tickets.

### Fast burn — 1h window @ 14.4× rate (page immediately)

```promql
# Availability fast burn
(
  1 - (
    sum(rate(healthops_http_requests_total{status!~"5.."}[1h]))
    /
    sum(rate(healthops_http_requests_total[1h]))
  )
) > (14.4 * 0.005)
```

### Slow burn — 6h window @ 6× rate (ticket / Slack)

```promql
# Availability slow burn
(
  1 - (
    sum(rate(healthops_http_requests_total{status!~"5.."}[6h]))
    /
    sum(rate(healthops_http_requests_total[6h]))
  )
) > (6 * 0.005)
```

Apply the same template to:

- **Latency** — replace numerator/denominator with the latency SLI; budget = 0.05.
- **Freshness** — replace with the freshness SLI; budget = 0.01.

Every burn-rate alert MUST link to the [runbook](runbook.md) section for the
SLI it covers.

### Auxiliary signals (do not page on these alone)

- `healthops_scheduler_lag_seconds > 30` for 5 min — scheduler falling
  behind. Route as a ticket; freshness SLO will follow.
- `up{job="healthops"} == 0` for >2 min — instance has stopped scraping.
  Pair with an external blackbox probe before paging to avoid Prometheus
  outage false positives.
- `rate(healthops_check_failures_total[10m]) > <baseline>` — change in the
  failure rate of monitored systems. This is information about the systems
  HealthOps watches, not about HealthOps itself.

---

## 4. What HealthOps does NOT promise

Be explicit so consumers do not build downstream systems on assumptions that
will burn them later.

- **Not a real-time alerter.** Time-to-fire is bounded by check
  `intervalSeconds` plus rule cooldowns plus alert channel latency.
  End-to-end notification under 60 seconds is typical, not guaranteed.
- **Not multi-region.** A region outage takes the service down. Plan for a
  manual cutover to the secondary if you stand one up.
- **Not high-availability within a region.** Single instance, single
  filesystem. Restarts cause brief 5xx; deploys consume budget.
- **MongoDB mirror is not authoritative.** It is best-effort, lossy under
  partition. Treat it as a read-replica, never as the source of truth.
- **Not a long-term metrics store.** Per-check raw results are pruned after
  `retentionDays` (default 7). Use the Prometheus scrape for retention
  beyond that.
- **No guaranteed delivery for alert notifications.** The outbox retries
  webhook/email failures, but email and third-party webhooks can drop
  messages. Critical alerting paths must have a second channel.
- **No SLA.** This is an internal SLO, not a contractual SLA. Breaching it
  consumes budget and triggers a review; it does not create financial
  liability.

---

## 5. Review cadence

- **Monthly:** review burn over the last 30 days. If any SLO breached its
  budget, freeze risky changes (allowCommandChecks toggles, schema
  migrations, Mongo upgrades) until the budget recovers.
- **Quarterly:** revisit the targets themselves. If the actual workload has
  grown, tighten the latency target. If business-hours-only is no longer
  acceptable, reclassify and add redundancy before promising more.
- **After every Sev-1 incident:** re-evaluate burn-rate thresholds and
  whether the breached SLO was the right one.
