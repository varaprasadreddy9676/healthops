# ADR 002: Primary Persistence Model

Status: Superseded by ADR 005 for runtime persistence
Related: ADR 005 finalizes MongoDB as the required runtime persistence layer and supersedes any production use of best-effort MongoDB mirroring or file-store fallback.

## Context
The initial prototype relied on a blob-style local state model with an optional full-state mirror to MongoDB. This approach required rewriting large mutable snapshots and syncing full state, which was not scalable or safe for a production environment.

## Decision
We will use **MongoDB as the primary persistence model** in production. We will persist discrete domain models such as check definitions, check runs, latest status, users, incidents, AI config, AI queues, notifications, and audit records.

ADR 005 supersedes this ADR's earlier allowance for local file persistence. The current runtime requires MongoDB and fails fast when `MONGODB_URI` is not configured or MongoDB is unavailable.

## Consequences
**Positive:**
- Provides durable, scalable storage capable of handling a large volume of historical check runs.
- Avoids the performance penalty and race conditions of rewriting a giant JSON blob on every state change.
- Simplifies concurrent access and updates by multiple backend instances if ever horizontally scaled.

**Negative:**
- Adds a hard dependency on MongoDB for production availability. If MongoDB goes down, the system cannot function normally (unlike the previous best-effort mirroring approach).

**Alternatives Considered:**
- **Local File Store Primary:** We considered continuing with the local JSON file. This was rejected for production due to scalability and reliability limits.
- **Relational DB (PostgreSQL):** We considered using a relational database. MongoDB was chosen as it aligns well with our pre-existing mirror integration and allows flexible document schemas for diverse check parameters.
