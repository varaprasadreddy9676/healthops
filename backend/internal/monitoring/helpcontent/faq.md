---
slug: faq
title: FAQ
summary: The questions everyone asks during their first week.
intent: Skim this before you assume something is broken.
category: Start Here
order: 40
icon: help-circle
relatedPaths:
relatedTopics: getting-started,troubleshooting
---

# FAQ

## Why do I see "no data" on the dashboard right after I started HealthOps?

The scheduler runs every check on its own interval. Most defaults are 60 seconds. Wait for the first cycle. Refresh.

## Do I need MongoDB?

No. HealthOps works fully with the local file store. Mongo is an optional mirror for resilience and for shared deployments. If `MONGODB_URI` is not set, the file store is the single source of truth.

## Do I need AI?

No. AI is optional. Without a configured AI provider, the AI surfaces (Root Cause, Ask AI, AI Results) are hidden. The rest of the product works without AI.

## I changed `config/default.json` but my change is ignored. Why?

`default.json` is only read on the **very first run**, to seed `data/state.json`. After that, the file store and API are the source of truth. Manage checks via the UI or `POST /api/v1/checks`. To re-seed from scratch, stop the service, delete `data/state.json`, then start again.

## How do I add a check via the API?

`POST /api/v1/checks` with a JSON body. See **API Quickstart** for an example.

## A check is failing but no incident opened. Why?

Incidents only open when the **failures-to-open** threshold is crossed. A single bad result is usually filtered out. Check the monitor's threshold settings.

## A check is succeeding but the incident did not close. Why?

Incidents only resolve when the **successes-to-resolve** threshold is crossed. One success after a failure burst will not auto-resolve. Wait for the configured number of clean runs.

## Where are my secrets stored?

API keys for AI providers are AES-256-GCM encrypted at rest in `data/ai_config.json`. The encryption key lives in `data/.ai_enc_key`. Database DSNs are read from environment variables you reference by name in the check config — they are never stored or logged.

## How do I forward HealthOps logs to my own log system?

HealthOps writes structured logs to stdout. Capture them with the same mechanism you use for any other container or process (Docker logs, journald, Fluent Bit, Vector, Loki, CloudWatch).

## Can I run multiple HealthOps instances behind a load balancer?

Not yet for the scheduler — the scheduler is a single process. The HTTP API can scale horizontally if you point all instances at the same Mongo. For now, run one node and back it with monitoring of the node itself.

## How do I back up?

Back up your MongoDB database — it holds all state, checks, incidents, AI config, and operational data. The `data/` directory contains only the AI encryption key and JWT secret.

## How do I rotate the AI encryption key?

There is a dedicated CLI: `go run ./cmd/rotate-ai-keys`. See `backend/docs/ai-key-rotation.md` for the full procedure.

## My UI shows old data. Why?

The frontend caches via React Query. Hard-refresh the browser. If the data is still stale, the backend is the source — check `/api/v1/summary` directly.

## Where is the API reference?

Open `docs/openapi.yaml` in the repo, or the **API Quickstart** topic on this page for the common cases.
