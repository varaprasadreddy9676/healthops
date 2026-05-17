# Pluggable Monitoring Architecture Plan

Date: 2026-05-17
Status: Draft (revision 2 — sharpened after internal review)

## TL;DR

The MySQL vertical (`MySQLSample`, `MySQLRuleEngine`, `MySQLEvidenceCollector`,
`/api/v1/mysql/*`, `frontend/src/features/mysql/*`) is a deep, hardcoded module.
It works, but it cannot be cloned 9 times for PostgreSQL, Redis, Docker, etc.
Without a shared substrate, every new integration adds ~3 weeks of mechanical
work.

This document proposes a substrate built on three new concepts:

1. **Integration** — a registered unit that owns a check executor, signal
   collector, default rule pack, evidence provider, and optional routes.
2. **Target** — the thing being monitored (a PostgreSQL instance, a Docker
   host) decoupled from the checks that evaluate it.
3. **SignalSample** — a normalized metrics envelope (gauges, counters, labels)
   so the rule engine, retention, query API, and UI work across integrations.

### First concrete milestone (do this before anything else)

1. Add the `Integration` interface and `Registry`.
2. Register MySQL behind it without changing its behavior.
3. Add the `Target` and `SignalSample` models with a Mongo-backed repository.
4. Dual-write MySQL samples into the generic signal store.
5. Ship a generic integration catalog UI alongside the existing MySQL pages.
6. Use PostgreSQL as the proof the substrate is reusable. If PostgreSQL
   doesn't feel obviously easier to build than MySQL was, stop and rework the
   substrate before adding more integrations.

### Explicitly out of scope for this plan

- **Generic ingestion (Prometheus scrape, OTLP, custom metric webhooks).**
  This is a product pivot ("we receive" vs "we collect") and deserves its own
  ADR after the substrate is proven. Tracked separately.
- **Kubernetes integration.** Real Kubernetes monitoring (events, RBAC,
  multi-cluster, kubeconfig flows, metrics-server, CrashLoopBackOff detection)
  is a product in itself. It will get its own ADR once Docker, PostgreSQL, and
  Redis prove the substrate. Until then, Kubernetes users can use the existing
  `command` and `api` check types against `kubectl` or the Kubernetes API.
- **Runtime in-process plugins (Go `.so`).** Operationally awkward,
  platform-sensitive, not pursued. All integrations are compiled in.
- **Multi-cluster / multi-tenant target RBAC.** Out of scope for v1. The
  `Target.Labels` field is reserved for future use.

## Purpose

HealthOps currently has a strong foundation for basic health checks, incidents,
notifications, evidence, AI-assisted analysis, status pages, and documentation.
The next major product step is to make monitoring integrations pluggable enough
to support popular databases and infrastructure services without duplicating the
current MySQL implementation for every new technology.

This plan describes how to evolve HealthOps from a set of built-in monitoring
features into an integration-driven monitoring platform that can support:

- Databases: MySQL, PostgreSQL, Redis, MongoDB, SQL Server, Elasticsearch, OpenSearch.
- Runtimes: Docker, Kubernetes, systemd, process checks.
- Web infrastructure: Nginx, Apache, HAProxy, CDN endpoints.
- Queues and streams: Kafka, RabbitMQ, SQS-compatible systems.
- Generic observability inputs: Prometheus metrics, OpenTelemetry signals, webhook events.

The goal is not to become a full Datadog/New Relic replacement. The goal is to
make HealthOps a practical self-hosted incident, evidence, status, and RCA layer
for small and mid-size operations teams.

## Current State

### What Is Already Pluggable

HealthOps already has a check executor registry:

```go
type CheckExecutor interface {
    Type() string
    ApplyDefaults(check *CheckConfig)
    Validate(check *CheckConfig, cfg *Config) error
    Execute(ctx context.Context, r *Runner, check CheckConfig, result *CheckResult) error
}
```

Check types register themselves with:

```go
RegisterCheckExecutor(exec CheckExecutor)
```

This means simple check types such as `api`, `tcp`, `dns`, `ssl`, `ping`,
`domain`, `heartbeat`, `ssh`, and `mysql` are already partly plugin-like.

