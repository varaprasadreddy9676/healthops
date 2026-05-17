---
slug: alert-rules
title: Alert Rules
summary: Configurable thresholds that decide when a metric crosses into "fire an incident" territory.
intent: Use alert rules when a simple pass/fail check is not enough — for example, when a value must stay above a threshold for some time before it counts.
category: Operate
order: 150
icon: bell-ring
relatedPaths:
relatedTopics: checks,incidents,monitor-tuning
---

# Alert Rules

Some monitors are not pass/fail. They are about a metric staying within a band. Alert rules express that.

## When You Need Alert Rules

- "Open an incident if MySQL connection utilization is above 90% for 5 minutes."
- "Warn if a check's p95 latency is above 800ms for the last 10 samples."
- "Open an incident if more than 5 of these checks fail at the same time."

## Components of a Rule

| Field | Meaning |
| ----- | ------- |
| `id` | Unique identifier |
| `name` | Human-readable description |
| `target` | What the rule watches (a check, a metric, a category) |
| `condition` | Comparator and threshold (e.g., `value > 90`) |
| `window` | How long the condition must hold |
| `cooldown` | Minimum time between consecutive firings |
| `severity` | `info`, `warning`, `critical` |
| `channelIds` | Channels to notify when it fires |

## How Rules Interact With Check Thresholds

A check threshold (`failuresToOpen`) handles binary pass/fail noise. An alert rule handles graded thresholds and time windows. They are complementary, not duplicates.

- For a simple "is the URL 200?" → use check thresholds.
- For "MySQL slow query rate above X" → use an alert rule on MySQL metrics.

## Cooldown Prevents Flapping

If a rule's condition oscillates around the threshold, cooldown prevents a storm of incidents. A 10-minute cooldown means once it fires, it will not fire again for 10 minutes regardless of what the metric does.

## Default MySQL Rules

HealthOps ships with 9 default MySQL rules: connection utilization, slow query rate, lock waits, replication lag, query throughput drops, deadlocks, thread saturation, error rate, uptime restart detection. Open **Alert Rules** to inspect and tune.

## Authoring a Custom Rule

`POST /api/v1/alert-rules`:

```json
{
  "name": "API p95 too slow",
  "target": { "type": "check", "id": "api-checkout" },
  "metric": "latencyP95",
  "condition": { "op": ">", "value": 800 },
  "windowSeconds": 600,
  "cooldownSeconds": 900,
  "severity": "warning",
  "channelIds": ["slack-platform"]
}
```

## Common Pitfalls

- **"My rule never fires."** The window is longer than the metric history available, or the metric name is wrong. Test on a recent known-bad period.
- **"My rule fires every minute."** Cooldown is too short, or the condition is too tight. Widen the threshold or extend cooldown.
- **"Two rules fire on the same problem."** That is fine — they may have different severities or channels. If genuinely duplicate, consolidate.
