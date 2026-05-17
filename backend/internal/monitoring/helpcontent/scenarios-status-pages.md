---
slug: scenarios-status-pages
title: Scenarios — Status Pages
summary: Step-by-step recipes for publishing a status page and communicating an incident publicly.
intent: Use these when external users (customers, partners) need to see your service status.
category: Scenarios
order: 680
icon: globe
relatedPaths:
relatedTopics: status-pages,incidents
---

# Scenarios — Status Pages

## Recipe 1 — Publish a public status page

**Goal:** A `status.example.com` page that shows overall health of "Web App", "API", and "Background Jobs".

**Steps:**

1. **Status Pages → Add Status Page**.
2. Fill:
   - Name: `Public Status`
   - Slug: `public` (the URL becomes `https://healthops.example.com/status/public`)
   - Visibility: `public`
3. Add components:
   - Component name: `Web App`
   - Backed by checks: `web-homepage`, `web-login`
   - Display order: 1
4. Repeat for `API` (backing checks: `api-checkout`, `api-search`) and `Background Jobs` (backing heartbeats).
5. Save.
6. Visit `https://healthops.example.com/status/public` in an incognito window — confirm it loads without auth.

**To put it on your own domain:**

- Add `status.example.com` as a CNAME to your HealthOps host (or to a reverse proxy that forwards to `/status/public`).
- Configure TLS at the proxy.
- Optionally set the slug to be the default (route `/status/example.com/` to `/status/public`).

**What is shown vs hidden:**

- ✅ Component name, status (operational / degraded / down), uptime percentage, public incident updates you write.
- ❌ Check IDs, server hostnames, raw error messages, stack traces, audit log, credentials.

---

## Recipe 2 — Communicate an active incident to customers

**Goal:** Tell users that checkout is broken, then update them as it recovers.

**Steps:**

1. Internal incident opens automatically when checks fail.
2. **Status Pages → Public Status → Add Update**.
3. Fill:
   - Components affected: `API`
   - Status: `degraded` or `down`
   - Title: `Checkout errors`
   - Body (user-facing, short, honest):
     > We are seeing elevated errors on checkout requests since 09:12 UTC. Our team is investigating and we will post the next update by 10:00 UTC.
4. Save. The update is now public.
5. Update again as you learn more:
   - **Identified** — "We have identified the cause as a database failover. Service is recovering."
   - **Resolved** — "All systems back to normal. Total impact: 47 minutes. Post-mortem to follow."
6. Mark the components back to `operational` and resolve the public incident.

**Status page incidents vs internal incidents:**

- An **internal incident** is created automatically and contains all evidence.
- A **public incident update** is something you write deliberately. They are linked but not the same thing.
- You can have an internal incident with **no** public update — most flaky monitor failures should never reach customers.

**Update cadence:**

- New incident: post within 15 minutes of internal detection.
- Active incident: update every 30 minutes even if there is no news ("still investigating, no ETA yet").
- Resolved: post promptly. Quiet recovery looks like you do not care.

---

## Common Tuning Notes For Status Pages

- **Component grouping matches customer mental models, not your internal architecture.** "Login" is a component. "auth-service-canary-east" is not.
- **Pre-write your templates.** Have draft "investigating / identified / monitoring / resolved" messages ready so you are not writing prose during the fire.
- **Subscribe to your own status page** to verify notifications actually fire.
- **Internal status pages** are useful too — a private page for staff that shows more components than the public one.
