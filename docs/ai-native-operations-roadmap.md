# AI-Native Operations Roadmap

Status: draft for product and implementation planning
Last updated: 2026-05-16

## Goal

Make HealthOps an AI-native operations platform: every signal, workflow, incident, and configuration change should produce structured context that helps the system detect, explain, prioritize, and resolve operational problems.

This should not become a generic dashboard clone. The useful product shape is an opinionated incident intelligence system for small and mid-sized teams that need practical monitoring, log intelligence, root-cause assistance, and safe automation without paying for a full enterprise observability suite.

## Research Summary

The current market pain is consistent across SRE, DevOps, platform, and incident-management sources:

- Teams collect too much telemetry but use little of it. Sawmills' 2025 telemetry report says only 13% of collected telemetry is actively used, and 70% of respondents identify high log ingestion/indexing as a top cost driver.
- Correlation is still the real problem. OpenTelemetry's logging specification explicitly emphasizes correlating logs with traces and metrics through common context such as trace IDs, span IDs, timestamps, and resource attributes.
- AI is already entering incident management, but trust and governance are blockers. Atlassian's 2025 incident-management research reports that 74% of respondents cite security risk as a top barrier to expanding AI use, even while many teams are exploring AI for incident trending.
- AIOps fails when operational knowledge is not AI-ready. Thoughtworks' 2025 AIOps lessons from production PoCs call out missing AI governance and fragmented/unstructured operational knowledge as core reasons PoCs fail.
- OpenTelemetry adoption is rising and should be treated as the integration standard. Grafana's 2024 observability survey notes OpenTelemetry is a major CNCF project with broad investigation/adoption, while CNCF's 2024 survey reflects continued cloud-native adoption.
- SRE teams still rely heavily on dashboards, alerts, and synthetic monitoring. Catchpoint's SRE Report 2026 reports dashboards/alerts and synthetic probes as common practices, with AI-based anomaly detection emerging but not yet universal.
- DORA's 2024 report is a warning: AI can improve individual productivity, but it can also hurt delivery stability/throughput unless teams keep fundamentals like small batches, robust testing, and user-centricity.

Sources:

- DORA 2024 Report: https://dora.dev/research/2024/dora-report/
- OpenTelemetry Logs specification: https://opentelemetry.io/docs/specs/otel/logs/
- OpenTelemetry Semantic Conventions: https://opentelemetry.io/docs/concepts/semantic-conventions/
- Atlassian 2025 State of AI Incident Management: https://www.atlassian.com/incident-management/2025-state-of-incident-management
- Thoughtworks AIOps lessons 2025: https://www.thoughtworks.com/en-us/insights/blog/generative-ai/aiops-what-we-learned-in-2025
- Grafana Observability Survey 2024: https://grafana.com/observability-survey/2024/
- CNCF Annual Survey 2024: https://www.cncf.io/reports/cncf-annual-survey-2024/
- Catchpoint SRE Report 2026: https://observability.com/wp-content/uploads/2026/02/The-SRE-Report-2026-Catchpoint.pdf
- Sawmills 2025 Telemetry and Observability Report: https://www.sawmills.ai/observability-report-2025
- PagerDuty State of AI-First Operations: https://www.pagerduty.com/state-of-ai-first-operations/

## Product Principles

1. AI must be evidence-first.
   Every AI answer must cite the checks, logs, metric windows, deployments, audit events, and previous incidents it used. No unexplained RCA.

2. AI must reduce operator work, not generate more text.
   Good output is a ranked action list, a timeline, a suspected cause, and a safe next command or runbook step.

3. Telemetry must be normalized before AI sees it.
   Raw logs alone are not enough. Normalize severity, service, host, environment, trace ID, fingerprint, deployment version, and incident relation.

4. Correlation beats generic anomaly detection.
   Start with deterministic joins: same host, same check, same service, same time window, same deployment marker, same trace ID, same fingerprint.

5. Keep costs visible.
   Log ingestion volume, high-cardinality tags, retention, and unused telemetry should be product surfaces, not hidden backend details.

6. Human approval remains mandatory for destructive remediation.
   AI can recommend and prepare actions. It should not kill queries, restart services, roll back deployments, or modify alerting without explicit authorization.

## Non-Goals (v1)

Explicitly out of scope so the product stays focused and does not drift toward a general-purpose observability suite:

- Distributed tracing storage backend. We accept trace IDs as correlation keys and link out to existing tracing systems (Jaeger, Tempo, Honeycomb, Datadog). We do not store spans.
- Long-term metrics TSDB. We keep short-window aggregates for incident evidence only. Users keep Prometheus/Mimir/Cloud TSDB for long-term metric history.
- APM auto-instrumentation agents. We ingest OTLP from existing agents; we do not ship our own.
- Frontend RUM and session replay.
- Multi-tenant SaaS billing/isolation in v1. Single-tenant deployments only. The signal envelope reserves a `tenantId` field for future use, but tenant enforcement is not implemented.
- General-purpose log search/grep over source logs at scale. We index fingerprints and short-window redacted log windows around incidents only. For full-text log archives at scale, users keep Loki/Elastic/CloudWatch.
- Synthetic browser monitoring with full DOM scripting in v1. HTTP-step flows only.

If a request lands in this list, the answer is “not in v1” unless the principle changes via ADR.

## Real User Problems To Solve

