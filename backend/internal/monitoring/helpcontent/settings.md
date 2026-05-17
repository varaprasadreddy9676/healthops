---
slug: settings
title: Settings
summary: Workspace-level configuration — AI providers, security, runtime defaults, retention.
intent: Use Settings when changing integrations or behavior that affects the whole HealthOps installation.
category: Admin
order: 510
icon: settings
relatedPaths: /settings
relatedTopics: ai-overview,security,data-retention
---

# Settings

Settings is where you change global HealthOps behavior. Most operators do not need to come here often; admins use it during setup and integration changes.

## The Seven Tabs

The Settings page is split into tabs along the left edge:

1. **General** — runtime configuration snapshot (read-only view of envs) plus the few values you can change live: retention days, check interval, worker count, "allow command checks".
2. **Users** — same data as the standalone Users page; here for one-stop admin access. Create, deactivate, reset password, change roles.
3. **Servers** — register and edit servers (the targets your checks run against). Same as the standalone Servers page.
4. **Health Checks** — every check across the workspace. Quick edit, enable/disable, change interval. Faster than the per-server views for bulk edits.
5. **Alert Rules** — global and per-check alert thresholds. Configure consecutive breaches, cooldowns, channel routing.
6. **AI Providers** — provider API keys, default model, prompt templates, fallback order. See **AI Configuration** below.
7. **Export** — bulk export of checks, results, incidents in CSV/JSON. Useful for backups, audits, or migrating between environments.

## What Lives Where

- AI provider keys, prompt templates → **AI Providers** tab.
- JWT TTL, login rate limits, audit retention → environment variables (not editable in UI).
- Data retention (days to keep results, incidents) → **General** tab.
- Notification channels → standalone **Notifications** page or the relevant tab.
- Webhook integrations → **Integrations** (legacy section); newer integrations use the Notifications page.
- Workspace name/branding → not yet exposed in the UI; edit `data/state.json` directly if you must.

## AI Configuration

Provider keys are stored AES-256-GCM-encrypted at rest. API responses always mask them. To rotate keys, replace the value and save — re-encryption is automatic. For the encryption-key rotation procedure (different from API key rotation), see `backend/docs/ai-key-rotation.md`.

## Security

- **JWT TTL** — lower for higher security, higher for fewer re-logins. Default 24h.
- **Bootstrap envs** — only read on the very first run.
- **Audit retention** — keep at least 90 days for compliance-relevant deployments.

## Operational Rule

Changing settings can affect alerting, status pages, and automation. Verify with a small test incident or demo check before relying on new production behavior. Audit log records every change.

## Things You Cannot Change Here (and Why)

- **Check intervals** — those are per-check, edited on the Checks page.
- **Database connection** — set via environment variables on startup; runtime mutation is not safe.
- **Frontend theme** — controlled by the user, not workspace.