### What Is Not Yet Pluggable

MySQL is only plugin-like at the basic check level. The deeper MySQL experience
is still a hardcoded vertical module:

- `CheckConfig.MySQL`
- `Runner.runMySQL`
- `MySQLSample`
- `MySQLDelta`
- `MySQLRuleEngine`
- `MySQLEvidenceCollector`
- `/api/v1/mysql/*` routes
- `frontend/src/features/mysql/*`
- MySQL-specific repositories
- MySQL-specific dashboard pages

This is acceptable for one deep integration, but it will not scale cleanly to
PostgreSQL, Redis, MongoDB, Docker, Kubernetes, Kafka, and other systems.

## Product Principles

1. Every integration must help an operator make a decision.
   Raw metrics alone are not enough. Each integration should produce health,
   evidence, alerts, and next actions.

2. Integrations should share the same lifecycle.
   Configure target, collect signals, normalize data, evaluate rules, open
   incidents, capture evidence, notify, explain, remediate, and document.

3. Common UI should cover most integrations.
   Do not create one sidebar item and one bespoke page set for every technology.

4. Domain-specific detail should exist only where it matters.
   PostgreSQL locks, Redis slowlog, Docker restarts, and Kubernetes pod events
   deserve specialized views, but they should sit inside a common integration
   framework.

5. Secrets must be handled consistently.
   No integration should invent its own password, token, TLS, or DSN behavior.

6. AI should be optional and evidence-grounded.
   Integrations should provide structured context so AI features can explain
   incidents without fabricating missing facts.

## Integration Levels

Not every technology needs the same implementation depth on day one.

| Level | Name | Purpose | Examples |
| --- | --- | --- | --- |
| 1 | Check plugin | Pass/fail health check with metrics | TCP, DNS, SSL, Redis ping |
| 2 | Signal integration | Periodic metrics/events with rules and evidence | PostgreSQL, Redis, MongoDB |
| 3 | Product module | Deep pages, actions, rule packs, evidence, remediation | MySQL, Kubernetes |

This keeps HealthOps practical. Redis can start as Level 2. Kubernetes probably
needs Level 3. Nginx may start as Level 1 or 2.

## Target Architecture

### Directory Layout

```text
backend/internal/monitoring/integrations/
  registry.go
  types.go
  targets.go
  connections.go
  signals.go
  rules.go
  evidence.go
  routes.go

backend/internal/monitoring/integrations/mysql/
  integration.go
  collector.go
  rules.go
  evidence.go
  routes.go

backend/internal/monitoring/integrations/postgres/
backend/internal/monitoring/integrations/redis/
backend/internal/monitoring/integrations/mongodb/
backend/internal/monitoring/integrations/docker/
backend/internal/monitoring/integrations/kubernetes/
backend/internal/monitoring/integrations/nginx/
```

Frontend:

```text
frontend/src/features/integrations/
  api/integrations.ts
  pages/IntegrationCatalog.tsx
  pages/IntegrationOverview.tsx
  pages/TargetDetail.tsx
  pages/TargetSignals.tsx
  pages/TargetRules.tsx
  pages/TargetEvidence.tsx
  components/IntegrationCard.tsx
  components/MetricCard.tsx
  components/SignalChart.tsx
  components/SignalTable.tsx
  components/TargetStatusBadge.tsx
```

Existing MySQL pages can remain as compatibility routes during migration, but
new integrations should use the generic integration UI first.

## Backend Contracts

### Integration Interface

```go
type Integration interface {
    ID() string
    Name() string
    Category() IntegrationCategory
    Capabilities() IntegrationCapabilities

    CheckExecutor() CheckExecutor
    Collector() SignalCollector
    RulePack() []AlertRule
    EvidenceProvider() EvidenceProvider
    RegisterRoutes(mux *http.ServeMux)
}
```

### Integration Category

```go
type IntegrationCategory string

const (
    IntegrationDatabase IntegrationCategory = "database"
    IntegrationRuntime  IntegrationCategory = "runtime"
    IntegrationQueue    IntegrationCategory = "queue"
    IntegrationWeb      IntegrationCategory = "web"
    IntegrationGeneric  IntegrationCategory = "generic"
)
```

