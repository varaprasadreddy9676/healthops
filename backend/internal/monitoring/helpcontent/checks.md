---
slug: checks
title: Checks
summary: The monitoring rules that HealthOps runs on a schedule.
intent: Use Checks to define what HealthOps should test, how often, and when failure should become an incident.
category: Operate
order: 120
icon: check-square
relatedPaths: /checks
relatedTopics: alert-rules,incidents,heartbeats
---

# Checks

A check is one repeatable test of one thing. Each check has a type, a target, a schedule, a timeout, thresholds, and optional notification channels.

## Check Types

| Type | What it tests |
| ---- | ------------- |
| `api` | HTTP/HTTPS endpoint — status code, latency, optional response substring |
| `tcp` | Port open and reachable |
| `ping` | ICMP reachability |
| `dns` | Resolution of a hostname to expected records |
| `ssl` | TLS certificate validity and days-to-expiry |
| `domain` | Domain registration expiry |
| `process` | Process matching a keyword is running on a target server (via SSH) |
| `command` | Arbitrary shell command — exit code and optional output substring (via SSH) |
| `ssh` | SSH login succeeds |
| `mysql` | Database health, connections, queries (via DSN env var) |
| `log` | File freshness or content on a host (via SSH) |
| `heartbeat` | External job pings HealthOps before a deadline |

`process`, `command`, `log`, and `ssh` types require the check to set `serverId` pointing at a configured server with SSH credentials.

## Anatomy of a Check

```json
{
  "id": "api-checkout",
  "name": "Checkout API",
  "type": "api",
  "target": "https://shop.example.com/health",
  "intervalSeconds": 60,
  "timeoutSeconds": 5,
  "expectedStatus": 200,
  "expectedSubstring": "ok",
  "failuresToOpen": 3,
  "successesToResolve": 2,
  "warningLatencyMs": 800,
  "tags": ["prod", "shop"],
  "notificationChannelIds": ["pager-prod"]
}
```

## Thresholds, Not Single Failures

A single failure rarely means a real incident. HealthOps uses:

- **`failuresToOpen`** — consecutive failures before an incident opens. Defaults to 3.
- **`successesToResolve`** — consecutive successes before the incident closes. Defaults to 2.

Tune these per monitor. A latency-sensitive endpoint may need `failuresToOpen: 1`; a flaky third-party may need `5`.

## Intervals

Shorter is not always better. A 60-second interval is enough for most production systems. 10 seconds is justified only when sub-minute detection matters and the target can handle the load. Use `300` (5 minutes) for slow integrations.

## Disabling vs Deleting

- **Disable** keeps history and configuration intact; the scheduler skips the check.
- **Delete** removes the check and all its future history. Past results may still be visible until retention prunes them.

## How To Add One

Easiest: **Checks page → Add Check**. Programmatic: `POST /api/v1/checks` (see API Quickstart).

## Common Pitfalls

- **"My check just spams alerts."** Raise `failuresToOpen` or attach an alert rule with longer windows.
- **"My check passes locally but fails here."** The HealthOps host has different network egress and may need a proxy or firewall rule.
- **"Log file check is always fresh even when the app is dead."** The app may still buffer-write to that file even when broken. Pick a file that only gets written during real activity.
