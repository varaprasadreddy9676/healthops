---
slug: security
title: Security Model
summary: Authentication, authorization, secret storage, audit trail, and rate limits.
intent: Read this before deploying HealthOps anywhere that handles real production data.
category: Admin
order: 540
icon: shield
relatedPaths:
relatedTopics: users,settings
---

# Security Model

This page documents the security guarantees and the things you must do to keep them true.

## Authentication

- Users authenticate with username + password and receive a JWT.
- JWTs are signed with a per-deployment secret (`data/.jwt_secret`), auto-generated on first run. Rotate by deleting the file and restarting (all sessions invalidated).
- Bootstrap admin is created from envs **only on first run**.
- Login is rate-limited at 10 attempts/minute/IP.
- Password hashing: bcrypt with a per-user salt.

## Authorization

- Role-based: Admin / Operator / Read-only. See Users.
- All mutating endpoints require Admin unless the operation is on the user's own resource.
- Authorization is enforced in middleware, not at the handler — there is no path that accidentally skips it.

## Secret Storage

| Secret | Where it lives | How it is protected |
| ------ | -------------- | ------------------- |
| User passwords | MongoDB users collection | bcrypt-hashed, never returned by API |
| AI provider API keys | MongoDB AI config collection | AES-256-GCM encrypted, masked in API |
| AI encryption key | `data/.ai_enc_key` | Filesystem permissions only — protect with disk encryption |
| JWT signing secret | `data/.jwt_secret` | Filesystem permissions only |
| MySQL DSNs | Environment variables | Never logged, never returned by API |
| SSH keys | Filesystem path referenced by server records | HealthOps reads on demand; path is logged, key is not |
| Webhook secrets | Notification channel records | Stored but masked in API responses |

## Audit Trail

Every sensitive action is recorded:

- Login (success + failure)
- User create / delete / role change / password reset
- Check create / update / delete
- Alert rule changes
- Notification channel changes
- AI configuration changes
- Incident acknowledge / resolve
- Manual remediation execution

Audit entries are append-only, timestamped, and include the acting user. Retain at least 90 days for compliance-relevant deployments.

## Network Surface

- Default port: `8080` (HTTP). Put a TLS-terminating reverse proxy in front for production (nginx, Caddy, cloud load balancer).
- Rate limits: 3000/min/IP general, 10/min/IP login.
- Security headers: `X-Content-Type-Options`, `X-Frame-Options`, baseline CSP applied by middleware.
- Public endpoints (no auth):
  - `/healthz`, `/readyz`, `/api/v1/system/status`
  - `/api/v1/auth/login`
  - `/api/v1/heartbeats/<token>` (token-only access)
  - `/api/v1/help/*` (documentation)
  - `/status/*` (configured status pages, if any)

Everything else requires a valid JWT.

## What HealthOps Is Not

- **Not** a SOC2/HIPAA appliance out of the box. It supports compliant deployments but you own the configuration (retention, audit export, TLS, backup).
- **Not** a secret manager. Do not store production credentials in HealthOps beyond what the integrations explicitly need.
- **Not** a remote-shell. SSH-based checks need explicit allowlisting per server; there is no shell exposed.

## Hardening Checklist

- [ ] TLS-terminating proxy in front of port 8080
- [ ] Bootstrap admin env vars removed after first run
- [ ] AI encryption key on encrypted disk
- [ ] User passwords meet your length/complexity policy (currently no built-in enforcement — coming)
- [ ] Audit log shipped to external storage with longer retention
- [ ] MongoDB on encrypted disk and authenticated private network
- [ ] Back up MongoDB plus `data/.jwt_secret` and `data/.ai_enc_key`
- [ ] Restrict outbound network so only required webhook destinations are reachable
- [ ] Use a dedicated service account user (admin role) for automation
- [ ] Monitor HealthOps itself with a second monitoring system or external uptime check
