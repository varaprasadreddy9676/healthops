---
slug: api-quickstart
title: API Quickstart
summary: Call HealthOps from your own scripts and services in five minutes.
intent: Use this when you want to automate something against the HealthOps API or send data into it.
category: Admin
order: 520
icon: code
relatedPaths:
relatedTopics: heartbeats,log-events,integrations
---

# API Quickstart

HealthOps is API-first. Everything the UI does, the API does too. This page is the fastest path to a working integration.

## Authentication

HealthOps uses JWT bearer tokens. To get one:

```bash
curl -X POST https://healthops.example.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"you@example.com","password":"<your password>"}'
```

Response:

```json
{
  "success": true,
  "data": { "token": "eyJhbGc...", "user": { "id": "...", "role": "admin" } }
}
```

Use the `token` as `Authorization: Bearer <token>` on every other call. Default TTL is 24 hours.

## Response Envelope

All endpoints return:

```json
{ "success": true, "data": { ... } }
```

Errors:

```json
{ "success": false, "error": { "code": 400, "message": "validation failed", "details": [...] } }
```

## Common Calls

### List checks

```bash
curl -H "Authorization: Bearer $T" https://healthops.example.com/api/v1/checks
```

### Create a check

```bash
curl -X POST https://healthops.example.com/api/v1/checks \
  -H "Authorization: Bearer $T" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "api-checkout",
    "name": "Checkout API",
    "type": "api",
    "target": "https://shop.example.com/health",
    "intervalSeconds": 60,
    "timeoutSeconds": 5,
    "expectedStatus": 200,
    "failuresToOpen": 3,
    "successesToResolve": 2
  }'
```

### Trigger a check immediately

```bash
curl -X POST https://healthops.example.com/api/v1/runs \
  -H "Authorization: Bearer $T" \
  -H "Content-Type: application/json" \
  -d '{"checkId":"api-checkout"}'
```

### List open incidents

```bash
curl -H "Authorization: Bearer $T" \
  "https://healthops.example.com/api/v1/incidents?state=open"
```

### Acknowledge an incident

```bash
curl -X POST https://healthops.example.com/api/v1/incidents/<id>/acknowledge \
  -H "Authorization: Bearer $T"
```

### Send log events

```bash
curl -X POST https://healthops.example.com/api/v1/logs/ingest \
  -H "Authorization: Bearer $T" \
  -H "Content-Type: application/json" \
  -d '{"entries":[{
    "timestamp":"2026-05-17T08:30:00Z",
    "level":"error",
    "source":"auth-service",
    "server":"prod-web-01",
    "message":"JWT signature verification failed",
    "tags":["security","auth"]
  }]}'
```

### Ping a heartbeat

```bash
curl -X POST https://healthops.example.com/api/v1/heartbeats/<token>
```

This endpoint is intentionally unauthenticated by token only — anyone with the token can ping. Treat the token as a secret.

### Server-Sent Events stream

```bash
curl -N -H "Authorization: Bearer $T" \
  https://healthops.example.com/api/v1/events
```

Streams incident open/resolve and check state-change events in near real time.

## Rate Limits

- General API: 3000 requests/minute/IP.
- Login: 10 attempts/minute/IP.

Hit one, you get `429 Too Many Requests`. Back off.

## Full Reference

The OpenAPI spec is `docs/openapi.yaml` in the repo. The full text reference is `backend/docs/api-reference.md`.

## Tips

- Use the SSE stream for low-latency UI integrations.
- Batch log ingestion. 50 entries per call is fine; one entry per call is wasteful.
- Cache the JWT for its TTL. Do not call `/login` on every request.
- For service-to-service automation, create a dedicated admin user just for that integration.
