---
slug: notifications
title: Notifications
summary: Delivery channels and notification history for incidents and monitor events.
intent: Use Notifications to decide who gets alerted, through which channel, and to verify whether a delivery succeeded.
category: Operate
order: 170
icon: bell
relatedPaths: /notifications
relatedTopics: incidents,alert-rules,integrations
---

# Notifications

Notifications are how HealthOps tells humans something happened. Without a channel attached, an incident still opens — but nobody finds out automatically.

## Channels

| Channel | Use for |
| ------- | ------- |
| Email | Low-urgency, batched, audit trail |
| Webhook | Custom integrations, pagers, ticketing |
| Slack-compatible | Team chat (Slack, Mattermost, others that accept the same payload) |
| SMS (if configured) | Critical pages |
| PagerDuty / Opsgenie (if integration installed) | On-call escalation |

## How a Channel Works

1. You add a channel in settings with the destination URL/address.
2. You **test** it. Do this immediately — half of all delivery issues are wrong URLs or wrong tokens.
3. You attach the channel to checks or alert rules (`channelIds: ["pager-prod"]`).
4. When an incident opens, HealthOps posts to the outbox, then attempts delivery in the background.
5. Outbox shows each attempt and the result.

## Reading the Outbox

The Notifications page shows recent delivery attempts:

- **Pending** — queued, not yet sent.
- **Delivered** — accepted by the destination (2xx response, or message ID returned).
- **Failed** — destination rejected it (4xx, 5xx, timeout). The error is recorded.

If somebody says "I did not get the alert", check here first. Often the alert was sent and the issue is on the destination side.

## Retries

Failed deliveries retry with backoff. After a configurable maximum, they go to a dead-letter state and stop trying. Investigate channel configuration when this happens.

## Throttling and Deduplication

- **Per-incident** HealthOps does not spam the same incident — one open notification, optionally one acknowledged notification, one resolved notification.
- **Per-rule** alert rule cooldown prevents repeat firing.
- **Per-channel** you can configure rate limits if a downstream cannot handle bursts.

## Per-User Subscriptions

A future feature. Today, channels are global; attach them where you want them.

## Common Pitfalls

- **"I tested it, it worked, but real alerts do not arrive."** The destination may rate-limit unknown sources. Check the destination's quota.
- **"Slack channel returns 200 but I see nothing."** You sent to the wrong webhook (an archived channel, a deleted integration). Recreate the webhook.
- **"Email landed in spam."** Add SPF/DKIM/DMARC records for HealthOps' sending domain.
