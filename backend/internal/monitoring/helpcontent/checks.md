---
slug: checks
title: Checks
summary: The monitoring rules HealthOps runs on a schedule — every type explained with config examples and tuning guidance.
intent: Use Checks to define what HealthOps should test, how often, with what timeout, and when failure should become a real incident.
category: Operate
order: 120
icon: check-square
relatedPaths: /checks
relatedTopics: alert-rules,incidents,heartbeats,monitor-tuning
---

# Checks

A check is **one repeatable test of one thing**. Every check has the same five-line shape:

- A **type** (api, mysql, log, ...).
- A **target** (URL, DSN, file path, ...).
- An **interval** (how often to run).
- A **timeout** (how long to wait).
- **Thresholds** that decide when failing means *incident*, not *blip*.

Everything else (tags, notification channels, alert rules) is optional.

## The 12 Built-In Check Types

| Type | What it tests | Typical use |
| ---- | ------------- | ----------- |
| `api` | HTTP(S) status, latency, optional body substring | Public health endpoints |
| `tcp` | Port open and accepting | Internal services, message brokers |
| `ping` | ICMP reachability | Network reachability, VPN tunnels |
| `dns` | Hostname resolves to expected record | Detect DNS misconfig before users do |
| `ssl` | TLS cert valid and days-to-expiry | Catch expiring certs 30 days out |
| `domain` | Domain registration expiry | Avoid the "we forgot to renew" outage |
| `process` | Process matching keyword is running (via SSH) | Daemons, queue workers |
| `command` | Arbitrary shell command exit code + output (via SSH) | Custom probes you cannot express otherwise |
| `ssh` | SSH login succeeds | Bastion/jumpbox availability |
| `mysql` | Connections, slow queries, locks, replication (via DSN) | Database health |
| `log` | File freshness or pattern match (via SSH) | Detect log silence or error spikes |
| `heartbeat` | External job pings HealthOps before deadline | Cron jobs, batch pipelines |

> `process`, `command`, `log`, and `ssh` require the check to set `serverId` pointing at a server entry with SSH credentials.

## Anatomy of a Check

The full shape with every common field:

```json
{
  "id": "api-checkout",
  "name": "Checkout API",
  "type": "api",
  "target": "https://shop.example.com/healthz",
  "intervalSeconds": 60,
  "timeoutSeconds": 5,
  "expectedStatus": 200,
  "expectedSubstring": "ok",
  "failuresToOpen": 3,
  "successesToResolve": 2,
  "warningLatencyMs": 800,
  "criticalLatencyMs": 2000,
  "enabled": true,
  "tags": { "server": "web-1", "application": "checkout", "env": "prod" },
  "notificationChannelIds": ["pager-prod"]
}
```

You rarely need all of these. Start with `type`, `target`, `intervalSeconds`, `failuresToOpen`. Add more as the use case demands.

## Threshold Math — Why Single Failures Should Not Page

A single failure is a coin flip — DNS hiccup, brief network blip, a GC pause, anything. HealthOps treats *streaks* as signal:

```
   run 1   run 2   run 3   run 4   run 5     result
   ─────   ─────   ─────   ─────   ─────     ──────
    OK      OK      OK      OK      OK       healthy
    OK     FAIL     OK      OK      OK       healthy  (1 failure, no streak)
    OK     FAIL    FAIL    FAIL     OK       INCIDENT (failuresToOpen=3 hit)
    OK     FAIL    FAIL    FAIL    FAIL      INCIDENT continues
    OK     FAIL    FAIL    FAIL     OK       INCIDENT still open
    OK      OK      OK      OK      OK       resolves only after successesToResolve=2 streak
```

Tune the two thresholds per monitor:

| Monitor sensitivity | `intervalSeconds` | `failuresToOpen` | `successesToResolve` | Time to alert | Time to clear |
| ------------------- | ----------------- | ---------------- | -------------------- | ------------- | ------------- |
| Page-worthy (critical) | 30 | 2 | 2 | ~1 min | ~1 min |
| Normal production | 60 | 3 | 2 | ~3 min | ~2 min |
| Flaky third-party | 60 | 5 | 3 | ~5 min | ~3 min |
| Batch / best-effort | 300 | 2 | 1 | ~10 min | ~5 min |

## Pick the Right Interval

