---
slug: users
title: Users
summary: User accounts, roles, and access management for the HealthOps workspace.
intent: Use this page to grant only the access each operator needs.
category: Admin
order: 500
icon: users
relatedPaths: /users
relatedTopics: security,settings
---

# Users

Users are the human accounts that log into HealthOps. Each user has a role that determines what they can see and change.

## Roles

| Role | Read | Write | Admin actions |
| ---- | ---- | ----- | -------------- |
| **Admin** | Everything | Everything | Add users, edit roles, change settings, install integrations |
| **Operator** | Everything | Acknowledge/resolve incidents, run checks, write status updates | No |
| **Read-only** | Everything | No | No |

Roles are global today. Per-resource ACLs are not implemented.

## Adding a User

Admins create users from the Users page or via `POST /api/v1/users`. A first-time password is set by the admin and the user changes it on first login.

## First Admin (Bootstrap)

On the very first run, HealthOps reads `HEALTHOPS_BOOTSTRAP_ADMIN_USERNAME` and `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` from the environment and creates an admin. After that, those env vars are ignored — manage users from the UI.

## Password Reset

Admins can reset any user's password. Users can change their own. There is no email-based password reset yet — that requires a configured email channel.

## Sessions

Sessions are JWT-based. Default expiry is 24 hours. Sign out invalidates the session locally; the JWT itself is still valid until expiry (this is a known constraint of stateless JWT). For high-security environments, lower the JWT TTL.

## Audit

Every user-management action (create, delete, role change, password reset, login, failed login) is recorded in the audit log. Admins can review it on the audit page.

## Disabling vs Deleting

- **Disable** keeps the user record (and their history of actions) but rejects logins.
- **Delete** removes the record. Audit history of past actions remains.

## Common Pitfalls

- **"I cannot demote myself."** That is intentional — at least one admin must exist. Create another admin first.
- **"User logged in successfully but every API call returns 401."** Their account role may not permit the page they are on, or the JWT secret rotated and they need to sign back in.
- **"Forgot the admin password."** Stop the service, delete `data/users.json`, restart with bootstrap envs. You lose all users, so back up first.
