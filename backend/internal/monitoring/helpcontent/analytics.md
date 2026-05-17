---
slug: analytics
title: Analytics
summary: Longer-window reliability metrics for checks, incidents, latency, and availability.
intent: Use Analytics to find recurring reliability patterns instead of reacting only to the latest alert.
category: Understand
order: 310
icon: bar-chart-3
relatedPaths: /analytics
relatedTopics: monitor-tuning,incidents
---

# Analytics

Analytics is for week-and-month thinking, not minute-and-hour thinking. It is where you find the noisy monitors, the chronically slow endpoints, and the services that keep regressing.

## What It Measures

- **Availability / uptime** per check or service.
- **Latency percentiles** (p50, p95, p99) and trends.
- **Incident volume** and severity distribution.
- **MTTA / MTTR** — mean time to acknowledge and to resolve.
- **Recurring noisy monitors** — checks with high incident counts and low real impact.
- **Failure rate** — how often the average check fails over the window.

## Picking a Window

- **24h** — yesterday's surprises.
- **7d** — weekly review and on-call handoff.
- **30d** — service-level reporting.
- **90d** — quarterly trends and capacity planning.

Short windows over-react to one bad day. Long windows hide recent regressions. Use both.

## Reading Latency Percentiles

- **p50** — your typical user.
- **p95** — your slowest 1-in-20 user.
- **p99** — your slowest 1-in-100. Often where the worst customer experience hides.
- Watch p95 and p99 trends. p50 rarely changes; the tail does.

## Reading MTTR

MTTR is "from open to resolve". Lower is better, but optimize honestly:

- Cutting MTTR by auto-resolving noisy alerts is **cheating** — the underlying noise stays.
- Cutting MTTR by faster diagnosis or runbooks is **real improvement**.

## What Analytics Cannot Tell You

- Whether your customers cared (use product analytics for that).
- Whether the right monitors exist (use **Monitor Tuning** for that).
- Whether the team is healthy (incident counts are a poor proxy for stress).
