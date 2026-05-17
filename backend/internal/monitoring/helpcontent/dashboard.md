---
slug: dashboard
title: Dashboard
summary: The operating overview for health, incidents, uptime, latency, and active risk.
intent: Use this page first when you want to know whether the monitored estate is healthy right now.
category: Operate
order: 100
icon: layout-dashboard
relatedPaths: /,/dashboard
relatedTopics: incidents,checks,analytics
---

# Dashboard

The Dashboard is your first stop. It answers one question: **is the monitored estate healthy right now?**

## What It Shows

- **Status counts** — how many checks are healthy, warning, failing, or unknown.
- **Open incidents** — anything currently broken that needs attention.
- **Recent activity** — checks that just ran, results that just changed.
- **Availability and latency trends** — last few hours so you can spot a live incident vs old noise.
- **Server and service health** — summary cards for grouped resources.

## How To Read It

1. Start at the top — critical counts and open incidents. If both are zero and you trust the monitors, you are done.
2. If a count is non-zero, click into it. Do not interpret a number on its own.
3. Use trend charts to distinguish a live outage from a recovered one.
4. Use **Refresh** when you just added a check or just resolved an incident.

## Where the Data Comes From

The scheduler runs checks on their configured intervals. Every result is stored. The dashboard queries the summary API, which reads stored state. **No data on the dashboard is invented.** If something looks wrong, open the underlying check or incident.

## Common Pitfalls

- **"Everything is green but production is down"** — you may not be monitoring the failing component. Add a check for the customer-visible URL.
- **"Numbers don't add up"** — a check may be disabled or in a maintenance window. Open the Checks page to see state per monitor.
- **"It says healthy but the chart shows a gap"** — the gap is the scheduler being stopped or the host being unreachable. Check uptime of the HealthOps process itself.

## Permissions

All authenticated users can view the dashboard. Admins additionally see configuration links.
