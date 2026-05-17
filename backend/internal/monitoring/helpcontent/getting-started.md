---
slug: getting-started
title: Getting Started
summary: A 10-minute walkthrough for your first day with HealthOps.
intent: Read this first if you have never opened HealthOps before. It explains what the product does, what to set up, and what to look at.
category: Start Here
order: 10
icon: rocket
relatedPaths: /
relatedTopics: quick-tour,glossary,faq
---

# Getting Started

HealthOps watches your servers, APIs, processes, databases, log streams, and scheduled jobs, then tells you when something looks wrong. This page is the fastest path from "I have no idea what this is" to "I can use it confidently".

## What HealthOps Does in One Paragraph

You configure **checks** (small tests like "is this URL returning 200?" or "is this MySQL connection healthy?"). A scheduler runs them at a fixed interval. Results are stored. When a check keeps failing, an **incident** opens. Incidents trigger **notifications** through your configured channels (email, webhook, Slack-compatible). Optional **AI** can review incident evidence and suggest a root cause. Long-term **analytics** show you which monitors are noisy and which services are unreliable.

## Your First 10 Minutes

1. **Log in** with the bootstrap admin credentials (set via the `HEALTHOPS_BOOTSTRAP_ADMIN_USERNAME` and `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` environment variables on first start). If you already have an account, just sign in.
2. **Open the Dashboard.** This is the operating overview — current status counts, open incidents, recent latency and availability.
3. **Open Checks.** You will see seeded demo checks. Each row is a monitor with type, target, interval, and current state.
4. **Open one check** to see history, recent results, and the configuration that produced them.
5. **Open Incidents.** If nothing is broken, this is empty. If something failed enough times to cross a threshold, it lives here with evidence.
6. **Open Settings.** This is where you configure AI providers (optional), users, notification channels, and global behavior.

## Your First Real Check

Click **Add Check** on the Checks page. The simplest useful one is an **API check**:

- Type: `api`
- URL: a public health endpoint you control (for example `https://yourapp.example.com/healthz`)
- Expected status: `200`
- Interval: `60` seconds
- Failures to open incident: `3` (so a single blip does not page you)

Save. Within a minute the check will run and you will see a result. Force a failure (turn off the service or change the URL) and watch an incident open after three consecutive failures.

## Your First Notification

Open **Settings → Notification Channels** (or the equivalent on your build). Add a webhook or email destination. Send a test message. Then, on a check, attach that channel so incidents on this monitor reach you.

## Things You Do Not Need on Day One

- AI providers
- Status pages
- Heartbeat checks
- MySQL monitoring (unless you have a database to watch)
- Log ingestion

Add these only when you have a concrete need. HealthOps is useful without any of them.

## Where To Go Next

- **Quick Tour** — a 30-second tour of every screen.
- **Glossary** — every term defined in plain English.
- **FAQ** — questions everyone asks during week one.
- **Troubleshooting** — fix the most common setup problems.
