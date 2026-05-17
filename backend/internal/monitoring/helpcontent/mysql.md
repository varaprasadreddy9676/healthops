---
slug: mysql
title: MySQL
summary: Database health, connections, slow queries, replication signals, and process analysis.
intent: Use this page when application errors may be caused by database saturation, lock contention, slow queries, or connectivity.
category: Detect
order: 210
icon: database
relatedPaths: /mysql
relatedTopics: alert-rules,checks,incidents
---

# MySQL

The MySQL page surfaces database health independently of any single check. It runs a periodic collector against each MySQL DSN you configure.

## What HealthOps Reads

- Connection utilization and active sessions
- Slow query digests and per-second query throughput
- Thread and process list state (active, waiting, blocking)
- Replication lag (when applicable)
- Server version, uptime, key status counters (`SHOW GLOBAL STATUS` and `SHOW GLOBAL VARIABLES`)

Every sample is stored. Deltas are computed between consecutive samples so you see rates, not raw counters.

## Required Setup

Add a MySQL check with a `dsnEnv` pointing at an environment variable on the HealthOps host:

```json
{
  "id": "mysql-prod",
  "name": "Prod MySQL",
  "type": "mysql",
  "dsnEnv": "MYSQL_PROD_DSN",
  "intervalSeconds": 60
}
```

Then set the env var on the HealthOps process: `MYSQL_PROD_DSN=user:pass@tcp(host:3306)/`. The DSN is **never** logged or returned by the API.

## Required Privileges

The monitored user should have minimum read access:

```sql
GRANT PROCESS, REPLICATION CLIENT, SELECT ON performance_schema.* TO 'healthops'@'%';
```

Do not grant write permissions. HealthOps does not need them.

## What the Rules Do

Nine default rules ship with HealthOps:

| Rule | What it watches |
| ---- | --------------- |
| Connection utilization | `Threads_connected / max_connections` |
| Slow query rate | `Slow_queries` delta |
| Long-running queries | Process list duration |
| Locks waited | `Innodb_row_lock_waits` |
| Lock wait time | `Innodb_row_lock_time` rate |
| Deadlocks | `Innodb_deadlocks` |
| Replica lag | `Seconds_Behind_Source` |
| Query throughput drop | `Questions` delta below baseline |
| Restart detected | Uptime decreasing |

Tune any of these from **Alert Rules**. Disable any that are noisy for your workload.

## Per-Incident Snapshots

When a MySQL rule opens an incident, HealthOps captures a snapshot (process list, slow query digest, current variables) so you can investigate even after the database returns to normal.

## Common Pitfalls

- **"Connection utilization is high but the app is fine."** Connection pool may be sized aggressively. Tune `max_connections` or the app pool.
- **"Slow query rule keeps firing on backup windows."** Put backups inside a maintenance window.
- **"Replica lag is always non-zero."** That is normal. Tune the threshold; only sustained lag matters.
