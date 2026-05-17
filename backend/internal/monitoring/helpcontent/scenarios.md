---
slug: scenarios
title: All Scenarios
summary: Index of every "how do I monitor X" recipe with copy-paste configuration.
intent: Start here when you know what you want to monitor and need the exact steps.
category: Scenarios
order: 600
icon: list
relatedPaths:
relatedTopics: getting-started,checks
---

# All Scenarios

Browse by category. Every recipe gives you the exact configuration to paste, plus what to expect when it works.

## Websites and APIs

- Monitor a public website is up
- Monitor a JSON API returns 200 with expected body
- Monitor HTTPS certificate expiry
- Monitor a domain registration does not expire

See: **Scenarios — Web and APIs**.

## Network

- Monitor a TCP port is open (database, Redis, custom port)
- Monitor DNS resolves to the right record
- Monitor a host with ping
- Monitor that SSH login works

See: **Scenarios — Network**.

## Linux Servers

- Monitor a systemd service / process is running
- Monitor disk space on a server
- Monitor CPU and memory on a server
- Monitor a custom shell command returns success
- Monitor a log file is fresh (and detect when an app stops writing)

See: **Scenarios — Servers**.

## Databases

- Monitor MySQL connection saturation
- Monitor MySQL slow queries
- Monitor MySQL replication lag
- Monitor MySQL deadlocks and lock waits
- Monitor a custom SQL query result

See: **Scenarios — Databases**.

## Scheduled Jobs and Cron

- Monitor a cron job ran on time
- Monitor a nightly backup completed
- Monitor a Kubernetes CronJob
- Monitor a serverless function invocation

See: **Scenarios — Scheduled Jobs**.

## Application Logs

- Detect repeated errors via log ingestion
- Alert when a specific log pattern crosses a threshold
- Ingest logs from Vector / Fluent Bit / Logstash

See: **Scenarios — Log Monitoring**.

## Notifications and Alerting

- Send incidents to Slack
- Send incidents to email
- Send incidents via webhook to your own system
- Send incidents to PagerDuty / Opsgenie

See: **Scenarios — Notifications**.

## Public Status

- Publish a public status page
- Communicate an active incident to customers

See: **Scenarios — Status Pages**.

## Operations and AI

- Schedule a maintenance window for a deploy
- Create an alert rule for graded metrics
- Auto root-cause critical incidents
- Auto-categorize unclassified log patterns

See: **Scenarios — Operations**.

---

If your scenario is not listed, the closest recipe is almost always one HTTP, TCP, command, or heartbeat check away. Read **Checks** for the full type matrix.