Shorter intervals are not always better — they cost target load and produce more storage. Rule of thumb:

- **30 s** — customer-facing critical paths only.
- **60 s** — almost everything else.
- **300 s** — slow third parties, batch systems.
- **>300 s** — domain expiry, SSL expiry, anything that changes daily at best.

## API Checks — The 80 % Case

Most production monitoring is just "does this URL respond, fast enough, with the right body". Use:

```json
{
  "type": "api",
  "target": "https://api.example.com/healthz",
  "expectedStatus": 200,
  "expectedSubstring": "\"status\":\"ok\"",
  "warningLatencyMs": 500,
  "criticalLatencyMs": 1500
}
```

If your `/healthz` returns 200 even when the database is down, it is lying. Use a deeper readiness endpoint that actually exercises dependencies.

## TCP Checks — When HTTP Is Not There

```json
{ "type": "tcp", "target": "redis-prod:6379", "timeoutSeconds": 3 }
```

Use for: Postgres, Redis, Kafka brokers, message queues, anything that does not speak HTTP.

## MySQL Checks — See the Dedicated Page

See **MySQL** for the full guide. Short version:

```json
{
  "type": "mysql",
  "name": "primary-mysql",
  "dsnEnv": "MYSQL_PRIMARY_DSN",
  "intervalSeconds": 30
}
```

The DSN is **only ever read from an environment variable**, never stored or shown.

## Log Checks — Two Modes

```json
// Mode 1: freshness — alert if file has not been written recently
{ "type": "log", "serverId": "web-1", "target": "/var/log/app.log", "freshnessSeconds": 300 }

// Mode 2: pattern — alert when a regex matches more than N times in window
{ "type": "log", "serverId": "web-1", "target": "/var/log/app.log",
  "pattern": "ERROR|FATAL", "matchThreshold": 10, "windowSeconds": 300 }
```

## Heartbeat Checks — Inverted Monitoring

For cron jobs and batch pipelines, the job pings HealthOps when it succeeds. If the ping does not arrive within the grace window, an incident opens. See **Heartbeats**.

## State Transitions

| Current state | Result of last run | Next state |
| ------------- | ------------------ | ---------- |
| `healthy` | OK | `healthy` |
| `healthy` | FAIL (streak < threshold) | `warning` |
| `warning` | FAIL (streak == threshold) | `critical` → opens incident |
| `critical` | OK (streak < resolve) | `critical` |
| `critical` | OK (streak == resolve) | `healthy` → resolves incident |
| any | (never run or timed out) | `unknown` |

## Disable vs Delete

| Action | History | Future runs | Use when |
| ------ | ------- | ----------- | -------- |
| **Disable** | kept | stopped | Temporarily silence (maintenance, migrations) |
| **Maintenance window** | kept | run but suppress alerts | Planned downtime |
| **Delete** | removed (after retention) | stopped | Monitor is no longer relevant |

## How To Add a Check

| Method | Where |
| ------ | ----- |
| UI | **Checks → Add Check** |
| API | `POST /api/v1/checks` with the JSON above |
| Seed (first-run only) | `backend/config/default.json` |

After the first run, MongoDB is the source of truth. Edits to `default.json` are **ignored**.

## Common Pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| Check spams alerts every few minutes | Threshold too tight or target genuinely flaps | Raise `failuresToOpen`, or attach an alert rule with `forDuration` |
| Passes from your laptop, fails in HealthOps | Network egress differs (firewall, proxy, DNS, geo) | Check from the HealthOps host, not your laptop |
| Log freshness always green even when app is dead | Health log gets written by a separate buffer thread | Watch a file that only changes during real activity |
| MySQL check fails with "auth error" | DSN env var missing or wrong | Confirm `dsnEnv` is set on the process running HealthOps |
| SSH check fails | Key missing on the HealthOps host, or wrong user | Confirm key permissions (600), confirm `serverId` user |
| `unknown` state persists | Check has never completed within timeout | Raise `timeoutSeconds` or fix the target |

## Where to Go Next

- **Monitor Tuning** — fix noisy or under-sensitive monitors.
- **Alert Rules** — build conditions across multiple checks/metrics.
- **MySQL** — the deepest check type, with its own snapshot/delta model.
- **Heartbeats** — for jobs that should ping you, not be polled.
- **API Quickstart** — manage checks via API for IaC.
