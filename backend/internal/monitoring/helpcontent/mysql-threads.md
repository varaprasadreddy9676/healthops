---
slug: mysql-threads
title: MySQL Threads
summary: Thread cache hits, thread creation rate, and the cost of churn.
intent: Use this when MySQL is consuming unexpected CPU on connection handling, or when query latency is spiked by thread creation.
category: Detect
order: 213
icon: layers
relatedPaths: /mysql/threads
relatedTopics: mysql,mysql-connections
---

# MySQL Threads

Threads in MySQL serve connections. The server keeps a small cache (`thread_cache_size`) so it does not have to create a new OS thread every time a client connects. Thread churn is usually invisible — until it isn't.

## Key Metrics

- **Threads_created** — total threads MySQL has had to create since startup. Should be roughly flat once the cache is warm.
- **Threads_cached** — threads currently sitting in the cache, ready for the next connection.
- **Threads_connected** — current active connections (same as on Connections page).
- **Threads_running** — connections currently doing work (not sleeping). Useful spike indicator.

## Thread Cache Hit Rate

The view shows the cache hit rate: `1 - (Threads_created / Connections)`. Above 99% is healthy. Below 90% is wasteful.

When the hit rate is poor:

- Connections are arriving faster than they're being cached.
- Likely cause: app uses short-lived connections (PHP-FPM, serverless, scripts that connect-query-disconnect).
- Fix at the app: use a connection pool.
- Fix at MySQL: raise `thread_cache_size`.

## Threads Running Over Time

A chart of `Threads_running` is the most useful signal here. Quick spikes are normal (a burst of queries). Sustained high values mean:

- Queries are slow and piling up.
- A lock is being held.
- A single query is using a lot of parallel work (InnoDB read threads).

Pair the spike with the Queries page to confirm whether query count is also up.

## Slow Launch Threads

`Slow_launch_threads` counts threads that took longer than `slow_launch_time` (default 2s) to start. Should be zero. If you see non-zero, the OS is under memory or CPU pressure.

## When to Worry

- Hit rate < 90% → app connection pool problem.
- `Threads_running` sustained > 50 on a single instance → contention or saturation.
- `Slow_launch_threads` > 0 → host-level pressure; check OS metrics.

## What This Page Is Not

It is not a replacement for the OS view. If MySQL is starving for memory or CPU, you'll see it earlier in the host's metrics page than here.

## Related

- **MySQL → Connections** for the per-client view.
- **MySQL → Server** for engine-level (InnoDB, buffer pool) metrics.