### 1. "Something is broken but I do not know where to start"

Current user workflow in many teams:

- Alert fires.
- Operator opens dashboard.
- Operator checks logs.
- Operator checks recent deploys.
- Operator asks who changed something.
- Operator tries to remember previous incidents.

HealthOps should instead create an Incident Brief automatically:

- What changed?
- What failed first?
- Which users/services are impacted?
- Which evidence supports the suspected cause?
- What should I check next?

Feature: AI Incident Brief

- Triggered when an incident opens or worsens.
- Pulls check results, logs, metrics, deployments, audit events, server data, MySQL data, and similar incidents.
- Produces a concise, evidence-linked summary.

Minimum useful output:

```text
Likely cause: MySQL connection saturation after deploy api@1.14.3
Confidence: medium
Evidence:
- checkout-api 500 rate started at 10:42:13
- MySQL connections rose from 42% to 96% between 10:40 and 10:43
- top query fingerprint abc123 appeared 17x more often after deploy api@1.14.3
- no CPU or disk pressure on app servers
Next actions:
1. Inspect query fingerprint abc123
2. Check deploy diff for checkout repository data access layer
3. Increase pool limit only if rollback is not possible
```

### 2. "Logs are noisy and expensive"

Users need log monitoring, but unbounded full-text log ingestion becomes noisy and costly.

Feature: Log Intelligence Pipeline

Inputs:

- SSH file tailing
- Docker container logs
- HTTP log ingestion endpoint
- Syslog-compatible receiver later
- OpenTelemetry/OTLP logs receiver

Pipeline:

```text
Ingest
→ Parse
→ Normalize
→ Redact secrets
→ Enrich with service/host/env/deployment
→ Fingerprint
→ Sample/drop according to policy
→ Store redacted source event short-term
→ Store fingerprint aggregates longer-term
→ Correlate to incidents
```

Core capabilities:

- Error fingerprinting by stack trace, message template, SQL code, exception type.
- Spike detection per fingerprint.
- Noise suppression for repeated identical events.
- Secret redaction before storage and AI analysis.
- Retention tiers: redacted source logs short-term, aggregates longer-term.

Why this matters:

- Users do not need every info/debug log indexed forever.
- They need the right error groups and evidence around incidents.

### 3. "Five alerts are actually one incident"

Alert fatigue comes from duplicate symptoms.

Feature: Incident Correlation Engine

Correlation keys:

- time window
- check ID
- host/server ID
- application/service
- deployment ID
- trace ID/span ID
- log fingerprint
- database instance
- remote server
- dependency endpoint

Behavior:

- Group related check failures and log spikes into one incident.
- Track contributing signals as incident events.
- Update severity based on blast radius and persistence.
- Avoid paging repeatedly for the same underlying issue.

Implementation model:

```text
Incident
IncidentEvent
CorrelationGroup
SignalEvidence
```

### 4. "Was this caused by a deployment?"

Feature: Deployment Intelligence

APIs:

```http
POST /api/v1/deployments
GET /api/v1/deployments
GET /api/v1/deployments/{id}/impact
```

Deployment fields:

- service
- environment
- version/commit SHA
- repository
- author
- startedAt / finishedAt
- change summary
- CI URL
- rollback command or runbook URL

AI-native behavior:

- Detect incidents that start after deployment.
- Compare before/after latency, errors, checks, MySQL metrics, logs.
- Generate deploy impact analysis.
- Suggest rollback when confidence is high and error budget burn is severe.

### 5. "Our cron job silently failed"

Feature: Heartbeat and Job Monitoring

API:

```http
POST /api/v1/heartbeats/{jobId}
```

Use cases:

- backup completed
- invoice generation completed
- ETL completed
- cache warmup completed
- certificate renewal completed
- report generation completed

AI-native behavior:

- Explain missed heartbeat based on recent logs, host status, deployments, and previous job runs.
- Detect runtime drift: job usually takes 3 minutes, now takes 27 minutes.
- Summarize failed job history and likely owner.

### 6. "We only know uptime, not reliability"

Feature: SLO and Error Budget Monitoring

Objects:

- SLO target
- service objective
- burn-rate window
- error budget policy
- user-facing dependency map

Initial SLOs:

- availability from check results
- latency from check duration or API metrics
- MySQL query latency/error indicators
- heartbeat freshness

AI-native behavior:

- Summarize why error budget is burning.
- Suggest which alerts are customer-impacting vs noisy.
- Create weekly reliability review.

### 7. "The dashboard is green but users are unhappy"

Feature: User Journey and Synthetic Flow Monitoring

Beyond single endpoint checks, users need multi-step checks:

- login
- search
- checkout
- create record
- payment callback
- background job completion

Data model:

- SyntheticFlow
- SyntheticStep
- SyntheticRun
- StepEvidence

AI-native behavior:

- Identify which step failed first.
- Compare current failure to previous similar flow failures.
- Attach screenshots/HTTP traces later.

### 8. "A dependency broke us"

Feature: Dependency and Third-Party Monitoring

Track:

- external APIs
- payment gateways
- email providers
- object storage
- DNS providers
- queue brokers
- databases

AI-native behavior:

- Identify if multiple internal services are failing because of one dependency.
- Suppress duplicate internal alerts and create one dependency incident.
- Suggest customer communication template.

