---
slug: status-pages
title: Status Pages
summary: Public or internal pages that communicate customer-facing service status.
intent: Use Status Pages to publish clear availability and incident updates without exposing private operational details.
category: Understand
order: 300
icon: globe
relatedPaths: /status-pages
relatedTopics: incidents,checks
---

# Status Pages

A status page is a public-facing summary of your services for users, customers, or stakeholders. It converts internal check state into safe public components and human-readable updates.

## What Is Public, What Stays Private

| Public | Internal-only |
| ------ | ------------- |
| Component names ("API", "Checkout") | Check IDs and types |
| Component status (operational, degraded, down) | Raw error messages, stack traces |
| Uptime percentage | Server hostnames, IPs |
| Incident summaries you wrote | Credentials, DSNs, file paths |
| Scheduled maintenance | Audit log entries |

The status page renderer enforces this — internal data cannot accidentally leak.

## Components

A component is a public-facing piece of your product mapped to one or more internal checks. Examples: "Web App" (mapped to your homepage + auth checks), "Search" (mapped to the search API + Elasticsearch checks).

## Incident Updates Are Human-Written

The status page does not auto-publish raw error messages. When you want to tell users about an incident, you write a short update:

> **Investigating** — We are seeing elevated checkout errors and our team is investigating.

Updates can be scheduled and edited as the situation evolves.

## Multiple Pages

You can have several status pages — one public for customers, one internal for staff that exposes more detail. Each has its own URL slug.

## Common Pitfalls

- **"Status page shows operational but the service is down."** The component mapping does not include the failing check. Edit the component.
- **"It shows down for a glitch."** Component status uses the same threshold logic as checks. Tune `failuresToOpen` for monitors backing public components.
- **"I do not want it public yet."** Mark the page as internal-only. Authentication is enforced at the API layer.
