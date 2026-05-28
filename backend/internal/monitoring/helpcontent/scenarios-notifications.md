---
slug: scenarios-notifications
title: Scenarios — Notifications
summary: Step-by-step recipes for sending HealthOps incidents to Slack, email, webhooks, and on-call providers.
intent: Use these when an incident opens and you want someone (or something) to know about it.
category: Scenarios
order: 670
icon: send
relatedPaths:
relatedTopics: notifications,integrations
---

# Scenarios — Notifications

## Recipe 1 — Send incidents to Slack

**Goal:** Every critical incident posts to your `#alerts` channel.

**Steps:**

1. In Slack: **Apps → Incoming Webhooks → Add to `#alerts`** and copy the webhook URL.
2. In HealthOps: **Settings → Notification Channels → Add → Slack-compatible**.
3. Paste the webhook URL. Name the channel `slack-alerts`.
4. Click **Test** — a message should appear in Slack.
5. **Apply the channel** in one of two ways:
   - **Per-check:** edit each critical check and add `slack-alerts` to its channels.
   - **Per-alert-rule:** when creating alert rules, add `slack-alerts` to `channelIds`.

**Verification:** open a test incident (run a known-bad check, or use `POST /api/v1/runs` with a deliberately broken target). The Slack message arrives within seconds of the incident opening.

---

## Recipe 2 — Send incidents to email

**Goal:** Send every incident to `oncall@example.com`.

**Steps:**

1. Put the SMTP password in the HealthOps process environment:
   ```
   HEALTHOPS_SMTP_PASS=<vault-managed-password>
   ```
   Restart HealthOps.
2. **Settings → Notification Channels → Add → Email**.
3. Enter SMTP host, port, username, sender, recipient, and set the password env field to `HEALTHOPS_SMTP_PASS`.
4. **Test** — confirm an email arrives. If it does not, check the HealthOps logs for SMTP errors.
5. Attach the channel to the checks you care about.

**Spam-deliverability checklist:**

- SPF record allows the SMTP server to send for your domain.
- DKIM signing enabled on the SMTP server.
- DMARC policy aligns. `p=quarantine` is a safe starting point.
- Reverse DNS for the sending IP matches the domain.

---

## Recipe 3 — Send incidents to a custom webhook

**Goal:** Forward incidents to your in-house alerting system at `https://oncall.internal/hooks/healthops`.

**Steps:**

1. **Settings → Notification Channels → Add → Webhook**.
2. URL: `https://oncall.internal/hooks/healthops`.
3. Optional secret header (recommended): `X-HealthOps-Token: <random-256-bit-value>`. Your receiver verifies this header to reject spoofed events.
4. **Test** — your endpoint receives a POST. Sample body:

   ```json
   {
     "event": "incident.opened",
     "incident": {
       "id": "inc_abc123",
       "checkId": "api-checkout",
       "severity": "critical",
       "openedAt": "2026-05-17T08:30:00Z",
       "evidence": { "lastResult": { "status": "down", "message": "connection refused" } }
     },
     "timestamp": "2026-05-17T08:30:01Z"
   }
   ```

5. Respond `2xx` quickly (<5s). Do real processing in your background queue.

**Why a shared-secret header:** the webhook URL alone is not authentication. Anyone who learns the URL could POST fake events. The header makes spoofing harder.

---

## Recipe 4 — Send incidents to PagerDuty / Opsgenie

**Goal:** Trigger an on-call escalation for critical incidents only.

**Steps (PagerDuty Events API v2):**

1. In PagerDuty, create a service with an "Events API v2" integration and copy the **Integration Key**.
2. In HealthOps: **Settings → Notification Channels → Add → Webhook**.
3. URL: `https://events.pagerduty.com/v2/enqueue`.
4. HealthOps' default payload does not match PagerDuty's schema, so configure the channel template (if available in your build) to map fields. Pseudocode:

   ```json
   {
     "routing_key": "<integration-key>",
     "event_action": "trigger",
     "dedup_key": "{{incident.id}}",
     "payload": {
       "summary": "{{incident.checkId}} — {{incident.severity}}",
       "source": "healthops",
       "severity": "{{incident.severity}}",
       "custom_details": "{{incident.evidence}}"
     }
   }
   ```

5. **Apply the channel only to alert rules / checks marked `critical`** — you do not want `info` incidents paging humans at 3 AM.

**For Opsgenie:** use the Opsgenie webhook API and adapt the payload similarly.

**Important:** put PagerDuty behind a tested filter. The cost of waking someone for nothing is high.

---

## Common Tuning Notes For Notifications

- **Test every channel before relying on it.** Half of all outage post-mortems include "we did not know because the alert went nowhere".
- **Severities matter.** Map `info → chat`, `warning → chat + ticket`, `critical → page`. Do not page on warnings.
- **Rate-limit one direction.** If a thousand checks fail at once, you do not want a thousand pages. Use alert rule cooldowns and channel-side throttling.
- **Maintenance windows silence notifications.** Use them.
- **Audit who changes channels.** Anyone who turns off paging temporarily must turn it back on. Audit log catches this.