### 9. "Security events look like ops events"

Feature: Security-Aware Operations

Signals:

- repeated auth failures
- unusual admin activity
- check disabled
- alert rule deleted
- notification channel disabled
- suspicious webhook target
- command checks enabled
- AI provider key changed

AI-native behavior:

- Classify as operational, security, or mixed incident.
- Produce evidence timeline.
- Recommend containment steps.
- Keep security events out of generic noisy logs.

### 10. "We need postmortems but nobody has time"

Feature: AI Post-Incident Review Assistant

Outputs:

- timeline
- impact
- root cause hypothesis
- contributing factors
- detection gaps
- what worked
- action items
- owner suggestions
- follow-up checks to add

Important constraint:

AI-generated postmortems must be draft artifacts. Humans approve final root cause and action items.

### 11. "We keep solving the same incident repeatedly"

Feature: Incident Memory and Similarity Search

Inputs:

- previous incident summaries
- fingerprints
- checks involved
- services involved
- deployment metadata
- remediation outcome
- postmortem action items

AI-native behavior:

- Show similar incidents during triage.
- Suggest proven remediation from previous incidents.
- Detect recurring incidents with failed action items.

### 12. "We do not know which alerts are useful"

Feature: Alert Quality Scoring

Track per alert rule:

- pages generated
- incidents created
- duplicates suppressed
- acknowledgements
- time to resolve
- false positive marking
- no-action incidents

AI-native behavior:

- Recommend threshold changes.
- Detect alerts that never lead to action.
- Suggest converting noisy symptoms into better service-level alerts.

### 13. "Telemetry broke because instrumentation changed"

Feature: Telemetry Quality Monitor

Detect:

- missing service names
- missing environment tags
- high-cardinality labels
- sudden ingestion spikes
- missing trace IDs in logs
- unparseable log formats
- broken heartbeat clients

AI-native behavior:

- Explain why telemetry quality declined.
- Suggest code or collector config changes.
- Estimate data-volume impact.

### 14. "AI systems themselves need monitoring"

Feature: AI Workload Monitoring

Track:

- provider latency
- provider errors
- token usage
- cost by provider/model/use case
- prompt version
- tool-call failures
- hallucination/low-confidence reports marked by users
- safety refusals
- missing evidence citations

HealthOps should monitor its own AI layer as a production dependency.

AI-native behavior:

- Detect provider degradation.
- Recommend fallback provider/model.
- Show which AI analyses were useful vs ignored.

## AI-Native Architecture

### Storage Decision (binding)

**MongoDB is the primary and only persistence layer for all AI-native operations data.**

- All new collections (`signal_events`, `log_events`, `log_fingerprints`, `correlation_groups`, `incident_events`, `deployments`, `heartbeats`, `slo_*`, `ai_*`, `runbooks`, `remediation_actions`, `telemetry_quality_findings`) are written directly to MongoDB. No file-store fallback for these collections.
- The legacy `FileStore` / JSONL repositories (`state.json`, MySQL JSONL samples, file AI queue, file notification outbox) are migration inputs only. Phase 0 migrates them into MongoDB and removes runtime use.
- Reads and writes for AI features assume MongoDB is reachable. The service refuses to start AI features (Incident Brief, log ingestion, correlation) if MongoDB is unavailable. After Phase 0, persisted monitoring also treats MongoDB as required; `/healthz` reports degraded/unready rather than silently writing to files.
- This supersedes the existing “hybrid store / best-effort mirror” pattern for new features. Captured in [ADR 005](decisions/ADR-005-mongodb-primary-persistence.md).

Indexing baselines (every collection):

- `{ timestamp: -1 }` for time-window queries.
- `{ service: 1, environment: 1, timestamp: -1 }` for service-scoped lookups.
- `{ incidentId: 1, timestamp: 1 }` on evidence-bearing collections.
- `{ fingerprint: 1, timestamp: -1 }` on log-derived collections.
- TTL indexes per retention policy (redacted source logs short, fingerprints/aggregates long).

### Core Pipeline

```text
Collectors
  API checks
  TCP checks
  MySQL checks
  SSH/server metrics
  log tailers
  deployment markers
  heartbeat pings
  audit events
  OTLP log ingest

Normalization Layer
  parse
  redact
  enrich
  apply semantic fields
  fingerprint
  assign service/env/host/deployment

Signal Store
  redacted source events
  aggregate windows
  fingerprints
  check results
  incidents
  evidence links

Correlation Engine
  deterministic grouping
  anomaly windows
  causality hints
  incident updates

AI Context Builder
  retrieve bounded evidence
  summarize long logs
  include similar incidents
  include runbooks
  include deployment/audit history

AI Reasoning Layer
  classify
  explain
  recommend
  draft postmortem
  suggest remediation

Human Control Layer
  approve action
  mark useful/not useful
  edit root cause
  create follow-up work
```

### Mongo Collections

All AI-native data lives here. No file-store equivalents.

```text
log_sources
log_events                  # redacted source events, short TTL (e.g. 7d)
log_fingerprints            # aggregated, long TTL (e.g. 90d)
signal_events               # unified envelope across all signal types
correlation_groups
incident_events             # timeline events linked to incidents
deployments
heartbeats
slo_definitions
slo_windows
ai_investigations           # AI run inputs, outputs, evidence refs, confidence
ai_feedback                 # human ratings on AI outputs
runbooks
remediation_actions
telemetry_quality_findings
```

