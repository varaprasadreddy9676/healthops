---
slug: login
title: Login and Sessions
summary: Sign in, session lifetime, password reset, and what to do when you lose access.
intent: Read this when you cannot log in, your session keeps expiring, or you need to bootstrap the very first admin.
category: Start Here
order: 25
icon: log-in
relatedPaths: /login
relatedTopics: users,security
---

# Login and Sessions

## First-Ever Login

HealthOps does not ship with a default username and password. The very first admin user is created from environment variables set on the first start:

```bash
HEALTHOPS_BOOTSTRAP_ADMIN_USERNAME=admin
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD=<a strong password>
HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL=admin@example.com
```

After the user store has at least one admin, these envs are ignored. You can (and should) remove them from the running configuration after the first sign-in.

## Day-to-Day Login

1. Open `https://healthops.example.com/login`.
2. Enter username and password. Press Enter or click **Sign in**.
3. You land on the Dashboard.

On success, HealthOps issues a JWT token (stored in browser local storage as `healthops_token`) and remembers the user record (`healthops_user`). The token's lifetime is 24 hours by default.

## When You Get Logged Out

- **Session expired** — the most common case. Just sign in again.
- **Token revoked** — an admin reset your password, or all sessions were invalidated by rotating the JWT signing secret (delete `data/.jwt_secret` and restart).
- **401 from any API call** — the frontend redirects to `/login` automatically. If this happens in a loop, clear your browser local storage for the site and try again.

## Forgotten Password

You cannot reset your own password if you are locked out (HealthOps does not send "forgot password" emails today). To recover:

1. Have another admin reset your password from **Users → your account → Reset password**.
2. If no other admin exists, restart HealthOps with the bootstrap envs again and create a new admin. The existing one is preserved.

## Service Accounts

For automation that calls the API, create a dedicated admin user named like `automation-ci`. Use that user's JWT in your CI/CD pipelines. Rotate by resetting the password.

## Rate Limits

- 10 login attempts per minute per IP. Six failures in a row pauses the form briefly.
- 3000 general API calls per minute per IP after login.

## Security Notes

- HealthOps is HTTP by default. Put a TLS-terminating proxy in front of it.
- JWTs are signed with a per-deployment secret. Treat the `data/` directory like secrets.
- Do not commit bootstrap env values to git. Inject them at deploy time.
