---
slug: maintenance-windows
title: Maintenance Windows
summary: Silence checks during planned work so expected downtime does not create false incidents.
intent: Schedule a window before deploys, migrations, network changes, or anything else that will intentionally break a monitor.
category: Operate
order: 160
icon: calendar-clock
relatedPaths:
relatedTopics: checks,incidents,notifications
---

# Maintenance Windows

A maintenance window tells HealthOps "this is planned — do not alert on it". During the window, affected checks still run, but failures **do not** open incidents or send notifications.

## When To Use One

- Application deploys
- Database migrations
- Network or DNS changes
- Hardware replacement
- Any time a check will fail for a known good reason

## Anatomy

| Field | Meaning |
| ----- | ------- |
| `name` | Human description ("Friday production deploy") |
| `startsAt` / `endsAt` | When it covers |
| `checkIds` / `tags` | What it silences (specific checks or anything matching a tag) |
| `suppressNotifications` | If true, no notifications fire even for unrelated incidents |
| `recurrence` | Optional — daily, weekly, custom cron |

## Suppress vs Skip

- **Suppress notifications** (default) — checks run, results are recorded, incidents do **not** open. You still see the failure in history.
- **Skip checks entirely** — the scheduler does not run them. Use this for migrations that physically take the dependency offline.

## Recurring Windows

Useful for routine maintenance — "every Sunday 02:00-04:00 UTC", "weeknight backups 23:00-23:30". HealthOps will silence affected checks on every occurrence.

## Common Pitfalls

- **"My window ended but alerts are still suppressed."** Check the timezone of the window — HealthOps stores in UTC.
- **"I added a window but I still got paged."** The window may not target the right check IDs/tags. Verify the scope.
- **"Window covers deploys but tests never recover."** That is a real failure, not maintenance. Resolve it manually after the window.
