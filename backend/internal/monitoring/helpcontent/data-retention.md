---
slug: data-retention
title: Data Retention
summary: How long each category of HealthOps data is kept, and how to change it.
intent: Tune retention to fit your disk budget and compliance requirements.
category: Admin
order: 550
icon: hard-drive
relatedPaths:
relatedTopics: settings,security
---

# Data Retention

HealthOps generates a lot of data. The retention job prunes daily. Defaults are conservative; adjust to your needs.

## Categories and Defaults

| Category | Default | Env override | Where stored |
| -------- | ------- | ------------ | ------------ |
| Check results | `retentionDays` from config (default 7) | `RETENTION_DAYS` | MongoDB |
| MySQL samples + deltas | 14 days | `RETENTION_SNAPSHOTS_DAYS` | MongoDB |
| Incident snapshots | 14 days | `RETENTION_SNAPSHOTS_DAYS` | MongoDB |
| Server metrics | 14 days | `RETENTION_SNAPSHOTS_DAYS` | MongoDB |
| Notifications outbox | 30 days | `RETENTION_NOTIFICATIONS_DAYS` | MongoDB |
| AI queue | 7 days | `RETENTION_AI_QUEUE_DAYS` | MongoDB |
| AI results | tracked per-incident, follow incident retention | — | MongoDB |
| Incidents | 90 days | `RETENTION_INCIDENT_DAYS` | MongoDB |
| Audit log | indefinite by default — export externally | — | MongoDB |
| Log events | 7 days | `LOG_RETENTION_DAYS` | MongoDB |

## How the Pruner Works

A background job runs daily. For each category it deletes entries older than the configured window. The pruner is safe to run while traffic is live. MongoDB TTL indices back up the prune job.

## When to Increase

- **Compliance** requires N-year audit retention → ship audit to your SIEM and keep a sane local window.
- **Incident review** requires longer than 14 days of snapshots → raise `RETENTION_SNAPSHOTS_DAYS` to 30 or 60.
- **Trend analysis** requires longer than 7 days of results → raise `RETENTION_DAYS` to 30. Watch disk.

## When to Decrease

- **Disk pressure** → lower categories that grow fastest, usually `LOG_RETENTION_DAYS` and `RETENTION_DAYS`.
- **GDPR / "right to forget" workflows** → shorter retention reduces exposure.

## Disk Budget Estimation

Rough rules of thumb for 100 active checks at 60s interval:

- Check results: ~50 MB / day
- MySQL samples (per DSN, 60s): ~20 MB / day
- Server metrics (per host, 60s): ~10 MB / day
- Notification outbox: ~1 KB per notification
- Log events: depends entirely on ingestion volume

Multiply by your retention window for total disk.

## What Retention Does NOT Cover

- The audit collection grows indefinitely by default. Ship and archive externally.
- The frontend cache (browser localStorage) is per-user; clear it when you change auth.

## Resetting Data

To start fresh:

```bash
systemctl stop healthops
# Drop the MongoDB database (replace with your database name)
mongosh --eval 'db.dropDatabase()' healthops
systemctl start healthops
```

This wipes all state. Bootstrap envs will re-create the admin user.
