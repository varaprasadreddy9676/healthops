---
slug: integrations
title: Integrations
summary: Connect HealthOps to your chat, paging, ticketing, and webhook destinations.
intent: Use this to wire incident notifications and external events into the tools your team already uses.
category: Admin
order: 530
icon: plug
relatedPaths:
relatedTopics: notifications,api-quickstart
---

# Integrations

HealthOps integrates outward (notifications going to other tools) and inward (other tools sending data in).

## Outbound — Notifications

### Slack (or any Slack-compatible incoming webhook)

1. In Slack: **Apps → Incoming Webhooks → Add to a channel** and copy the webhook URL.
2. In HealthOps: **Settings → Notification Channels → Add → Slack-compatible**.
3. Paste the webhook URL.
4. Click **Test**. A message should appear in Slack within seconds.
5. Attach the channel to checks or alert rules whose incidents you want in this Slack channel.

Mattermost, Rocket.Chat, and other Slack-API-compatible tools use the same channel type.

### Email

1. **Settings → Notification Channels → Add → Email**.
2. Enter the destination address (or distribution list).
3. SMTP credentials must be configured at the workspace level (envs: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`). Without SMTP, email channels fail delivery.
4. Test. If it lands in spam, add SPF/DKIM/DMARC records for the sending domain.

### Webhook (custom)

1. **Settings → Notification Channels → Add → Webhook**.
2. Paste the URL of your receiving endpoint.
3. Optional: add a shared secret header for verification.
4. Your endpoint receives a POST with:

```json
{
  "event": "incident.opened",
  "incident": { "id": "...", "checkId": "...", "severity": "...", "openedAt": "...", "evidence": {...} },
  "timestamp": "..."
}
```

5. Respond `2xx` quickly (<5s). Long processing should happen in your background queue, not in the webhook handler.

### PagerDuty / Opsgenie / OnCall systems

Use a webhook with their incoming-event format, or use their Slack-compatible inbound webhook if they support one. Map HealthOps severity to your provider's urgency.

## Inbound — Pushing Data Into HealthOps

### Log shippers

Any log shipper that can POST JSON can push to `/api/v1/logs/ingest`:

- **Vector** — use the `http` sink with a `lua` or `remap` transform to shape events.
- **Fluent Bit** — `http` output plugin.
- **Logstash** — `http` output.
- **Custom agent** — see Log Events for the payload shape.

### Cron job heartbeats

Append a `curl` to the end of every cron entry. See Heartbeats for the exact pattern.

### Cloud function / Lambda completion signals

Call `/api/v1/heartbeats/<token>` at the end of every successful invocation. If the function fails to invoke entirely (rare but real), the missed heartbeat fires.

### Application metrics

HealthOps does not ingest arbitrary metrics today (no `/api/v1/metrics` endpoint). Use checks or log events as the surface for now.

## Verifying an Integration

1. Send a test event from the integration source.
2. Open Notifications → Outbox. Find the attempt.
3. Status: Delivered. Done.
4. Status: Failed. Read the error. Common: wrong URL, expired token, destination rate limit.
