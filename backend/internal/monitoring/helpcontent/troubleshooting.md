---
slug: troubleshooting
title: Troubleshooting
summary: Fixes for the most common setup and runtime problems.
intent: Open this before opening a support issue. Most problems here have a one-line fix.
category: Start Here
order: 50
icon: wrench
relatedPaths:
relatedTopics: faq,getting-started
---

# Troubleshooting

## I cannot log in

- Check that the bootstrap admin envs were set on first start: `HEALTHOPS_BOOTSTRAP_ADMIN_USERNAME`, `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD`.
- If the user already exists, the bootstrap envs are ignored. Reset the password via another admin, or drop the users collection in MongoDB and restart (you will lose all users).
- If you see "authentication required" on every page, your JWT may have expired. Sign in again.

## The frontend loads but the API returns 401

- Confirm a user is logged in. Open the browser dev tools network tab and look for the `Authorization: Bearer ...` header on API calls.
- If the header is present but rejected, the JWT secret may have rotated. Sign out and back in.

## A check stays "pending" forever

- The scheduler runs every `checkIntervalSeconds`. Wait one cycle.
- If still pending after two cycles, the workers may be saturated. Reduce the number of checks or increase `workers` in config.
- For SSH-based checks, the target server entry may be missing or the SSH key path may be wrong. Open the **Servers** page and verify.

## API check fails but the URL works in my browser

- Browsers send cookies, the backend does not. If your endpoint requires auth, give the check the right header.
- Outbound from the HealthOps host may be different from your laptop. Test from the host: `curl -v <url>`.
- Self-signed certs fail unless you allow them on the check config.

## MySQL check fails with "unknown DSN"

- The check config references `dsnEnv: MYSQL_PROD_DSN` (for example). The env var with that name must be set on the HealthOps process — not on your shell.
- Restart HealthOps after changing the env.

## Process check never finds the process

- Process matching uses `ps` and looks for a keyword in the command line. The keyword must be unique enough to match only your process. Test on the host: `ps -ef | grep <keyword>`.
- On containers, the process may run under PID 1 with a different command line than expected.

## Log file check says "stale" but the file is being written

- File mtime, not size, is checked. Some loggers buffer and only flush on rotation. Configure your logger to flush more often, or watch a different file.

## Notifications never arrive

- Open **Notifications**. The outbox shows attempted deliveries with status.
- Test the channel from its settings page first.
- For webhooks, check that the receiving endpoint accepts HealthOps' payload format.
- For Slack-compatible webhooks, the URL must accept POST with `text` and `attachments`.

## "AI is not configured" everywhere

- Open **Settings → AI Configuration**.
- Add a provider with a valid API key. Run the provider health check.
- Without at least one healthy provider, all AI surfaces stay hidden by design.

## I added a check in `default.json` but it does not appear

- `default.json` is read **only on first run** to seed MongoDB. After that it is ignored. Add the check via the **Checks** page or `POST /api/v1/checks`. See API Quickstart.

## Disk is filling up

- The biggest contributors are MongoDB collections for check results, MySQL samples, and server metrics. Reduce retention in **Settings**, or set per-category retention envs. See **Data Retention**.
- Logs to stdout are not stored by HealthOps — your container runtime stores those.

## High CPU on the HealthOps process

- Too many checks at too short an interval. Increase intervals, raise `workers`, or split into multiple deployments.
- A pathological log ingestion pattern can also burn CPU. Reduce client batch frequency or batch size.

## "Degraded mode" banner appears

- The backend lost its primary persistence (usually Mongo) and refuses writes to protect data integrity. Reads still work. Restore the dependency, the banner clears automatically.

## Where do I get more detail?

- Server logs to stdout. Filter for `level=error` and the relevant request ID.
- Audit log (`/api/v1/audit`) shows who changed what.
- Health endpoint (`/healthz`) reports backend dependency status.
