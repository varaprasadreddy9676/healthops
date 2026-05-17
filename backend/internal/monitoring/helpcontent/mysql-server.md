---
slug: mysql-server
title: MySQL Server
summary: Server-level information — version, uptime, InnoDB buffer pool, key engine variables.
intent: Use this for "give me the basic facts about this MySQL" — version, uptime, engine config, buffer pool state.
category: Detect
order: 214
icon: database-zap
relatedPaths: /mysql/server
relatedTopics: mysql,mysql-connections,mysql-queries
---

# MySQL Server

The Server view shows server-level facts that do not change minute-to-minute. Useful when starting an investigation or auditing a fleet.

## Server Identity

- **Version** — the MySQL/MariaDB version string. Old majors (5.6, 5.7) need an upgrade plan.
- **Uptime** — seconds since the server started. A surprisingly low value usually means a crash-restart.
- **Hostname** — what the server reports as its own name.
- **Server ID** — replication identifier; should be unique per node in a topology.

## InnoDB Buffer Pool

InnoDB caches data in memory; this is the single most important config knob.

- **Buffer Pool Size** — `innodb_buffer_pool_size`. Rule of thumb: 60-80% of available RAM on a dedicated DB host.
- **Buffer Pool Pages Total / Free / Dirty** — total capacity, unused pages, modified-not-yet-flushed pages.
- **Buffer Pool Hit Rate** — `1 - (reads / read_requests)`. Above 99% healthy; below 95% means the working set doesn't fit.

When hit rate is low: either grow the buffer pool (more RAM), shrink the working set (drop indexes, archive old data), or accept the disk read cost.

## Engine Variables

A read-only snapshot of the variables HealthOps cares about most:

- `max_connections` — hard cap on concurrent clients.
- `wait_timeout` / `interactive_timeout` — idle session limits.
- `long_query_time` — slow query threshold.
- `slow_query_log` — on/off.
- `sync_binlog` — durability vs throughput trade-off.
- `innodb_flush_log_at_trx_commit` — same trade-off, InnoDB redo log.

For the full variable list, query `SHOW GLOBAL VARIABLES` from a DB client.

## Replication

If this server is part of a replication topology, you'll see:

- **Role** — source or replica.
- **Seconds Behind Source** — replica lag. Watch closely.
- **Last IO/SQL Error** — non-empty means replication is broken or paused.

The full state lives in `SHOW REPLICA STATUS`.

## What You Cannot Do Here

- Change a variable (HealthOps is read-only against DBs).
- Restart the server.
- Issue queries (use a DB client).

This page is for *understanding* the database, not operating it.

## Related

- **MySQL → Connections / Queries / Threads** for live operational data.
- **Scenarios → Databases → Monitor MySQL replication lag** turns the lag value here into alerts.