### Common Signal Schema

Every signal fits a common envelope. This is the contract every collector, normalizer, correlator, and AI context builder uses.

```json
{
  "id": "sig_...",
  "tenantId": "default",
  "type": "log|check|metric|deployment|heartbeat|audit|security",
  "timestamp": "2026-05-16T12:00:00Z",
  "severity": "info|warning|critical",
  "service": "checkout-api",
  "environment": "production",
  "host": "app-01",
  "source": "docker|ssh|api|mysql|otel",
  "fingerprint": "fp_...",
  "traceId": "...",
  "spanId": "...",
  "deploymentId": "dep_...",
  "incidentId": "inc_...",
  "message": "normalized message",
  "attributes": {},
  "redactionStatus": "clean|redacted|blocked"
}
```

`tenantId` is reserved (defaults to `"default"` in v1 single-tenant deployments) so a future multi-tenant rollout does not require a schema migration. v1 still enforces user/session boundaries; true cross-tenant authorization must be added before SaaS or multi-tenant mode.

### Redaction Model (allowlist)

Aligned with principle 3 (“normalized before AI sees it”) and the safety requirements:

- Allowlist, not blocklist. The pipeline keeps only fields/patterns explicitly permitted; everything else in `attributes` is dropped or hashed.
- Built-in detectors run before persistence: emails, IPv4/IPv6, JWTs, AWS keys, bearer tokens, credit-card-shaped numbers, common DSN/URL credentials, private-key headers.
- Detected secrets are replaced with a stable hash (`<redacted:sha256:abcd1234>`) so spike detection and fingerprinting still work.
- `redactionStatus`:
  - `clean` — no detectors fired.
  - `redacted` — detectors fired and were replaced.
  - `blocked` — detector fired with high confidence on a forbidden field; the event is dropped and a `telemetry_quality_findings` record is created.
- Redaction runs **before** writing to MongoDB and **before** any AI provider call. There is no “raw” copy stored anywhere.

### Fingerprinting

- Logs: Drain3-style template extraction (numbers/UUIDs/IPs/hex blobs replaced with placeholders), hashed to a stable `fingerprint`. Per-service cardinality cap (default 5,000); on overflow, low-volume fingerprints are coalesced into `fp_other_<service>` and a quality finding is raised.
- MySQL: existing query digest from `performance_schema` reused.
- Stack traces: top 3 frames + exception type, language-aware where possible (Go, Node, Python, JVM).

**Implementation requirements (Phase 2b):**

- Baseline window: 24h rolling window for "normal" fingerprint frequency.
- Spike threshold: fingerprint count > `baseline_mean + 3 * baseline_stddev` within a 5-minute tumbling window, OR absolute count ≥ `10×` baseline mean (whichever fires first).
- Aggregation schema: `{ fingerprint, service, environment, window_start, window_end, count, sample_message, first_seen, last_seen, linked_incidents[] }` in `log_fingerprints` collection.
- Cap behavior: when a service exceeds 5,000 distinct fingerprints, the lowest-frequency 10% are merged into `fp_other_<service>`. The merge is logged as a `telemetry_quality_findings` event. Capped fingerprints are un-merged if their original template reappears above threshold in a subsequent window.
- Test fixtures: at least 20 fixture log streams (covering Go panics, Node uncaught exceptions, Python tracebacks, MySQL errors, syslog) with expected fingerprint outputs. These gate merge in CI.

### Correlation Algorithm (Phase 4)

Deterministic-first, similarity-second. Online during ingest.

1. **Hard-key join** on identical `(service, environment, deploymentId)` within a 5-minute sliding window. Same group.
2. **Trace join**: any signals sharing `traceId` are in the same group regardless of service.
3. **Host join**: signals on the same `host` within 60s of a critical event join the host’s active group.
4. **Fingerprint similarity** (fallback): cosine similarity ≥ 0.85 on fingerprint vectors within window joins the group.
5. **Conflict resolution**: when keys disagree (e.g. same `traceId` but different `deploymentId`), `traceId` wins; the deployment becomes a contributing factor, not a separate group.
6. **Group lifecycle**: a group closes after 15 minutes of no new signals OR when its parent incident is resolved.

Groups are recomputed only forward-in-time; closed groups are immutable. Reconciliation (re-grouping historical data) is an explicit batch job, not automatic.

### AI Confidence Score

Deterministic, evidence-weighted. Not LLM self-report.

```
confidence = clip(
    0.2 * has_deployment_correlation
  + 0.2 * has_metric_anomaly
  + 0.2 * has_log_fingerprint_spike
  + 0.2 * has_similar_past_incident_with_same_root_cause
  + 0.2 * evidence_count_normalized,
  0, 1
)
```

`evidence_count_normalized = min(evidence_count / 10, 1.0)`, where `evidence_count` counts unique, deduplicated evidence references included in the AI context.

Mapped to bands: `low` (<0.4), `medium` (0.4–0.75), `high` (>0.75). The breakdown is shown in the UI so operators can see why a brief is rated as it is. Calibration against `ai_feedback` outcomes is a Phase 5 task.

### AI Cost and Latency Budget

Applies to every AI call HealthOps makes (Incident Brief, postmortem draft, log group explanation, etc.):

