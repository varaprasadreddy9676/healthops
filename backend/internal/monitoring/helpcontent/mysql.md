---
slug: mysql
title: MySQL
summary: Database health, connections, slow queries, replication signals, and process analysis — all from a single read-only DSN.
intent: Open this when application errors may be caused by database saturation, lock contention, slow queries, replication lag, or connectivity. It is the deepest check type HealthOps ships.
category: Detect
order: 210
icon: database
relatedPaths: /mysql
relatedTopics: alert-rules,checks,incidents,mysql-connections,mysql-queries,mysql-threads,mysql-server
---

# MySQL

MySQL monitoring in HealthOps is *not* "ping the port and call it healthy". It runs a periodic collector that:

1. Connects with a read-only user.
2. Pulls `SHOW GLOBAL STATUS`, `SHOW GLOBAL VARIABLES`, and `INFORMATION_SCHEMA.PROCESSLIST`.
3. Stores the raw sample.
4. Computes a **delta** against the previous sample (so you see rates, not cumulative counters).
5. Evaluates 9 default rules against the delta. Each rule can open an incident with a full snapshot.

```
   every 30-60 s
   ┌────────────────┐    SHOW GLOBAL STATUS      ┌──────────────┐
   │   collector    │ ─────────────────────────▶ │  MySQL host  │
   │   (1 per DSN)  │ ◀───────────────────────── │              │
   └───────┬────────┘   raw counters             └──────────────┘
           │
           ▼
   ┌────────────────┐   delta vs previous   ┌────────────────┐
   │  sample store  │ ────────────────────▶ │  rule engine   │
   │  (Mongo)       │                       │  (9 defaults)  │
   └────────────────┘                       └───────┬────────┘
                                                    │ threshold crossed
                                                    ▼
                                            ┌────────────────┐
                                            │   incident +   │
                                            │   snapshot     │
                                            └────────────────┘
```

## What HealthOps Reads

| Signal | Source | Why it matters |
| ------ | ------ | -------------- |
| Connection utilization | `Threads_connected / max_connections` | Saturation = new clients fail to connect |
| Active sessions | `Threads_running` | Spikes = workload stuck on a slow query |
| Slow query rate | `Slow_queries` delta | Symptom of bad indexes or load spike |
| Query throughput | `Questions` delta | Sudden drop = upstream stopped sending traffic |
| Row lock waits | `Innodb_row_lock_waits` | Contention — usually a hot row or missing index |
| Lock wait time | `Innodb_row_lock_time` rate | How long writers are stalling |
| Deadlocks | `Innodb_deadlocks` | Tx ordering bugs |
| Replica lag | `Seconds_Behind_Source` | Reads on the replica are stale |
| Uptime | `Uptime` | Decreasing value = the server restarted |
| Process list | `INFORMATION_SCHEMA.PROCESSLIST` | What is *actually running right now* |

Everything is stored. Nothing is sampled in-memory and discarded.

## Required Setup — 3 Steps

### 1. Create a read-only monitoring user on the MySQL host

```sql
CREATE USER 'healthops'@'%' IDENTIFIED BY 'choose-strong-password';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'healthops'@'%';
GRANT SELECT ON performance_schema.* TO 'healthops'@'%';
FLUSH PRIVILEGES;
```

**Do not grant any write privileges.** HealthOps never writes. If your security policy requires further restriction, drop `REPLICATION CLIENT` (you will lose replica-lag monitoring only).

### 2. Set the DSN on the HealthOps host as an environment variable

```bash
export MYSQL_PROD_DSN='healthops:choose-strong-password@tcp(db-prod.internal:3306)/'
```

The DSN is **never** logged, returned by the API, or visible in the UI. Only the env var name is stored.

### 3. Add the check

```json
{
  "id": "mysql-prod",
  "name": "Prod MySQL",
  "type": "mysql",
  "dsnEnv": "MYSQL_PROD_DSN",
  "intervalSeconds": 30,
  "tags": { "server": "db-prod", "application": "shared-db" }
}
```

Save. Within `intervalSeconds` you will see samples in the **MySQL** page.

## The 9 Default Rules

| Rule | Threshold (default) | What it catches |
| ---- | ------------------- | --------------- |
| Connection utilization high | > 80 % for 2 min | Approaching `max_connections` |
| Slow query rate high | > 5/s for 3 min | Index regression or workload spike |
| Long-running queries | > 60 s alive | Stuck query holding locks |
| Row lock waits spiking | > 10/s for 2 min | Contention |
| Lock wait time high | > 200 ms avg for 5 min | Writers stalling |
| Deadlock detected | any deadlock | Tx ordering bug — investigate query plans |
| Replica lag high | > 30 s for 1 min | Reads serving stale data |
| Query throughput drop | < 50 % of 1 h baseline | Upstream broke before MySQL did |
| Restart detected | Uptime decreased | Unexpected restart — check OOM, crash log |

Every rule is editable in **Alert Rules**. Disable the ones that do not match your workload (e.g. drop the replica-lag rule if you have no replicas).

## Reading the MySQL Page

Three tabs:

- **Health card** — current connection use, query rate, active sessions, lag — at a glance.
- **Time series** — sparkline per signal over the last hour/day/week. Look for cliffs and sustained climbs.
- **Process list** — what is running *right now*. Sort by `time` desc to find stuck queries.

When an incident is open, the linked snapshot includes the process list, slow query digest, and variable values **at the moment the threshold was crossed** — gold for post-mortem.

## What Good vs Bad Looks Like

| Signal | Healthy | Worth investigating | Bad |
| ------ | ------- | ------------------- | --- |
| Connection use | < 50 % | 50 – 80 % | > 80 % sustained |
| Slow queries | < 0.5/s | 0.5 – 5/s | > 5/s sustained |
| Lock waits | < 1/s | 1 – 10/s | > 10/s |
| Replica lag | < 1 s | 1 – 30 s | > 30 s sustained |
| Active sessions | < 2× CPU cores | 2 – 4× cores | > 4× cores |

## Common Pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| Connection % high but app fine | Pool sized > actual concurrency | Match pool to `Threads_running` p95, not pool max |
| Slow query rule fires during backups | Backup uses long-running selects | Put backup window in maintenance |
| Replica lag flaps around the threshold | Threshold too tight; replicas catch up in bursts | Raise to 60 s and only count sustained lag |
| `unknown` state | DSN env var missing on HealthOps host | Restart with env var set |
| Auth error in collector logs | Wrong user/password, or `host` not allowed | Confirm GRANT includes `'%'` or the HealthOps IP |
| No process list visible | Missing `PROCESS` privilege | Re-grant — see setup section |
| Snapshot is empty | Incident opened on the first sample (no previous to delta) | Wait one more interval; ignore one-off |

## Where to Go Next

- **MySQL — Connections** — drill into connection state breakdowns.
- **MySQL — Queries** — slow query digest, top offenders.
- **MySQL — Threads** — per-thread CPU and wait analysis.
- **MySQL — Server** — version, variables, configuration drift.
- **Alert Rules** — tune the 9 defaults for your workload.
