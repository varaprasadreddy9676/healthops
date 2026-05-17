---
slug: dashboard
title: Dashboard
summary: The operating overview for health, incidents, uptime, latency, and active risk.
intent: Open this first to answer one question — is the monitored estate healthy right now, and if not, what is the blast radius?
category: Operate
order: 100
icon: layout-dashboard
relatedPaths: /,/dashboard
relatedTopics: incidents,checks,analytics
---

# Dashboard

The Dashboard answers **one question** in under five seconds:

> Is anything broken right now, and if so, how bad is it?

If the answer is "no", you close the tab. If the answer is "yes", every other surface (Incidents, Checks, MySQL, Logs) is one click away with full context preserved.

## The 4 Things to Read First

Read them in this order. Each one filters the next.

| # | Element | What it tells you | When to act |
| - | ------- | ----------------- | ----------- |
| 1 | **Status counts** — healthy / warning / critical / unknown | Aggregate health across all checks | Any non-zero `critical` or `unknown` → click in |
| 2 | **Open incidents** | What is actively failing right now | Any open incident → Incidents page |
| 3 | **Live activity stream** | Checks that just ran or just changed state | Spikes of state-changes = something is flapping |
| 4 | **Latency & uptime sparklines** | Trend over the last few hours | Sudden cliff = live incident. Recovered curve = old noise. |

## How the Data Flows

```
   scheduler                    store                     dashboard
   ─────────                    ─────                     ─────────
   ┌───────────┐    runs    ┌────────────┐   reads   ┌──────────────┐
   │ Check #1  │ ─────────▶ │ MongoDB    │ ────────▶ │ Snapshot API │
   │ Check #2  │  every     │ results    │   every   │ + SSE stream │
   │   ...     │  interval  │ incidents  │   3 s     │              │
   │ Check #N  │            │ snapshots  │           └──────┬───────┘
   └───────────┘            └────────────┘                  │
                                                            ▼
                                                  ┌──────────────────┐
                                                  │  Your browser    │
                                                  │  auto-refresh    │
                                                  └──────────────────┘
```

Nothing on this page is invented. If a number looks wrong, the underlying check or incident is the source of truth — open it.

## What "Healthy" Actually Means

The dashboard rolls up the latest result of every **enabled** check. Maintenance-window'd checks are counted as `paused`, not `healthy`. A check in `unknown` state means it has never run yet, or its last run timed out — treat `unknown` as a warning, not a free pass.

## What Good Looks Like

- All counts on the top row are green or zero.
- Open incidents = 0.
- Latency sparkline is flat or trending down.
- Uptime is ≥ 99.9 % over the visible window.
- The live activity stream is calm — state changes are rare.

## What Bad Looks Like (and What to Do)

| Symptom | Likely cause | First action |
| ------- | ------------ | ------------ |
| Spike in `critical` count, latency cliff | Live incident in progress | Open Incidents → most recent |
| Many `unknown` checks at once | Scheduler stalled, host unreachable, or DB write failure | Check `/healthz` and the degraded-mode banner |
| Activity stream is flapping (state changes every few seconds) | Threshold too tight, or a service is genuinely oscillating | Open the noisy check → raise `failuresToOpen` or attach an alert rule |
| Uptime % suddenly dropped to 92 % | A check has been failing for hours and is now eroding the rolling window | Sort Checks by uptime, find the offender |
| Dashboard is empty | First-run, or all checks disabled | Open Checks → seed at least one |

## Refresh, Live Updates, and Staleness

- The dashboard auto-refreshes every 3 seconds via Server-Sent Events.
- A **Live** badge in the top-right confirms the stream is connected.
- If the badge says **Offline**, the page falls back to polling every 30 s. Refresh manually if you just added a check.
- "Stale data" indicators on individual cards mean the last sample is older than 2 × the check interval.

## Demo Mode

In demo mode, the dashboard is pre-seeded with realistic synthetic traffic — failing checks, recovering services, incidents at various ages. The label **DEMO** appears in the top-right. Disable demo mode by removing the `demo` flag from your config and restarting.

## Permissions

| Role | Can see dashboard | Can see config links |
| ---- | ----------------- | -------------------- |
| `viewer` | yes | no |
| `editor` | yes | yes |
| `admin` | yes | yes + user management |

## Common Pitfalls

- **"Everything is green but production is down."** You are not monitoring the failing path. Add a check that touches the customer-visible URL or workflow.
- **"Numbers do not add up."** Some checks are disabled or in a maintenance window. The Checks page shows per-monitor state.
- **"Sparkline has a gap."** That is the scheduler not running. Confirm the HealthOps process itself is healthy via `/healthz`.
- **"Live badge keeps flipping."** A proxy or load balancer is killing long-lived connections. Increase its idle timeout to ≥ 60 s, or accept polling fallback.

## Where to Go Next

- See a failing check → **Checks** (filter by status `critical`).
- See an open incident → **Incidents** (most recent first).
- Diagnose a noisy monitor → **Monitor Tuning**.
- Long-term trends → **Analytics** (last 7 / 30 / 90 days).
