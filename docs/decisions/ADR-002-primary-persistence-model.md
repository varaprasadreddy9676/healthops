# ADR 002: Primary Persistence Model

## Context
The existing prototype of Medics Health Check relies on a blob-style state model, writing to a local JSON file (`data/state.json`) with an optional full-state mirror to MongoDB. This approach requires rewriting large mutable snapshots and syncing full state, which is not scalable or safe for a production environment.

## Decision
We will use **MongoDB as the primary persistence model** in production. We will persist discrete domain models (e.g., CheckDefinition, CheckRun, CheckLatestStatus). The local fallback storage (file store) will be restricted to local development and testing purposes only. 

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