- Hard timeout: 20s per provider call. Fallback chain (configured per `AIService`) kicks in on timeout or 5xx.
- Token cap: 8k input / 1k output per call by default; tunable per use case.
- Evidence cap: max 50 evidence items in context; over the cap, items are summarized by deterministic rollups (counts, top-N fingerprints) before being sent.
- Per-incident AI spend cap (configurable, default $0.50 USD-equivalent across all providers); exceeded calls are queued for manual trigger only.
- Provider/model price table is maintained in MongoDB-backed AI config as `modelPricing` with provider, model, input token cost, output token cost, currency, and effective date.
- Provider rate limit: token-bucket per provider in `AIService`, leaving 20% headroom for the user’s own application of the same key.
- All costs and latencies recorded in `ai_investigations` for the AI Workload Monitoring feature.

## AI Features By Application Area

### Dashboard

- AI daily operations summary.
- Top risks today.
- Noisy alerts needing cleanup.
- Services with worsening reliability.
- Cost/telemetry anomalies.

### Checks

- AI-generated check suggestions from incidents and logs.
- Detect checks that never catch real incidents.
- Recommend better thresholds.
- Explain why a check is flaky.

### Incidents

- AI incident brief.
- Related logs, metrics, traces, deployments, audit events.
- Similar previous incidents.
- Suggested owner/team.
- Suggested customer impact statement.

### Logs

- Live log stream.
- Error groups.
- Fingerprint trend.
- AI explanation per error group.
- Redaction and parsing quality.

### MySQL

- Query fingerprint RCA.
- Slow query regression after deploy.
- Lock/deadlock explanation.
- Connection pool saturation analysis.
- Suggested indexes/query rewrites as hypotheses.

### Servers

- Process anomaly explanation.
- Disk growth cause from logs/processes.
- Memory leak suspicion based on process trend.
- SSH collector health.

### Notifications

- Alert routing recommendations.
- Channel health and missed delivery analysis.
- Noise score by channel.

### Settings

- AI config review: risky command checks, missing auth, weak retention, missing notification route.
- Telemetry cost preview before enabling a log source.

### Reports

- Weekly reliability report.
- Incident review draft.
- SLO burn report.
- Telemetry cost report.
- Alert quality report.

## Phase 0/1 Implementation Contract

This section makes the first two phases implementation-ready by specifying mechanics that the phase descriptions leave abstract.

### MongoDB Consistency Note

The "Storage Decision" section above states that MongoDB is the primary-only persistence layer for all AI-native operations data, with no file fallback. This is the **target state** after Phase 0 completes. Until Phase 0 is finished:

- The backend README, deployment guide, and `main.go` still describe optional/best-effort MongoDB with JSONL fallback — that is the **current runtime behavior**.
- Phase 0 explicitly migrates away from JSONL and removes the hybrid mode for the listed collections.
- After Phase 0 ships, the backend README and deployment guide must be updated to reflect "MongoDB required" for AI features. This is tracked as a Phase 0 exit task.

### Phase 0 Migration Mechanics

The Phase 0 description lists *what* to migrate but not the operational details. Implementation must follow this contract:

| Aspect | Requirement |
|--------|-------------|
| **Source-to-target mapping** | `FileMySQLRepository` JSONL → `<prefix>_mysql_samples` + `<prefix>_mysql_deltas` collections; in-memory `IncidentRepository` → `<prefix>_incidents` collection; `FileNotificationOutbox` JSONL → `<prefix>_notification_outbox` collection; `FileAIQueue` JSONL → `<prefix>_ai_queue` collection. Collection prefix is set by `MONGODB_COLLECTION_PREFIX` (default `healthops`). |
| **Idempotency** | Migration script uses `BulkWrite` with `ReplaceOne` upserts (matching on deterministic `_id`). Re-running the script on a partially-migrated dataset must not create duplicates. Each document includes a deterministic `_id` derived from its source (e.g. SHA-256 of `checkId + timestamp` for samples). |
| **Rollback** | Before migration, the script creates a MongoDB backup via `mongodump --archive`. On failure, operator can restore with `mongorestore --archive` and redeploy the previous binary (which still uses `STORAGE_BACKEND=file`). File-based repos remain untouched on disk as a read-only archive. |
| **Backup** | The migration script refuses to run unless a fresh backup exists (checks `mongodump` output timestamp < 1 hour old or `--force` flag is passed). |
| **Cutover sequence** | 1. Take MongoDB backup (`mongodump --archive`). 2. Run migration script for historical data. 3. Run verification queries. 4. Deploy new code with `STORAGE_BACKEND=mongo` (Mongo-only reads and writes; file runtime writes removed in the same release). 5. Smoke-test `/healthz`, `/api/v1/checks`, `/api/v1/incidents`. File repos remain on disk as read-only archive but are not referenced at runtime. |
| **Verification queries** | `db.<prefix>_incidents.countDocuments()` matches file repo count ±0.1%. `db.<prefix>_mysql_samples.find().sort({timestamp:-1}).limit(1)` timestamp matches latest JSONL entry. Audit log replay: every `incident.created` / `incident.resolved` audit event has a corresponding MongoDB document with matching `status` and timestamps. |
| **Audit log prerequisite** | The exit metric "replaying the audit log" assumes the audit log fully captures incident lifecycle transitions (`created`, `acknowledged`, `resolved`). If audit events are missing for any transition today, Phase 0 must first backfill or fix the audit emission before relying on it for verification. |

