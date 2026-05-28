---
slug: scenarios-logs
title: Scenarios — Log Monitoring
summary: Step-by-step recipes for ingesting logs, detecting repeated errors, and wiring log shippers.
intent: Use these when you want HealthOps to detect log-based incidents across many services.
category: Scenarios
order: 660
icon: file-search
relatedPaths:
relatedTopics: log-events,api-quickstart
---

# Scenarios — Log Monitoring

## Recipe 1 — Detect repeated application errors

**Goal:** Open an incident when "JWT verification failed" appears more than 100 times in 5 minutes.

**Steps:**

1. **Send the events.** From your auth service, every time a JWT fails, call:

   ```python
   import requests, datetime
   requests.post(
       "https://healthops.example.com/api/v1/logs/ingest",
       headers={"Authorization": "Bearer <token>"},
       json={"entries":[{
           "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
           "level": "error",
           "source": "auth-service",
           "server": "prod-auth-01",
           "message": "JWT signature verification failed",
           "tags": ["security","auth"],
           "meta": {"reason": "signature_invalid"}
       }]},
       timeout=2
   )
   ```

   Wrap in `try/except` — log ingestion failures should not break your auth service.

2. Open **Log Events**. After a few events, you will see a pattern row "JWT signature verification failed".
3. Click the pattern → **Create Alert Rule**.
4. Fill:
   - Condition: `count > 100` in `300` seconds
   - Severity: `warning`
   - Channel: your pager channel
5. Save.

**Generate 100 events in 5 minutes** (a brute-force tester, or a synthetic test) to confirm the rule fires.

---

## Recipe 2 — Alert when a specific log pattern crosses a threshold

**Goal:** Open an incident whenever an `ERROR OutOfMemory` line appears at all (any non-zero count is bad).

**Steps:**

1. Confirm your app sends each error line via `POST /api/v1/logs/ingest`.
2. **Log Events** → find or wait for the `OutOfMemory` pattern.
3. **Create Alert Rule** on the pattern with condition `count > 0` in `60` seconds, severity `critical`, channel pager.
4. Save.

**Why a critical-on-1 rule:** some errors are zero-tolerance. Do not water them down with windows and cooldowns; if it happens, page immediately.

---

## Recipe 3 — Ingest from Vector

**Goal:** Forward production logs from Vector to HealthOps without changing your apps.

**Steps:**

1. Add an HTTP sink to your `vector.toml`:

   ```toml
   [sinks.healthops]
   type = "http"
   inputs = ["app-logs"]
   uri = "https://healthops.example.com/api/v1/logs/ingest"
   method = "post"
   encoding.codec = "json"
   framing.method = "newline_delimited"
   request.headers.Authorization = "Bearer <token>"
   batch.max_events = 50
   batch.timeout_secs = 5

   # HealthOps expects { "entries": [...] }, so wrap.
   [transforms.healthops_wrap]
   type = "remap"
   inputs = ["app-logs"]
   source = '''
     . = { "entries": [{
       "timestamp": to_string!(now()),
       "level": .level,
       "source": .service,
       "server": .host,
       "message": .message,
       "tags": .tags,
       "meta": .meta
     }] }
   '''
   ```

2. Restart Vector.
3. In HealthOps, **Log Events** populates within a minute.

**Same pattern works for Fluent Bit (`http` output plugin) and Logstash (`http` output)** — wrap each batch in `{ "entries": [...] }`.

---

## Recipe 4 — Ingest from a Dockerized sidecar

**Goal:** Tail `/var/log/app/*.log` and forward without modifying the app.

**Steps:**

1. Add a sidecar container next to your app that runs:

   ```bash
   tail -F /var/log/app/*.log | while IFS= read -r line; do
     curl -fsS -m 5 -X POST https://healthops.example.com/api/v1/logs/ingest \
       -H "Authorization: Bearer <token>" \
       -H "Content-Type: application/json" \
       -d "$(jq -nc --arg ts "$(date -u +%FT%TZ)" --arg msg "$line" \
            '{entries:[{timestamp:$ts,level:"info",source:"app",server:"'$HOSTNAME'",message:$msg}]}')"
   done
   ```

2. This is one POST per line — fine for low-volume; replace with a batching shipper (Vector) for high-volume.

---

## Common Tuning Notes For Log Monitoring

- **Batch.** One POST per log line is wasteful. 25-100 events per batch is normal.
- **Strip IDs from `message`.** Keep request IDs and user IDs in `meta` so pattern clustering works.
- **Sample noisy categories.** Send 1 in 100 INFO logs, all WARN, all ERROR. Otherwise log ingestion dominates your disk.
- **Retention.** Log ingestion can dwarf every other category on disk. Tune log-event retention early.
