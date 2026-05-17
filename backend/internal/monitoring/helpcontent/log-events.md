---
slug: log-events
title: Log Events
summary: Application and server log messages sent into HealthOps, grouped into recurring patterns.
intent: Use Log Events when you want HealthOps to detect repeated errors across services instead of reading raw log files by hand.
category: Detect
order: 200
icon: file-text
relatedPaths: /logs
relatedTopics: incidents,api-quickstart
---

# Log Events

Log Events are log messages your applications or agents send to HealthOps. HealthOps clusters them into patterns so operators can see "this error happened 240 times in the last hour" instead of scrolling thousands of lines.

## Who Calls the Ingestion API

HealthOps does not magically read log files on every machine. **Something must send the events**:

- Your application calls `POST /api/v1/logs/ingest` directly.
- A sidecar/daemon tails files, batches events, and posts every few seconds.
- A cron script posts operational errors from batch jobs.
- A log forwarder (Vector, Fluent Bit, custom) transforms external logs into HealthOps events.

In the demo stack, `docker/demo-log-emitter` sends a realistic mix of log families every 15 seconds so the page has data.

## Raw Events vs Patterns

| | Raw event | Pattern |
| --- | --------- | ------- |
| What | One single message | Cluster of repeated messages |
| Example | `JWT signature failed for req-123` | "JWT signature failures" (240 events in 1h) |
| Where shown | Pattern detail page → recent samples | The main Log Events list |

The list is patterns because operators care about recurring issues, not individual lines. Each pattern's detail page shows representative samples and recent raw entries.

## How To Send Logs

```bash
curl -X POST https://healthops.example.com/api/v1/logs/ingest \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "entries": [
      {
        "timestamp": "2026-05-17T08:30:00Z",
        "level": "error",
        "source": "auth-service",
        "server": "prod-web-01",
        "message": "JWT signature verification failed for request_id=req-123",
        "tags": ["security", "auth"],
        "meta": { "reason": "signature_invalid" }
      }
    ]
  }'
```

Batch multiple entries per request when possible — much cheaper.

## How Categories Are Assigned

HealthOps first applies deterministic rules:

- timeout messages → **Timeout**
- disk-full messages → **Disk I/O**
- failed login messages → **Security**
- HTTP access lines → **Access Log**
- and more

Anything the rule classifier cannot identify confidently is marked **Unclassified**. If AI is configured, the **AI Categorize** action on the patterns page can review unclassified patterns and assign a category.

## How This Differs From Log File Checks

| | Log file check | Log Events ingestion |
| --- | -------------- | -------------------- |
| Direction | HealthOps **reads** the file | Your app/agent **pushes** events |
| Granularity | One file per check | Stream of arbitrary events |
| Detection | Freshness or substring | Pattern clustering and frequency |
| Use when | You have a specific file to watch | You have many sources and want grouping |

## Common Pitfalls

- **"Nothing shows up."** Nothing is calling the ingestion API. Add an emitter or wire your app's logger.
- **"Everything is Unclassified."** Your messages do not match any deterministic rule. Add a `tags` field with a category, or enable AI Categorize.
- **"Patterns explode in cardinality."** Your `message` contains a unique ID (request ID, timestamp). Tokenize those out at the source — keep IDs in `meta`, not in `message`.