### Phase 1 Incident Brief v1 — Evidence Sources

The roadmap says the Brief "pulls check results, logs, metrics, deployments, audit events, server data, MySQL data, and similar incidents." Several of those arrive in later phases. Brief v1 must be explicit about what it uses:

**Brief v1 evidence sources (available at Phase 1 delivery):**

- Check results (existing)
- MySQL samples and deltas (existing)
- Server metrics from SSH/process checks (existing)
- Audit events (existing)
- Incident history (existing)
- Alert rule context (existing)

**NOT available until later phases:**

- Logs / log fingerprints → Phase 2
- Deployment markers → Phase 3
- Similar incidents via vector/fingerprint search → Phase 4
- SLO burn data → Phase 5

**Degraded behavior contract:** When a later evidence type is unavailable, the Brief must:
1. Omit that evidence category silently (no "N/A" placeholder noise).
2. Reduce confidence score proportionally (each missing category reduces the max achievable confidence by its weight from the confidence formula).
3. Note in the Brief metadata which evidence categories were available vs total possible.

As each phase lands, the Brief automatically picks up new evidence types without requiring a Brief-specific code change (evidence categories are registry-driven, not hardcoded).

### RBAC and Context Retrieval Security (Phase 1 requirement)

Safety requirements mention user boundaries and evidence scoping but no phase owns them. **Phase 1 must include:**

1. **Context retrieval authorization**: Before the AI context builder assembles evidence for an incident, it must verify the requesting user/session has read access to that incident and all referenced checks/services. No cross-boundary data leakage through AI Briefs.
2. **Evidence scoping tests**: Integration tests that create two users with different service access, trigger a Brief for a shared incident, and verify each user's Brief only contains evidence from their authorized services.
3. **Prompt injection defense**: Evidence text (log messages, check output, audit messages) inserted into AI prompts must be enclosed in delimiters and the system prompt must instruct the model to treat evidence as data, not instructions. Test fixtures with adversarial evidence text must be part of the eval harness.

### Redaction Test Gates (Phase 2a enhancement)

The Phase 2a exit metric says "Zero raw secrets in `log_events` sample audit (100-event manual review)." This is insufficient as a primary gate. Add:

1. **Automated redaction fixture suite**: A test corpus of 500+ synthetic log lines with injected secrets (AWS keys, JWTs, DSNs, bearer tokens, emails, credit card numbers, private key headers, GitHub tokens). The redaction pipeline must detect and replace 100% of these in CI. Failures block merge.
2. **Canary injection test**: On every deploy, a synthetic log event with a known test secret is ingested. A background job queries `log_events` for the raw secret within 60s. If found, the pipeline is broken and alerting fires.
3. **The 100-event manual review** remains as a human-in-the-loop validation on first deploy and after major parser changes, but is not the primary safety gate.

## Recommended Implementation Sequence

Dependencies: Phase 0 → Phase 1 → (Phase 2a → Phase 2b) → Phase 3 → Phase 4 → Phase 5 → Phase 6. Phase 3 (deployments + heartbeats) can run in parallel with Phase 2 once Phase 1 lands.

### Phase 0: Storage Unification and Migration Backbone

Goal: commit to MongoDB-as-primary and migrate existing file-backed data so Phase 1 builds on solid ground.

Build:

1. [ADR 005](decisions/ADR-005-mongodb-primary-persistence.md): “MongoDB is the primary persistence layer for AI-native features.”
2. Migrate existing `FileMySQLRepository` (JSONL) data into a MongoDB `mysql_samples` / `mysql_deltas` collections. Keep the file repo as a one-time export source.
3. Migrate `IncidentRepository` (in-memory) to MongoDB `incidents` collection.
4. Migrate `FileNotificationOutbox` and `FileAIQueue` to MongoDB-backed implementations.
5. Mongo connection becomes a hard startup dependency for the AI features (with a clear health check and failure mode).
6. Index baselines from “Storage Decision” applied via migration script.

Exit metrics:

- 100% of new writes for the listed collections go to MongoDB.
- Migration script validated on a copy of production data; row counts match within 0.1%.
- Incidents created during the cutover window are not lost, verified by replaying the audit log and comparing expected incident IDs/transitions against MongoDB.
- Service refuses to enable AI features when MongoDB ping fails; `/healthz` reports degraded/unready instead of enabling file-backed writes.

### Phase 1: Evidence Backbone

Goal: create the foundation AI needs.

Build:

1. `SignalEvent` model and MongoDB repository (envelope from “Common Signal Schema”).
2. `IncidentEvent` model and timeline API.
3. Evidence linking from checks, MySQL samples, server metrics, audit events — all written as `SignalEvent`s with `incidentId` set.
4. `EvidenceProvider` registry: interface + registration mechanism so each phase can add new evidence categories (logs, deployments, SLO burn) without modifying the context builder. Phase 1 ships with providers for checks, MySQL, server metrics, audit, and incident history.
5. AI context builder that retrieves evidence by incident/time window via the registry, applies the evidence cap and summarization rollups.
6. AI Incident Brief v1, including deterministic confidence score and evidence citations.
7. AI eval harness: 10–20 fixture incidents with expected briefs; runs on every prompt change; gates merges.

