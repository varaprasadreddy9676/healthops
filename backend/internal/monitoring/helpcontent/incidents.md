---
slug: incidents
title: Incidents
summary: A focused timeline for active and historical operational problems.
intent: Use Incidents to answer what broke, when it started, what evidence proves it, and what action is next.
category: Operate
order: 140
icon: alert-triangle
relatedPaths: /incidents
relatedTopics: checks,alert-rules,root-cause
---

# Incidents

An incident is a failing check (or alert rule, or heartbeat miss, or log signal, or MySQL signal) that crossed its open threshold. HealthOps records the evidence at the moment it opened and tracks the timeline until you resolve it.

## How an Incident Opens

- A check fails `failuresToOpen` times in a row.
- A heartbeat is missed by more than its grace period.
- An alert rule's condition holds for its required duration.
- A MySQL rule (slow query rate, lock waits, connection saturation, etc.) crosses its threshold.
- A log pattern crosses its frequency threshold.

When this happens, HealthOps captures a **snapshot** of evidence — the failing result, related metrics, recent samples — and attaches it to the incident.

## How an Incident Resolves

- A failing check succeeds `successesToResolve` times in a row.
- A heartbeat resumes pinging.
- An alert rule's condition stops being true for the configured cooldown.
- An operator explicitly resolves it.

## How To Read One

1. **Start with the evidence summary.** Do not read AI commentary first.
2. **Look at the latest observed value vs the expected threshold.** This is the most important line.
3. **Open the snapshot at incident open time.** That is the truth about what was happening when it broke.
4. **Use the timeline** to see whether the problem is getting worse, recovering, or oscillating.
5. **Then** read AI suggestions, if any. They are a second opinion.

## States

| State | Meaning |
| ----- | ------- |
| `open` | Currently failing and unacknowledged |
| `acknowledged` | Someone is on it. Notifications may pause. |
| `resolved` | Threshold for recovery met or manually closed |

## Acknowledging vs Resolving

- **Acknowledge** when you know about the incident and are working on it. Stops re-notifications during the work.
- **Resolve** when the underlying issue is fixed. Do not resolve to silence pages — that hides real failures.

## When AI Is Configured

HealthOps may enqueue automatic root-cause analysis when an incident opens. The result appears on the incident as a separate panel. Treat it as one more diagnostic, not as truth — always cross-check against the evidence snapshot.

## Common Pitfalls

- **"The dashboard is green but the incident is still open."** Resolution requires `successesToResolve` consecutive clean runs. One success after a streak of failures does not auto-close.
- **"I resolved it but it reopened immediately."** The underlying check is still failing. Look at the latest result.
- **"Too many duplicates."** A noisy check is generating many incidents. Tune `failuresToOpen` or add an alert rule with a longer window. See **Monitor Tuning**.
