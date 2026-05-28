---
slug: getting-started
title: Getting Started
summary: A 10-minute walkthrough that takes you from never-opened to confidently operating HealthOps.
intent: Read this first if you have never opened HealthOps before. It explains what the product does, what to set up, and what to look at — in the order that actually matters.
category: Start Here
order: 10
icon: rocket
relatedPaths: /
relatedTopics: quick-tour,glossary,faq
---

# Getting Started

HealthOps watches your servers, APIs, processes, databases, log streams, and scheduled jobs, then tells you when something looks wrong — with enough evidence to act, not just enough to panic.

> If you only have one minute: open the **Dashboard**, look at the four numbers across the top, and open any **incident** that is red. That is 90 % of the job.

## How It All Fits Together

```
   you define              HealthOps runs                  you see
   ──────────              ─────────────                   ───────
   ┌──────────┐  every    ┌─────────────┐   crosses    ┌────────────┐
   │ a check  │ ──60 s──▶ │  scheduler  │ ─threshold─▶ │  incident  │
   │ (api,    │           │  + workers  │              │  opens     │
   │  mysql,  │           └──────┬──────┘              └──────┬─────┘
   │  log,..) │                  │ stores                     │
   └──────────┘                  ▼                            ▼
                          ┌─────────────┐              ┌────────────┐
                          │  MongoDB    │              │ notify     │
                          │  (results,  │              │ (email,    │
                          │  incidents) │              │  webhook)  │
                          └──────┬──────┘              └────────────┘
                                 │
                                 ▼
                          ┌─────────────┐  optional   ┌────────────┐
                          │  Dashboard  │ ─────────▶  │  AI: RCA   │
                          │  + APIs     │             │  + remediation │
                          └─────────────┘             └────────────┘
```

Every screen in the product is a view onto this loop. Nothing else is happening.

## The Core Loop in Plain English

1. **You configure checks.** Small tests like "is this URL returning 200?" or "is this MySQL connection accepting writes?". Each check has a target, an interval, and thresholds.
2. **A scheduler runs them** at the cadence you picked, in parallel, with timeouts.
3. **Results are stored** in MongoDB and pruned after the retention window (default 7 days).
4. **When a check fails N times in a row**, an incident opens. N is configurable per check.
5. **Notifications fire** through your configured channels.
6. **(Optional) AI reads the incident evidence** and writes a root-cause summary with suggested fixes.
7. **(Optional) Remediation rules** auto-execute safe, pre-approved actions (e.g. restart a process, clear a queue).
8. **Analytics aggregate** results into uptime %, latency percentiles, and noisy-check rankings over 7 / 30 / 90 days.

## Your First 10 Minutes

| Minute | Do this | Why |
| ------ | ------- | --- |
| 0–1 | **Log in** with the fixed `admin` user and the first-run `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | You cannot do anything without auth |
| 1–3 | **Open the Dashboard** | See the layout; understand "healthy now" vs "currently broken" |
| 3–5 | **Open Checks** → click any seeded demo check | Understand the per-monitor view: history, last 50 results, current config |
| 5–7 | **Open Incidents** → click an open one (or recently resolved) | See evidence collection, state machine, snapshots |
| 7–9 | **Open Settings** | Glance at: users, notification channels, AI providers, retention |
| 9–10 | **Add one real check** (instructions below) | Make it yours |

## Your First Real Check

The simplest useful check is an **API check** against something you already run.

Click **Add Check** → fill in:

```json
{
  "type": "api",
  "name": "Public API healthz",
  "target": "https://yourapp.example.com/healthz",
  "expectStatus": 200,
  "intervalSeconds": 60,
  "timeoutSeconds": 5,
  "failuresToOpen": 3,
  "tags": { "server": "web-1", "application": "api" }
}
```

Save. Within `intervalSeconds` it runs once. Force a failure (stop the service or break the URL) and after 3 consecutive failures an incident opens. Recover the service and the incident auto-resolves after the next successful run.

### Threshold Sanity Defaults

| Sensitivity | `intervalSeconds` | `failuresToOpen` | Time to alert |
| ----------- | ----------------- | ---------------- | ------------- |
| Page-worthy (critical paths) | 30 | 2 | ~1 min |
| Normal production | 60 | 3 | ~3 min |
| Best-effort (batch jobs) | 300 | 2 | ~10 min |

Start at "normal" and tighten only when a real incident proves you need to.

## Your First Notification

1. **Settings → Notification Channels** → Add Channel.
2. Pick `webhook`, `email`, or `slack` (Slack uses an incoming-webhook URL).
3. Send a test message. Confirm it arrives.
4. On any check, attach the channel. Now incidents on that monitor reach you.

If the test never arrives:

- Check the **Notifications** page (outbox) — every dispatch attempt is logged with status and error.
- Webhook failed? Confirm the URL is reachable from the HealthOps host and returns HTTP 2xx.
- Email failed? Check the email channel's SMTP host/user/password-env settings and the outbox `lastError`.

## What You Do **Not** Need on Day One

| Feature | Add when... |
| ------- | ----------- |
| AI providers (BYOK) | You have an incident you cannot diagnose in 5 minutes |
| Status pages | You have external users who ask "is it down?" |
| Heartbeats | You have cron jobs / batch pipelines that must run on schedule |
| MySQL monitoring | You actually run MySQL and want connection/lock/slow-query alerts |
| Log ingestion | You want HealthOps to alert on log patterns (e.g. spike in 5xx) |
| Remediation rules | You have an incident you have manually resolved 3+ times the same way |

HealthOps is useful with just one API check. Add complexity only when justified.

## Where to Go Next

- **Quick Tour** — 30-second walkthrough of every screen.
- **Checks** — every check type explained with config examples.
- **Incidents** — the state machine, evidence, and acknowledgement flow.
- **Glossary** — every term in one place.
- **FAQ** — questions everyone asks in week one.
- **Troubleshooting** — fixes for the 12 most common setup mistakes.