### Capabilities

```go
type IntegrationCapabilities struct {
    HealthCheck     bool `json:"healthCheck"`
    Metrics         bool `json:"metrics"`
    Logs            bool `json:"logs"`
    Events          bool `json:"events"`
    Queries         bool `json:"queries"`
    Processes       bool `json:"processes"`
    Replication     bool `json:"replication"`
    Remediation     bool `json:"remediation"`
    AIContext       bool `json:"aiContext"`
    StatusPageInput bool `json:"statusPageInput"`
}
```

### Integration Registry

```go
type Registry struct {
    mu           sync.RWMutex
    integrations map[string]Integration
}

func (r *Registry) Register(integration Integration) error
func (r *Registry) Lookup(id string) (Integration, bool)
func (r *Registry) Catalog() []IntegrationManifest
```

The registry should be initialized during service startup. Built-in
integrations are compiled into the binary first. External runtime plugins are
not recommended in the first version because Go `.so` plugins are operationally
awkward and platform-sensitive.

### Relationship to the existing `RegisterCheckExecutor`

Today, check types (`api`, `tcp`, `mysql`, ...) self-register via
`RegisterCheckExecutor`. After Phase 1, that function becomes a private
implementation detail of `Registry.Register()`. Specifically:

- `Integration.CheckExecutor()` returns the executor that the registry will
  pass to the existing executor map.
- Code outside `backend/internal/monitoring/integrations/` MUST NOT call
  `RegisterCheckExecutor` directly.
- During Phase 1 both call paths coexist; by end of Phase 6 the direct path is
  removed and a `go vet`/lint rule should enforce it.

This keeps backwards compatibility while making the registry the single source
of truth.

## Target Model

The app should distinguish between a check and a monitored target.

A target is the thing being monitored:

