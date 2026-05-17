---
slug: scenarios-web-apis
title: Scenarios — Web and APIs
summary: Step-by-step recipes for monitoring websites, APIs, certificates, and domains.
intent: Pick the recipe that matches what you want to monitor and paste the configuration.
category: Scenarios
order: 610
icon: globe-2
relatedPaths:
relatedTopics: checks,api-quickstart
---

# Scenarios — Web and APIs

## Recipe 1 — Monitor that a public website is up

**Goal:** Open an incident if `https://shop.example.com` is unreachable or returns 5xx.

**Steps:**

1. Open **Checks → Add Check**.
2. Fill:
   - Type: `api`
   - Name: `Shop homepage`
   - Target URL: `https://shop.example.com`
   - Method: `GET`
   - Expected status: `200`
   - Interval: `60` seconds
   - Timeout: `10` seconds
   - Failures to open: `3`
   - Successes to resolve: `2`
3. Save.
4. **Test it:** click **Run now**. Confirm result is "up" with a latency value.

**Equivalent API call:**

```bash
curl -X POST $H/api/v1/checks -H "Authorization: Bearer $T" -H "Content-Type: application/json" -d '{
  "id":"shop-homepage","name":"Shop homepage","type":"api",
  "target":"https://shop.example.com","intervalSeconds":60,"timeoutSeconds":10,
  "expectedStatus":200,"failuresToOpen":3,"successesToResolve":2
}'
```

**Verify it works:** Stop the target service (or change the URL to a bad one). After 3 minutes (3 × 60s) you should see an incident open.

---

## Recipe 2 — Monitor a JSON API returns 200 with expected body

**Goal:** Confirm `/api/health` returns `200` AND the body contains `"status":"ok"`.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `api`
   - Target URL: `https://api.example.com/health`
   - Expected status: `200`
   - Expected body substring: `"status":"ok"`
   - Interval: `30` seconds
   - Warning latency: `500` ms (warns above this, but does not open an incident)
   - Failures to open: `3`
3. Optional: add a custom request header for auth — `Header name: Authorization`, `Value: Bearer <token>`.
4. Save and **Run now**.

**Why expected body matters:** plenty of dead apps still return `200` from a misconfigured proxy. A body check catches that.

---

## Recipe 3 — Monitor HTTPS certificate expiry

**Goal:** Warn 30 days before a cert expires, open an incident 7 days before.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `ssl`
   - Target: `shop.example.com:443`
   - Interval: `3600` (one hour is plenty)
   - Warn threshold: `30` days
   - Critical threshold: `7` days
3. Save.

**What you see:** the check reports `daysUntilExpiry`. Below 30 → warning state. Below 7 → failing and incident opens after `failuresToOpen` (default 3 hours of failure).

**Pair with:** an `acme` renewal automation on your side. HealthOps tells you the renewal failed; the actual renewal is your responsibility.

---

## Recipe 4 — Monitor a domain registration does not expire

**Goal:** Catch a forgotten domain renewal before the domain goes dark.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `domain`
   - Target: `example.com`
   - Interval: `86400` (once a day is enough)
   - Warn threshold: `60` days
   - Critical threshold: `14` days
3. Save.

**Why this matters:** domain registrar emails go to a former employee's address more often than anyone expects.

---

## Common Tuning Notes For Web Checks

- **Latency warning vs failure** — set `warningLatencyMs` so slow-but-up shows as warning, not as a green check.
- **Auth headers** — store tokens in HealthOps' check config; rotate when they expire.
- **From-where matters** — HealthOps tests from wherever it is deployed. If you need synthetic monitoring from multiple regions, deploy multiple HealthOps instances.
- **Self-signed certs** — disable cert validation only when you control both ends.
