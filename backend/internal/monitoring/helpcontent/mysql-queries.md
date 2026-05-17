---
slug: mysql-queries
title: MySQL Queries
summary: Slow query counters, query type breakdown, and indicators of unhealthy query patterns.
intent: Use this when latency is up and you want to confirm (or rule out) that database queries are the cause.
category: Detect
order: 212
icon: file-bar-chart
relatedPaths: /mysql/queries
relatedTopics: mysql,mysql-connections,scenarios-databases
---

# MySQL Queries

The Queries view aggregates query-level counters from `SHOW GLOBAL STATUS`, computed as deltas per sampling interval (default 60s).

## Counters Explained

- **Queries** — total statements executed since last sample. Use as the baseline.
- **Slow Queries** — statements that exceeded `long_query_time` (default 10s, often tuned to 1s or 2s).
- **Questions** — same as Queries minus commands run by stored procedures internally. Closer to "what the app sent".
- **Selects / Inserts / Updates / Deletes** — broken out from `Com_select`, `Com_insert`, etc.
- **Full Joins** — `Select_full_join`: joins that did NOT use an index. Should be near zero.
- **Range Scans** — `Select_range_check`: range scans on tables without good indexes.
- **Sort Merge Passes** — `Sort_merge_passes`: external sorts that spilled to disk. Tune `sort_buffer_size` or add indexes.

## Read vs Write Ratio

The view displays the ratio of read (SELECT) to write (INSERT/UPDATE/DELETE) traffic. A sudden inversion (writes spiking, reads flat) often points to a runaway job or a stuck consumer retrying.

## Time-Series Charts

Each counter has a chart over the configured retention window (default 14 days). Look for:

- **Sustained climb in Slow Queries** → query plan regression, missing index, data volume growth.
- **Cliff drop in Queries** → app is no longer talking to this DB (deployment? broken pool?).
- **Spikes in Full Joins** → someone shipped a query without proper joins/indexes.

## What "Slow" Means

A "slow query" is server-side: `Slow_queries` counter increments when a statement runs longer than `long_query_time`. It does NOT include the network round-trip time the app sees. App-side timeouts will show as `Aborted Clients` on the Connections page.

## Sampling Cost

Reading `SHOW GLOBAL STATUS` is cheap. Reading the slow query log is not — HealthOps does not parse it (we just watch counters). For per-query analysis, use `performance_schema.events_statements_summary_by_digest` directly in a DB client.

## Tuning Suggestions

- **Slow queries > 0 sustained** → enable the slow query log (`slow_query_log=ON`) and review with `pt-query-digest` to find the worst statements.
- **Full Joins > 0** → grep your app code for the queries that are missing indexes. Run `EXPLAIN` on them.
- **Sort Merge Passes > 0 sustained** → raise `sort_buffer_size`, or add an index that satisfies the ORDER BY.

## Related Recipes

- **Scenarios → Databases → Monitor MySQL slow queries** sets up the rate alert.
- For deeper RCA on individual statements, use the AI Assistant: "show me the slowest digest in checkout DB for the last hour".
