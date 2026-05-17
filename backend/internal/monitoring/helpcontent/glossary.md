---
slug: glossary
title: Glossary
summary: Every HealthOps term defined in one sentence.
intent: Open this when a word in the UI does not match how you would describe it.
category: Start Here
order: 30
icon: book
relatedPaths:
relatedTopics: getting-started,faq
---

# Glossary

## Check
A small repeatable test of one thing — for example "is this URL 200?", "is this port open?", "is this process running?", "is this log file fresh?". Checks have a type, target, interval, timeout, and threshold settings.

## Check Type
The kind of test a check runs. Common types: `api`, `tcp`, `ping`, `dns`, `ssl`, `domain`, `process`, `command`, `ssh`, `mysql`, `log`, `heartbeat`.

## Result
The output of one execution of a check — status (up, down, warn), latency, message, and any captured metrics.

## Incident
A failing check that crossed its open threshold. An incident has a start time, a current state, the evidence that opened it, and a timeline of updates until it is resolved.

## Threshold
The rule that turns repeated failures into an incident, and repeated successes back into a healthy state. Usually expressed as "open after N failures, resolve after M successes".

## Alert Rule
A configurable rule that fires when a metric crosses a threshold, for example "MySQL connection utilization above 90% for 5 minutes".

## Notification
A delivery of an incident or alert to a configured channel.

## Channel
A destination for notifications — email, webhook, Slack-compatible endpoint, or any supported integration.

## Heartbeat
A check where an external job pings HealthOps on a schedule. If the ping is missed by more than the configured grace period, an incident opens.

## Server
A host that HealthOps monitors. Servers have live signals (CPU, memory, disk, processes) and are referenced by checks.

## Log Event
A single log message your application or agent sent to HealthOps via `POST /api/v1/logs/ingest`.

## Log Pattern
A cluster of repeated log events that look like the same problem.

## Maintenance Window
A planned period during which checks are suppressed so expected downtime does not create incidents.

## Status Page
A public or internal page that communicates service status to your users or stakeholders.

## RCA / Root Cause Analysis
An AI-assisted explanation of why an incident happened, built from incident evidence.

## BYOK
"Bring Your Own Key" — you provide the API key for an AI provider; HealthOps does not ship one.

## Evidence
The data attached to an incident — check results, MySQL snapshots, server metrics, log events, audit entries — that an operator (or AI) uses to diagnose.

## Snapshot
A point-in-time capture of evidence at the moment an incident opened.

## Retention
How long HealthOps keeps each kind of data. Different categories (results, snapshots, notifications) have separate retention windows.

## Bootstrap
The first-run process that creates the initial admin user and seeds default configuration.

## Audit Log
A tamper-evident record of who did what — logins, configuration changes, role updates, channel modifications.
