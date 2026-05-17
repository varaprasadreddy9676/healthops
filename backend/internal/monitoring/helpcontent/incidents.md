---
slug: incidents
title: Incidents
summary: A focused timeline of every operational problem — open, acknowledged, or resolved — with the evidence captured at the moment it broke.
intent: Use Incidents to answer what broke, when it started, what evidence proves it, who is on it, and what action is next.
category: Operate
order: 140
icon: alert-triangle
relatedPaths: /incidents
relatedTopics: checks,alert-rules,root-cause,remediation
---

# Incidents

An incident is the system's memory of a real problem. Unlike a one-off failing result, an incident is **persistent, evidenced, acknowledgeable, and assignable**. It exists from the moment a threshold is crossed until you (or the system) prove the problem is gone.

## The Incident State Machine

```
                       failuresToOpen reached
   ┌──────────┐            (or rule fires)         ┌────────────┐
   │  (none)  │ ─────────────────────────────────▶ │    open    │
   └──────────┘                                    └──────┬─────┘
                                                          │
                                              operator    │     successesToResolve
                                              clicks ack  │     reached, OR
                                                          ▼     operator resolves
                                                  ┌──────────────┐
                                                  │ acknowledged │
                                                  └──────┬───────┘
                                                          │
                                                          ▼
                                                  ┌──────────────┐
                                                  │   resolved   │
                                                  └──────────────┘
                                                          │
                                              re-fails    │   auto-reopen if
                                              within     │   threshold crosses
                                              cooldown   ▼   again within window
                                                  ┌──────────────┐
                                                  │  open (re)   │
                                                  └──────────────┘
```

## What Triggers an Incident to Open

| Source | Trigger |
| ------ | ------- |
| **Check** | Fails `failuresToOpen` consecutive runs |
| **Heartbeat** | No ping for longer than `gracePeriodSeconds` |
| **Alert rule** | `condition` evaluates true for `forDuration` |
| **MySQL rule** | Built-in or custom rule crosses its threshold (slow queries, lock waits, connection saturation, etc.) |
| **Log event** | Pattern matches `>= threshold` times within window |

The moment any of these fire, HealthOps **captures a snapshot** — failing result, surrounding samples, related metrics, host info, recent log lines if available — and freezes it on the incident. That snapshot never changes, even if you fix the bug and the metric recovers. It is your post-mortem evidence.

## What Resolves an Incident

| Source | Recovery condition |
| ------ | ------------------ |
| Check | `successesToResolve` consecutive clean runs |
| Heartbeat | A ping arrives within window |
| Alert rule | Condition is false for `cooldownSeconds` |
| Manual | Operator clicks **Resolve** |

A single good result after a long red streak is **not** enough. This is intentional — flapping services would otherwise spam your operators with open/resolve pairs.

## Reading an Incident Page — In This Order

1. **Title + severity** — what broke, how bad.
2. **Latest observed value vs threshold** — the one line that proves the system is right to be alarmed.
3. **Open snapshot** — the evidence at incident-open time. This is canonical truth.
4. **Timeline** — is it getting worse, holding, or recovering?
5. **Related incidents** — has this same monitor flapped recently? (If yes → tune it.)
6. **AI root-cause panel** — if AI is configured, this is the system's hypothesis. Treat as a second opinion, not gospel.
7. **Remediation history** — what auto-actions ran (if any), what they returned.

## Severity Ladder

| Severity | Meaning | Default behavior |
| -------- | ------- | ---------------- |
| `critical` | Customer-facing impact likely | Page immediately, no batching |
| `warning` | Degraded but functional | Notify, do not page |
| `info` | Anomalous but acceptable | Log only |

Severity is derived from the check or rule that opened the incident — you set it once on the source, not per-incident.

## Acknowledge vs Resolve — Do Not Confuse Them

| Action | Use when... | Effect |
| ------ | ----------- | ------ |
| **Acknowledge** | You see it and are working on it | Stops re-notifications. State = `acknowledged`. Timer keeps running. |
| **Resolve** | The underlying issue is actually fixed | Closes the incident, records MTTR |

> Never resolve an incident to silence pages. Acknowledge it instead. Resolving while the system is still broken hides reality from analytics and from your team.

## What "Good Incident Hygiene" Looks Like

- Median time-to-acknowledge (MTTA) under 5 minutes during business hours.
- Median time-to-resolve (MTTR) under 30 minutes for `critical`.
- Zero re-opens within 1 hour of resolution (re-opens mean you resolved too early).
- Fewer than 3 incidents per monitor per week (more than that = noisy monitor, tune it).

The **Analytics** page surfaces all four numbers per monitor.

## When AI Is Configured

If BYOK AI is set up, HealthOps auto-enqueues a root-cause analysis when an incident opens (or you can trigger one on-demand). The result appears in a panel on the incident with:

- A hypothesised root cause (one sentence).
- Three to five contributing signals from the evidence snapshot.
- Suggested next actions, ranked by reversibility.
- An "evidence trail" — every metric/log line the analysis used.

The AI never resolves incidents. It never executes remediation. It is read-only commentary.

## Common Pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| Incident says open, dashboard says green | `successesToResolve` not yet met | Wait one or two more intervals |
| Incident reopens within seconds | Underlying check still failing | Look at latest result, not the snapshot |
| Same monitor produces 20 incidents/day | Threshold too tight or service is flapping | Raise `failuresToOpen`, or use an alert rule with a longer `forDuration` |
| Snapshot is empty | Incident opened on a heartbeat miss with no recent samples | Expected — heartbeat snapshots only show the gap |
| AI panel is missing | No AI provider configured, or analysis still queued | Settings → AI → check provider health |
| Resolved but no MTTR recorded | Incident was resolved without an open snapshot (manual create) | Always let the system open incidents itself |

## API Surface (quick reference)

| Endpoint | Purpose |
| -------- | ------- |
| `GET /api/v1/incidents` | List, with filters: `state`, `severity`, `monitorId`, `since` |
| `GET /api/v1/incidents/{id}` | Full incident with snapshot |
| `POST /api/v1/incidents/{id}/acknowledge` | Mark acknowledged |
| `POST /api/v1/incidents/{id}/resolve` | Force resolve (records actor) |
| `GET /api/v1/incidents/{id}/snapshots` | Evidence snapshots over time |

See **API Reference** for full request/response shapes.

## Where to Go Next

- **Alert Rules** — design conditions that catch the right things, not the loud things.
- **Monitor Tuning** — diagnose and fix flapping monitors.
- **RCA Reports** — automatically generated post-mortems for resolved incidents.
- **Remediation** — auto-execute safe fixes when an incident opens.
