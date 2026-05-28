# ADR 003: Authentication Strategy

## Status
Superseded by the current Mongo-backed JWT/RBAC implementation.

## Context
HealthOps now persists users in MongoDB, bootstraps a fixed first `admin` account from `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD`, and requires JWT bearer tokens for operational APIs.

## Decision
The active product decision is JWT bearer authentication with role-based access:

- `POST /api/v1/auth/login` exchanges username/password for a 24-hour JWT.
- Non-public API routes require `Authorization: Bearer <token>`.
- Mutating APIs require the `admin` role.
- `ops` users can use read-only operational views.
- Bootstrap envs create/reset only the fixed `admin` user; they are not a general credential store.

## Consequences
**Positive:**
- User-level audit attribution.
- Admin/ops separation for day-to-day operations.
- No shared static write token in source or deployment config.

**Negative:**
- JWT TTL is currently fixed in code.
- External identity provider integration is not implemented yet.

**Alternatives Considered:**
- **Shared deployment token:** rejected for the current product because it does not provide user-level audit attribution or RBAC.
- **Full OIDC/OAuth2 Integration:** still a reasonable future option for larger organizations, but it is not required for the open-source self-hosted baseline.
