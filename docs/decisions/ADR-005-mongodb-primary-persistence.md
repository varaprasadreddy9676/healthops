# ADR 005: MongoDB Primary Persistence for AI-Native Operations

Status: Accepted — Migration Complete
Date: 2026-05-16
Supersedes: ADR 002 production best-effort mirror and runtime file fallback allowances for AI-native operations data.

## Context

ADR 002 established MongoDB as the production persistence model and restricted local file storage to development and testing. The current codebase uses MongoDB as the sole runtime persistence layer. Legacy file-based and JSONL repositories have been migrated and removed.

The AI-native roadmap depends on reliable evidence retrieval across incidents, logs, checks, MySQL samples, alert deliveries, AI investigations, and audit events. A best-effort mirror can lose ordering, hide write failures, and make AI incident explanations inconsistent. For these workflows, silent degradation is worse than an explicit degraded health state.

## Decision

MongoDB is the primary and only runtime persistence layer for AI-native operations data.

- New AI-native collections are written directly to MongoDB. There is no file-store equivalent and no best-effort mirror for these collections.
- Legacy file, JSONL, and in-memory stores have been migrated. Production runtime writes use MongoDB exclusively.
- If MongoDB is unavailable, AI-native features must refuse to start or report degraded/unready through health checks. They must not silently write to files.
- The service may keep file-backed stores only for local development tests, explicit migration tooling, and backup/export utilities.
- AI provider pricing is part of persisted AI configuration. The price table lives in MongoDB-backed AI config as `modelPricing`, keyed by provider and model, with input/output token cost, currency, and effective date.

## Phase 0 Migration Acceptance Criteria

- JSONL to MongoDB migrations verify source and target counts within 0.1% for durable records.
- The in-memory incident cutover verifies that incidents created during the cutover window are not lost by replaying the audit log and comparing expected incident IDs/transitions against MongoDB.
- Startup health checks clearly distinguish MongoDB-ready, MongoDB-degraded, and migration-required states.
- Indexes required by the AI-native roadmap exist before enabling new write paths.
- File-backed production write paths have been removed from the runtime.

## Consequences

Positive:

- Operational evidence has one source of truth.
- AI incident briefs and post-incident reviews can rely on consistent retrieval semantics.
- MongoDB failures become visible operational failures instead of hidden data divergence.
- Migration and retention policies can be tested collection by collection.

Negative:

- Production availability now depends on MongoDB availability.
- Local development needs either MongoDB or explicit test-only file adapters.
- Cutover requires careful audit-log replay for data that was previously in memory.

## Alternatives Considered

- Keep best-effort MongoDB mirroring with file fallback. Rejected because it can silently lose or reorder operational evidence.
- Store AI-native data in a relational database. Rejected for v1 because MongoDB is already the chosen production store and fits the event/evidence document model.
- Add a dedicated log database immediately. Rejected for v1 because HealthOps will store only redacted short-window events and aggregates, while users can keep Loki, Elastic, CloudWatch, or another archive for full-scale log search.
