---
slug: mysql-connections
title: MySQL Connections
summary: Live connection list, aborted clients, refused connections, and per-thread analysis.
intent: Use this when you suspect MySQL is running out of connections, refusing new ones, or carrying long-running clients.
category: Detect
order: 211
icon: plug-zap
relatedPaths: /mysql/connections
relatedTopics: mysql,alert-rules,scenarios-databases
---

# MySQL Connections

The Connections view shows MySQL's live connection state for the selected database.

## At-a-Glance Cards

- **Current** — `Threads_connected`. How many clients are connected right now.
- **Aborted Connects** — clients that failed to authenticate or hit a network error during handshake. Spike = bad credentials, network flakiness, or misconfigured client pool.
- **Aborted Clients** — clients that connected successfully but the connection died mid-flight. Spike = idle timeouts, packet size limits, network drops.
- **Connections Refused** — clients turned away because `max_connections` was full. Even one is a problem at peak.

## Thread Stats

- **Total Processes** — rows in `SHOW PROCESSLIST`.
- **Max Used** — peak `Threads_connected` since server start. Compare to `max_connections`.
- **Active Queries** — non-idle processes (state != "Sleep").
- **Long Running** — processes whose query has been executing longer than the long-running threshold (default 5 seconds).

## All Connections Table

The full process list, one row per connected client:

| Column | Meaning |
| --- | --- |
| ID | Process / connection ID |
| User | DB user the client authenticated as |
| Host | Source IP/hostname |
| DB | Current database |
| Command | Query, Sleep, etc. |
| Time | Seconds the current command has been running |
| State | Per-engine state (Sending data, Locked, etc.) |
| Info | The actual SQL (truncated) |

Sort and filter to find: idle clients, long sleepers, hot users, hot tables.

## When to Worry

- **Current > 80% of `max_connections`** → tune `max_connections` or fix the app's connection pool.
- **Aborted Connects > 0 and rising** → likely a stale credential or a client misconfigured the wrong host.
- **Connections Refused > 0** → at-capacity event. Open the All Connections table, find idle clients, kill them or shorten `wait_timeout`.
- **Many "Sleep" with Time > wait_timeout** → connection pool is too large.

## Killing a Bad Connection

The UI does not kill connections (read-only by design). Use a DB client:

```sql
KILL <process-id>;
```

Or terminate the offending app process — usually a stuck cron, runaway script, or a misbehaving worker.

## Related Recipes

- **Scenarios → Databases → Monitor MySQL connection saturation** sets up the alert rule that catches this.
- **MySQL → Alert Rules → Connection utilization** is the rule that drives incidents from this page.