```go
type Target struct {
    ID          string            `json:"id" bson:"id"`
    Name        string            `json:"name" bson:"name"`
    Type        string            `json:"type" bson:"type"`
    Category    string            `json:"category" bson:"category"`
    Connection  ConnectionRef     `json:"connection" bson:"connection"`
    Tags        []string          `json:"tags,omitempty" bson:"tags,omitempty"`
    Labels      map[string]string `json:"labels,omitempty" bson:"labels,omitempty"`
    Enabled     bool              `json:"enabled" bson:"enabled"`
    CreatedAt   time.Time         `json:"createdAt" bson:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt" bson:"updatedAt"`
}
```

A check evaluates a target:

```go
type CheckConfig struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Type     string `json:"type"`
    TargetID string `json:"targetId,omitempty"`
    // existing fields remain for compatibility
}
```

This allows one PostgreSQL target to have multiple checks:

- connection health
- long-running transactions
- replication lag
- lock waits
- slow query volume

## Connection Model

All integrations should use one connection reference model:

```go
type ConnectionRef struct {
    Mode        string `json:"mode"` // direct, dsnEnv, tokenEnv, socket
    Host        string `json:"host,omitempty"`
    Port        int    `json:"port,omitempty"`
    Username    string `json:"username,omitempty"`
    PasswordEnv string `json:"passwordEnv,omitempty"`
    TokenEnv    string `json:"tokenEnv,omitempty"`
    DSNEnv      string `json:"dsnEnv,omitempty"`
    Database    string `json:"database,omitempty"`
    TLS         bool   `json:"tls,omitempty"`
    TLSCAEnv    string `json:"tlsCaEnv,omitempty"`
}
```

Rules:

- Never return raw secrets in API responses.
- Prefer env var references for production.
- Mask secrets in UI.
- Store only references unless encrypted secret storage is explicitly enabled.
- Every integration should validate its required connection fields.

## Generic Signal Model

Avoid creating a permanent primary storage type for every technology such as
`PostgresSample`, `RedisSample`, `DockerSample`, and so on. Use a generic signal
model for everything that participates in rule evaluation.

```go
type SignalSample struct {
    ID          string             `json:"id" bson:"_id"`
    Integration string             `json:"integration" bson:"integration"`
    TargetID    string             `json:"targetId" bson:"targetId"`
    CheckID     string             `json:"checkId,omitempty" bson:"checkId,omitempty"`
    Timestamp   time.Time          `json:"timestamp" bson:"timestamp"`
    Status      string             `json:"status" bson:"status"`
    Message     string             `json:"message,omitempty" bson:"message,omitempty"`
    Gauges      map[string]float64 `json:"gauges,omitempty" bson:"gauges,omitempty"`
    Counters    map[string]float64 `json:"counters,omitempty" bson:"counters,omitempty"`
    Labels      map[string]string  `json:"labels,omitempty" bson:"labels,omitempty"`
}
```

### Why no `Tables` field

An earlier draft included `Tables map[string]json.RawMessage` for shapes like
process lists, slowlogs, and `pg_stat_activity`. We deliberately removed it:

- It would become the dumping ground for everything domain-specific, defeating
  the point of normalization.
- The generic UI cannot render arbitrary JSON, so each integration would end
  up needing a bespoke renderer anyway.
- Storing tables on every sample multiplies storage cost dramatically.

**Domain tables belong in evidence snapshots** (`IncidentSnapshot`), which
already exist and are captured on incident open
rather than every collection interval. If a real-time view of a domain table
is needed in the UI (e.g. live process list), the integration exposes its own
route under `/api/v1/integrations/{integration}/...` and the integration
manifest declares which typed table component to render.

### Cardinality controls

To prevent runaway storage and UI cost:

- Max 50 gauges + counters per sample. Collectors that need more should split
  into multiple targets.
- Max 16 labels per sample.
- Total unique label values per integration: soft cap 10k, hard cap 100k
  (logged + alerted, not silently dropped).
- The signal repository must reject samples that exceed hard limits, with a
  user-facing error so the integration author finds out immediately.

Examples:

| Native metric | Normalized field |
| --- | --- |
| MySQL `Threads_running` | `gauges.db.threads.running` |
| MySQL `Connections` | `gauges.db.connections.current` |
| PostgreSQL active sessions | `gauges.db.sessions.active` |
| PostgreSQL deadlocks | `counters.db.deadlocks` |
| Redis used memory | `gauges.cache.memory.used_bytes` |
| Redis evictions | `counters.cache.evictions` |
| Docker restart count | `counters.container.restarts` |
| Kubernetes pod ready count | `gauges.k8s.pods.ready` |

## Signal Collector Contract

```go
type SignalCollector interface {
    Integration() string
    Collect(ctx context.Context, target Target) (SignalSample, error)
}
```

Collectors should:

- Respect target timeout.
- Return user-facing errors.
- Avoid logging secrets.
- Populate normalized gauges/counters using stable metric paths.
- Return derived gauges (e.g. `db.connections.utilization_pct = connections / max_connections * 100`)
  rather than forcing every rule to recompute them. The MySQL
  `TestMySQLRuleEngine_ZeroMaxConnections` case (divide-by-zero must not
  panic or breach) is the canonical example: derivation lives in the
  collector, not the rule engine.

## Generic Repository

Add one signal repository:

```go
type SignalRepository interface {
    Append(sample SignalSample) error
    Latest(targetID string) (SignalSample, error)
    Recent(targetID string, limit int) ([]SignalSample, error)
    Query(filter SignalQuery) ([]SignalSample, error)
    PruneBefore(cutoff time.Time) error
}
```

This should be backed by MongoDB.

Indexes:

- `{ targetId: 1, timestamp: -1 }`
- `{ integration: 1, timestamp: -1 }`
- `{ checkId: 1, timestamp: -1 }`
- `{ status: 1, timestamp: -1 }`

## Rule Packs

Every integration should ship default alert rules.

```go
type RulePack struct {
    Integration string      `json:"integration"`
    Version     string      `json:"version"`
    Rules       []AlertRule `json:"rules"`
}
```

Rules should refer to normalized fields:

```json
{
  "id": "postgres-connection-utilization-critical",
  "name": "PostgreSQL Connection Utilization Critical",
  "field": "gauges.db.connections.utilization_pct",
  "operator": "greater_than",
  "value": 90,
  "severity": "critical"
}
```

The alert engine should not be MySQL-specific. It should evaluate normalized
signals from any integration.

## Evidence Providers

Evidence is what makes incidents and RCA credible.

```go
type EvidenceProvider interface {
    Integration() string
    Capture(ctx context.Context, target Target, incident Incident) []IncidentSnapshot
}
```

Examples:

### MySQL Evidence

- Latest sample.
- Process list.
- Top query digests.
- User and host connection stats.
- Recent deltas.

### PostgreSQL Evidence

- `pg_stat_activity`.
- Blocking locks.
- Long-running transactions.
- Top query stats from `pg_stat_statements`.
- Replication lag.
- Database stats from `pg_stat_database`.

### Redis Evidence

- `INFO` sections.
- `SLOWLOG GET`.
- Memory fragmentation.
- Connected clients.
- Replication state.

### Docker Evidence

- Container inspect.
- Health status.
- Restart count.
- Last logs.
- CPU and memory usage.

### Kubernetes Evidence

- Pod status.
- Recent events.
- Restart counts.
- Deployment rollout status.
- Node pressure.
- Container waiting reasons.

## API Design

### Integration Catalog

```http
GET /api/v1/integrations/catalog
```

Returns available integrations, categories, capabilities, setup requirements,
and supported UI cards.

### Targets

```http
GET    /api/v1/targets
POST   /api/v1/targets
GET    /api/v1/targets/{id}
PUT    /api/v1/targets/{id}
DELETE /api/v1/targets/{id}
```

### Signals

```http
GET /api/v1/signals?targetId=...&metric=...&from=...&to=...
GET /api/v1/signals/latest?targetId=...
```

### Rules

```http
GET  /api/v1/integrations/{integration}/rules
POST /api/v1/integrations/{integration}/rules/install-defaults
```

### Integration-Specific Routes

Specific integrations may expose extra routes only when generic routes are not
enough:

```http
GET  /api/v1/integrations/mysql/queries
POST /api/v1/integrations/mysql/kill
GET  /api/v1/integrations/postgres/locks
GET  /api/v1/integrations/redis/slowlog
GET  /api/v1/integrations/docker/containers
```

Existing routes such as `/api/v1/mysql/*` should remain as compatibility aliases
during migration.

## Frontend Information Architecture

Avoid adding top-level sidebar items for every integration.

Preferred navigation:

```text
Monitor
  Dashboard
  Targets
  Checks
  Logs

Respond
  Incidents
  Root Cause
  Remediation

Integrations
  Catalog
  Databases
  Runtimes
  Queues
  Web Servers

Configure
  Notifications
  Users
  Settings
```

Integration pages:

```text
/integrations
/integrations/databases
/integrations/mysql
/integrations/mysql/:targetId
/integrations/postgres/:targetId
/integrations/redis/:targetId
/integrations/docker/:targetId
```

## Generic UI Components

Build reusable components:

- `IntegrationCard`
- `TargetList`
- `TargetHealthSummary`
- `MetricCard`
- `MetricTrendChart`
- `SignalTable`
- `RulePackTable`
- `EvidenceTimeline`
- `ConnectionSetupForm`
- `SecretField`
- `CapabilityBadge`

Use integration manifests to configure these components.

```ts
type IntegrationManifest = {
  id: string
  name: string
  category: 'database' | 'runtime' | 'queue' | 'web' | 'generic'
  capabilities: IntegrationCapabilities
  cards: MetricCardDefinition[]
  charts: ChartDefinition[]
  tables: TableDefinition[]
  routes: IntegrationRoute[]
}
```

## AI and RCA Integration

AI should consume the same normalized evidence model.

The RCA context builder should receive:

- Incident.
- Target metadata.
- Latest signal sample.
- Recent signal trend.
- Evidence snapshots.
- Related logs.
- Related checks.
- Rule that triggered the incident.

AI prompt should clearly say when data is missing.

Example context sections:

```text
Target:
- Type: postgres
- Name: production-postgres
- Tags: prod, payments

Signal:
- db.connections.utilization_pct: 94
- db.locks.waiting: 12
- db.transactions.long_running: 4

Evidence:
- pg_stat_activity snapshot
- blocking locks snapshot
- recent slow query summary
```

## Documentation Requirements

Every integration should include embedded help content:

- What it monitors.
- How to configure it.
- Required permissions.
- What each metric means.
- Default alert rules.
- Common failure modes.
- Remediation suggestions.
- Security notes.
- Demo scenario.

Example docs:

```text
backend/internal/monitoring/helpcontent/postgres.md
backend/internal/monitoring/helpcontent/redis.md
backend/internal/monitoring/helpcontent/docker.md
backend/internal/monitoring/helpcontent/kubernetes.md
```

Each feature page should expose a contextual help button.

## Demo Scenario Requirements

Every meaningful integration should have a demo scenario.

Examples:

PostgreSQL:

- Connection outage.
- Long-running transaction.
- Lock contention.
- Replication lag.

Redis:

- Memory pressure.
- Eviction spike.
- Slow command.
- Replication disconnected.

Docker:

- Container unhealthy.
- Restart loop.
- Log error spike.
- High memory usage.

Kubernetes:

- CrashLoopBackOff.
- ImagePullBackOff.
- Pod not ready.
- Deployment rollout stalled.
- Node pressure.

Scenarios should exercise real code paths:

- collector
- rule engine
- incident creation
- evidence capture
- notifications
- AI RCA when configured
- status page updates where relevant

## Migration Plan

### Phase 0: Stabilize Existing MySQL Behavior

Before refactoring:

- Add tests around current MySQL API behavior.
- Add tests around MySQL rule engine behavior.
- Add tests around MySQL evidence capture.
- Add tests around MySQL frontend critical flows if e2e tooling exists.
- Document existing route compatibility requirements.

Deliverable:

- Safety net before moving MySQL into the integration architecture.

### Phase 1: Add Integration Registry

Tasks:

- Create `backend/internal/monitoring/integrations`.
- Add `Integration`, `IntegrationCapabilities`, `Registry`, and `IntegrationManifest`.
- Initialize registry in service startup.
- Expose `GET /api/v1/integrations/catalog`.
- Register a thin MySQL integration manifest without moving all MySQL logic yet.

Acceptance criteria:

- Catalog returns MySQL as a registered integration.
- Existing MySQL pages and APIs still work.
- No change to existing check execution behavior.

### Phase 2: Add Target Model

Tasks:

- Add `Target` and `ConnectionRef` types.
- Add target repository.
- Add target CRUD API.
- Add migration path from existing `CheckConfig.MySQL` to a target reference.
- Keep existing check config compatibility.

Acceptance criteria:

- A MySQL target can be created.
- A check can reference `targetId`.
- Existing MySQL checks still work without target migration.
- Secrets are masked in API responses.

### Phase 3: Add Generic Signal Storage

Tasks:

- Add `SignalSample`.
- Add `SignalRepository`.
- Add MongoDB-backed signal repository.
- Add signal query API.
- Add retention pruning for signal samples.
- Dual-write MySQL samples into generic signal storage.

Acceptance criteria:

- MySQL continues writing old samples.
- MySQL also writes normalized signal samples.
- Generic signal API can return latest and recent MySQL signals.
- Retention removes old signal samples.

### Phase 4: Generic Rule Evaluation

Tasks:

- Add metric path resolver for `gauges.*`, `counters.*`, `labels.*`.
- Make alert rules evaluate against `SignalSample`.
- Convert MySQL default rules to normalized field paths.
- Keep compatibility for existing MySQL rule engine until parity is proven.

Acceptance criteria:

- MySQL rules can be evaluated through generic signal rules.
- Existing incidents still open and resolve correctly.
- Rule state persists across restarts.
- Consecutive breach/recovery semantics still work.

### Phase 5: Generic Evidence Pipeline

Tasks:

- Add integration-aware evidence registry.
- Register MySQL evidence provider through the registry.
- Capture evidence by integration ID and target ID.
- Store evidence using existing snapshot repository.

Acceptance criteria:

- MySQL incident evidence remains available.
- Evidence capture is no longer called directly from `Runner.runMySQL`.
- New integrations can register evidence providers without changing runner code.

### Phase 6: Refactor MySQL Into Full Integration

Tasks:

- Move MySQL collector, rules, evidence, and routes behind `mysql.NewIntegration`.
- Remove MySQL-specific dependencies from `Runner` where possible.
- Keep `/api/v1/mysql/*` as aliases.
- Add `/api/v1/integrations/mysql/*` canonical routes.

Acceptance criteria:

- Existing MySQL UI works.
- New integration UI can display MySQL target health.
- MySQL incidents, rules, evidence, notifications, and AI context still work.
- No duplicate rule evaluation.

### Phase 7: Add PostgreSQL

Tasks:

- Add PostgreSQL check executor.
- Add PostgreSQL signal collector.
- Collect:
  - connection count and utilization
  - `pg_stat_activity`
  - long-running transactions
  - lock waits
  - deadlocks
  - `pg_stat_database`
  - replication lag when available
  - `pg_stat_statements` when installed
- Add default rule pack.
- Add evidence provider.
- Add help docs.
- Add demo scenarios.

Acceptance criteria:

- User can add a PostgreSQL target.
- Health check appears in checks and dashboard.
- PostgreSQL signals appear in generic integration UI.
- PostgreSQL incidents open with evidence.
- Documentation explains required database permissions.

### Phase 8: Add Redis

Tasks:

- Add Redis check executor.
- Collect `INFO`.
- Collect `SLOWLOG`.
- Collect replication state.
- Add default rules:
  - high memory usage
  - evictions spike
  - blocked clients
  - rejected connections
  - replication disconnected
  - slowlog spike
- Add evidence provider.
- Add docs and demo scenarios.

Acceptance criteria:

- Redis target can be monitored.
- Redis incidents include useful evidence.
- Slowlog is visible where configured.

### Phase 9: Add Docker

Tasks:

- Add Docker target type.
- Support Docker socket and remote Docker endpoint.
- Collect container status, health, restarts, image, ports, CPU, memory.
- Capture container logs as evidence.
- Add rules for:
  - container down
  - unhealthy container
  - restart loop
  - high memory
  - high CPU
- Add docs and demo scenarios.

Acceptance criteria:

- Docker target lists containers.
- Container incidents include inspect data and recent logs.
- Restart loops are detected.

### Phase 10: Add Kubernetes

Tasks:

- Add Kubernetes target type.
- Support kubeconfig env var or in-cluster config.
- Collect:
  - nodes
  - namespaces
  - deployments
  - pods
  - pod restarts
  - pod phases
  - waiting reasons
  - events
  - resource usage when metrics server is available
- Add default rules:
  - CrashLoopBackOff
  - ImagePullBackOff
  - pod not ready
  - deployment unavailable
  - rollout stalled
  - node pressure
- Add evidence provider.
- Add docs and demo scenarios.

Acceptance criteria:

- Kubernetes target can be configured.
- Pod/deployment health appears in integration UI.
- Kubernetes events are attached to incidents.
- Missing metrics-server is handled gracefully.

### Phase 11: Generic Ingestion

Tasks:

- Add Prometheus-compatible ingestion or scrape support.
- Add OpenTelemetry OTLP ingestion for metrics/logs/events.
- Add custom metric webhook endpoint.
- Normalize incoming signals into `SignalSample`.
- Add rule support for custom metrics.

Acceptance criteria:

- Users can ingest arbitrary metrics without native integration support.
- Custom metric rules can open incidents.
- Ingested evidence can be used by RCA.

## Integration Priority

Recommended order:

1. MySQL refactor into integration framework.
2. PostgreSQL.
3. Redis.
4. Docker.
5. MongoDB.
6. Kubernetes.
7. Nginx/HAProxy.
8. Kafka/RabbitMQ.
9. Elasticsearch/OpenSearch.
10. Prometheus/OpenTelemetry generic ingestion.

Reasoning:

- PostgreSQL is the most important database gap.
- Redis is common and relatively easy to collect.
- Docker is highly valuable for self-hosted users.
- MongoDB is natural because HealthOps already uses MongoDB internally.
- Kubernetes is high-value but larger in scope.
- Generic ingestion prevents endless one-off integration work.

## Testing Strategy

### Unit Tests

- Integration registry duplicate detection.
- Target validation.
- Connection validation and secret masking.
- Signal path resolution.
- Rule evaluation over normalized signals.
- Evidence provider registration.
- Retention pruning.

### Contract Tests

Every integration should pass a shared contract suite:

- registers with non-empty ID/name/category
- exposes valid capabilities
- validates missing connection details
- collects sample with required fields
- returns no raw secrets
- provides default rules with valid metric paths
- evidence capture does not panic on missing optional data

### Integration Tests

Use Docker Compose services for:

- MySQL
- PostgreSQL
- Redis
- MongoDB
- Docker-in-Docker or socket-based Docker test where safe

### E2E Tests

Core flows:

- add target
- add check
- run check
- see signal on integration page
- trigger rule
- open incident
- view evidence
- resolve incident
- verify notification queued

## Security Requirements

- No raw DSNs in API responses.
- No secrets in logs.
- Password fields masked in UI.
- Target test connection endpoint must not return secret-bearing errors.
- Integration routes require auth unless explicitly public.
- Docker and Kubernetes integrations need clear warnings about permissions.
- Remote command-like capabilities must be opt-in.
- Evidence snapshots must redact secrets from logs and query text where possible.

## Performance Requirements

- Collectors must respect per-target timeout.
- Integration collection should be concurrent but bounded.
- Slow integrations must not block unrelated check types.
- Signal samples must be pruned by retention policy.
- High-cardinality labels must be controlled.
- Tables in `SignalSample.Tables` should be capped by integration-specific limits.
- UI should paginate large tables such as process lists, pods, containers, and query digests.

## Backward Compatibility

Keep these stable during migration:

- Existing checks API payloads.
- Existing MySQL check config.
- Existing `/api/v1/mysql/*` routes.
- Existing frontend `/mysql` routes.
- Existing incidents and snapshots.
- Existing notification filtering by check type.

Deprecation should happen only after:

- Generic integration UI reaches parity.
- Existing MySQL pages are either migrated or intentionally kept as specialist pages.
- Documentation explains the new target/integration model.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Over-abstracting too early | Slower product delivery | Refactor MySQL first, then prove with PostgreSQL |
| Generic UI becomes too vague | Operators lose useful detail | Allow specialist tabs for domain-specific workflows |
| Metric cardinality grows too fast | Storage and UI performance issues | Cap labels, tables, and retention |
| Secrets leak through errors | Security issue | Centralize connection handling and redaction |
| Duplicate alert paths | Duplicate incidents | Migrate one rule engine at a time with tests |
| Kubernetes scope explosion | Long delivery cycle | Ship Docker and PostgreSQL first |

## Definition of Done for a New Integration

An integration is production-ready when it has:

- Registered integration manifest.
- Target setup form.
- Connection validation.
- Health check executor.
- Signal collector.
- Normalized metrics.
- Default rule pack.
- Incident evidence provider.
- At least one demo scenario.
- Embedded help documentation.
- Tests for collector, rules, and evidence.
- UI pages using generic integration components.
- Secret masking and redaction.
- Retention support.
- RCA context support.

## First Concrete Milestone

The first milestone should not be "add PostgreSQL immediately." It should be:

1. Create integration registry.
2. Register MySQL as the first integration.
3. Add generic target and signal models.
4. Dual-write MySQL metrics into generic signal storage.
5. Build generic integration catalog UI.
6. Keep existing MySQL pages working.

Once that is complete, PostgreSQL becomes the proof that the architecture is
actually reusable.

## References

- PostgreSQL monitoring docs: https://www.postgresql.org/docs/current/monitoring.html
- Redis INFO command: https://redis.io/docs/latest/commands/info/
- MongoDB serverStatus: https://www.mongodb.com/docs/manual/reference/method/db.serverstatus/
- OpenTelemetry docs: https://opentelemetry.io/docs/
- Kubernetes metrics server: https://github.com/kubernetes-sigs/metrics-server