Why first:

- The current app already has checks, incidents, MySQL, audit, AI config.
- This converts existing data into AI-ready context before adding large log ingestion.

Exit metrics:

- ≥ 95% of newly-opened incidents get an AI brief generated within 30s.
- Brief includes ≥ 3 evidence citations on average.
- Baseline alert-to-incident ratio captured before Phase 4 correlation changes.
- Operator “useful” rating ≥ 70% on `ai_feedback` over a 2-week window.
- Eval harness pass rate ≥ 90% on the fixture set.

### Phase 2a: Log Ingestion + Redaction + Storage

Goal: get logs into the system safely and cheaply, before any intelligence is layered on.

Build:

1. `LogSource` config (CRUD API + Mongo collection).
2. **OTLP logs receiver** (recommended primary path) — reuse OpenTelemetry Collector config patterns.
3. SSH file tailer (legacy/no-agent path).
4. Docker log tailer (legacy/no-agent path).
5. HTTP ingestion endpoint (simple webhook path).
6. Parser pipeline: source-specific parsers → normalize to `SignalEvent` envelope.
7. Redaction pipeline (allowlist model from “Redaction Model” above).
8. Backpressure: bounded in-memory queue per source; on overflow, drop oldest with a `telemetry_quality_findings` record. No unbounded buffering.
9. Redacted `log_events` written to Mongo with TTL (default 7d).

MVP constraints:

- Line-oriented logs only.
- No full-text search index in v1; queries are bounded by `service + time-window` only.

Exit metrics:

- Sustained ingest of 5k events/sec on a single node without queue overflow.
- 100% of events pass through redaction before persistence (verified by injection test).
- Zero raw secrets in `log_events` sample audit (100-event manual review).

### Phase 2b: Fingerprinting + Error Groups

Goal: turn redacted log events into a small number of meaningful signals.

Build:

1. Drain3-style fingerprinting on the ingest pipeline.
2. `log_fingerprints` aggregates with longer TTL (default 90d).
3. Per-service cardinality cap with coalescing.
4. Error group UI: list of fingerprints, trend sparkline, sample message, linked incidents.
5. Incident creation rule: fingerprint count exceeds baseline by N× within window → open incident or attach as evidence.

Exit metrics:

- Fingerprint cardinality per service ≤ 5,000 (cap enforced).
- ≥ 80% reduction in distinct error messages shown to operators vs source line count.
- Log-driven incidents have median time-to-first-evidence ≤ 60s.

### Phase 3: Deployment and Heartbeat Monitoring

Goal: solve two very common operational gaps. Can run in parallel with Phase 2 once Phase 1 lands.

Build:

1. Deployment marker API and `deployments` collection.
2. Deployment impact view (before/after on checks, MySQL, log fingerprints).
3. Heartbeat API and `heartbeats` collection.
4. Missed heartbeat incidents.
5. AI explanation for deploy regressions and missed jobs (uses Phase 1 brief pipeline).

Exit metrics:

- Deployment markers ingested via at least one CI integration (GitHub Actions example shipped).
- 100% of incidents starting within 15min of a deployment have a **temporal correlation** flag (explicitly labeled "temporal proximity, not confirmed cause"). The flag carries a confidence band based on evidence strength: `low` (time-only), `medium` (time + metric shift), `high` (time + metric shift + matching service/fingerprint). Only `medium`+ correlations appear in AI Briefs as contributing factors.
- Heartbeat misses raise an incident within `expectedInterval + grace` 100% of the time in tests.

### Phase 4: Correlation Engine

Goal: reduce alert fatigue. Implements the algorithm in “Correlation Algorithm” above.

Build:

1. Online correlation worker consuming `signal_events`.
2. `correlation_groups` collection with lifecycle.
3. Incident grouping using the deterministic-first rules.
4. Duplicate suppression on notifications (do not page twice for the same group).
5. Alert quality scoring per rule.
6. Similar incident search, starting with deterministic fingerprint/service/deployment matching; optional vector search can use MongoDB Atlas Search or a local embedding index later.

Exit metrics:

- Alert-to-incident ratio reduced by ≥ 60% on a 2-week sample (e.g. 3:1 → 1.2:1).
- ≤ 5% of grouped incidents flagged by operators as “wrongly grouped.”
- Notification suppression cuts duplicate pages by ≥ 70%.

### Phase 5: SLO and Reliability Reports

Goal: shift from check status to reliability management.

Build:

1. SLO definitions and `slo_definitions` / `slo_windows` collections.
2. Burn-rate windows (fast 1h, slow 6h) with multi-window multi-burn-rate alerting.
3. Reliability dashboard.
4. Weekly AI reliability report.
5. Confidence-score calibration job using `ai_feedback`.

**Implementation requirements:**

- SLI formulas: availability = `1 - (bad_events / total_events)` where bad = status ≥ 500 or timeout. Latency = `count(duration < threshold) / total_events`. Both computed per rolling window.
- Burn-rate math: fast-burn alert when `error_rate > (1 - SLO_target) * 14.4` over 1h AND 5m windows. Slow-burn alert when `error_rate > (1 - SLO_target) * 6` over 6h AND 30m windows. (Google SRE multi-window multi-burn-rate pattern.)
- Low-traffic behavior: services with < 100 events/window are excluded from percentage-based SLI (insufficient sample). They get availability measured by check success/failure only.
- Service inventory rules: a service is "active" if it has ≥ 1 check result or ≥ 1 signal event in the last 7 days. Inactive services are hidden from SLO dashboards but retained in definitions.

Exit metrics:

- At least one SLO defined for every service ingesting telemetry.
- Weekly report opened by ≥ 50% of operators within 24h of generation.
- Confidence score calibration error (Brier score) reduced vs Phase 1 baseline.

### Phase 6: Safe Remediation Assistant

Goal: reduce time-to-repair without unsafe automation.

Build:

1. Remediation action registry (`remediation_actions` collection) with explicit allowlist of safe actions per service.
2. Dry-run support for every action.
3. Approval workflow (human-in-the-loop, two-person for destructive actions).
4. Audit trail for AI-suggested and human-approved actions.
5. Rollback/runbook integration.

**Implementation requirements:**

- Approval state machine: `proposed` → `dry_run_requested` → `dry_run_complete` → `approval_requested` → `approved` (by different user than proposer for destructive) → `executing` → `completed` | `failed` → `rolled_back` (optional). Any state can transition to `cancelled`.
- Two-person enforcement: actions tagged `destructive: true` require approval from a user different than the one who triggered or proposed the action. The system rejects self-approval for destructive actions.
- Dry-run semantics: dry-run executes the action in a sandboxed mode (read-only DB transaction, `--dry-run` CLI flag, or API simulation endpoint depending on action type). Dry-run output is stored in the action record and shown to the approver.
- MTTR baseline cohort: measure MTTR for incidents that had a remediation action available vs incidents that did not, over a 30-day rolling window. Report the delta in the weekly reliability report.
- Rollback: every approved action must have a `rollback_command` or `rollback_runbook_url` before execution. The system blocks execution if rollback is empty unless `--no-rollback-override` is explicitly passed with a reason.

Exit metrics:

- 0 destructive actions executed without explicit human approval (audit-verified).
- Median time-to-mitigation reduced for incidents where a remediation action existed.
- Every remediation execution has a linked rollback plan.

## Feature Prioritization Matrix

| Feature | User value | Build effort | Risk | Priority |
|---|---:|---:|---:|---:|
| Phase 0: storage unification on MongoDB | High | Medium | Medium | P0 |
| Evidence backbone + AI Incident Brief | Very high | Medium | Medium | P0 |
| Log ingestion + redaction (Phase 2a) | Very high | Medium | Medium | P0 |
| OTLP logs receiver | Very high | Medium | Low | P0 |
| Log fingerprinting + error groups (Phase 2b) | Very high | Medium | Medium | P1 |
| Deployment markers | High | Low | Low | P1 |
| Heartbeat monitoring | High | Low | Low | P1 |
| Incident correlation engine | Very high | Medium | Medium | P1 |
| AI postmortem drafts | High | Medium | Medium | P2 |
| SLO/error budget | High | Medium | Low | P2 |
| Telemetry cost controls | High | Medium | Low | P2 |
| Safe remediation assistant | High | High | High | P3 |
| AI workload monitoring | Medium | Medium | Medium | P3 |

## First Build Candidate

The best first product slice is:

```text
Phase 0 storage unification
  + Phase 1 AI Incident Brief + Evidence Timeline
  + Phase 2a log ingestion via OTLP receiver (with redaction)
```

This gives users immediate value:

- All data lives in one place (MongoDB) and is ready for AI context.
- Incidents become explainable with cited evidence.
- Logs flow in via the standard OTel path — no bespoke agent required for new users.
- The architecture becomes ready for fingerprinting, deployment markers, heartbeats, correlation, and SLOs.

Minimum implementation checklist:

- ADR-005 written and accepted.
- Mongo migration scripts for existing file-backed collections.
- `SignalEvent` repository and API (Mongo).
- `LogSource` repository and API (Mongo).
- `LogEvent` repository (Mongo, TTL index).
- OTLP logs receiver wired to the ingestion pipeline.
- Redaction utility (allowlist, with injection tests).
- Background log tailer worker for SSH/Docker (legacy paths).
- Incident evidence API.
- AI context builder with evidence cap and summarization rollups.
- AI Incident Brief endpoint with deterministic confidence score.
- AI eval harness with 10–20 fixture incidents gating prompt changes.
- Incident detail UI: Evidence tab and AI Brief tab.
- Logs UI: Live Tail (fingerprinting and error groups land in Phase 2b).

## Safety Requirements

AI-native operations can become dangerous if the system overstates confidence. These safeguards are mandatory:

- Every AI conclusion must include evidence links.
- Every AI result must expose confidence and uncertainty.
- AI must distinguish observed facts from hypotheses.
- Destructive remediation requires human approval.
- Secrets must be redacted before storage and before AI calls.
- User boundaries must be enforced before context retrieval; tenant boundaries must be enforced before enabling SaaS or multi-tenant mode.
- Prompt templates must be versioned.
- AI feedback must be tracked so bad analyses can be improved.
- Postmortem root cause remains human-approved.

## Product North Star

HealthOps should answer this question better than a dashboard:

> What broke, why is it likely broken, what evidence proves that, what changed recently, who is impacted, and what should we do next?

If every feature makes that answer faster, clearer, and safer, the application becomes genuinely AI-native instead of just AI-decorated.
