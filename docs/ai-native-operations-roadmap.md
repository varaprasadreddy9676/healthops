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
- OpenTelemetry logs later

Pipeline:

```text
Ingest
→ Parse
→ Normalize
→ Redact secrets
→ Enrich with service/host/env/deployment
→ Fingerprint
→ Sample/drop according to policy
→ Store raw event short-term
→ Store fingerprint aggregates longer-term
→ Correlate to incidents
```

Core capabilities:

- Error fingerprinting by stack trace, message template, SQL code, exception type.
- Spike detection per fingerprint.
- Noise suppression for repeated identical events.
- Secret redaction before storage and AI analysis.
- Retention tiers: raw logs short-term, aggregates longer-term.

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
  future OTLP ingest

Normalization Layer
  parse
  redact
  enrich
  apply semantic fields
  fingerprint
  assign service/env/host/deployment

Signal Store
  raw events
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

```text
log_sources
log_events
log_fingerprints
signal_events
correlation_groups
incident_events
deployments
heartbeats
slo_definitions
slo_windows
ai_investigations
ai_feedback
runbooks
remediation_actions
telemetry_quality_findings
```

### Common Signal Schema

Every signal should fit a common envelope:

```json
{
  "id": "sig_...",
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
  "message": "normalized message",
  "attributes": {},
  "redactionStatus": "clean|redacted|blocked"
}
```

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

## Recommended Implementation Sequence

### Phase 1: Evidence Backbone

Goal: create the foundation AI needs.

Build:

1. `SignalEvent` model and repository.
2. `IncidentEvent` model and timeline API.
3. Evidence linking from checks, MySQL samples, server metrics, audit events.
4. AI context builder that retrieves evidence by incident/time window.
5. AI Incident Brief v1.

Why first:

- The current app already has checks, incidents, MySQL, audit, AI config.
- This converts existing data into AI-ready context before adding large log ingestion.

### Phase 2: Log Intelligence MVP

Goal: continuous log monitoring without uncontrolled cost.

Build:

1. `LogSource` config.
2. SSH file tailer.
3. Docker log tailer.
4. HTTP ingestion endpoint.
5. Parser/redactor/fingerprinter.
6. Error group UI.
7. Incident correlation from log spikes.

MVP constraints:

- Start with line-oriented logs.
- Store raw logs for short retention only.
- Store fingerprints and aggregate counts longer-term.
- Redact before persistence and before AI.

### Phase 3: Deployment and Heartbeat Monitoring

Goal: solve two very common operational gaps.

Build:

1. Deployment marker API.
2. Deployment impact view.
3. Heartbeat API.
4. Missed heartbeat incidents.
5. AI explanation for deploy regressions and missed jobs.

### Phase 4: Correlation Engine

Goal: reduce alert fatigue.

Build:

1. Correlation rules.
2. Incident grouping.
3. Duplicate suppression.
4. Alert quality scoring.
5. Similar incident search.

### Phase 5: SLO and Reliability Reports

Goal: shift from check status to reliability management.

Build:

1. SLO definitions.
2. Burn-rate windows.
3. Reliability dashboard.
4. Weekly AI reliability report.

### Phase 6: Safe Remediation Assistant

Goal: reduce time-to-repair without unsafe automation.

Build:

1. Remediation action registry.
2. Dry-run support.
3. Approval workflow.
4. Audit trail for AI-suggested and human-approved actions.
5. Rollback/runbook integration.

## Feature Prioritization Matrix

| Feature | User value | Build effort | Risk | Priority |
|---|---:|---:|---:|---:|
| Evidence backbone + AI Incident Brief | Very high | Medium | Medium | P0 |
| Log Intelligence MVP | Very high | High | Medium | P0 |
| Deployment markers | High | Low | Low | P1 |
| Heartbeat monitoring | High | Low | Low | P1 |
| Incident correlation engine | Very high | Medium | Medium | P1 |
| Error fingerprinting | Very high | Medium | Medium | P1 |
| AI postmortem drafts | High | Medium | Medium | P2 |
| SLO/error budget | High | Medium | Low | P2 |
| Telemetry cost controls | High | Medium | Low | P2 |
| Safe remediation assistant | High | High | High | P3 |
| OpenTelemetry/OTLP ingest | High | High | Medium | P3 |
| AI workload monitoring | Medium | Medium | Medium | P3 |

## First Build Candidate

The best first product slice is:

```text
AI Incident Brief v1 + Evidence Timeline + Log Fingerprint MVP
```

This gives users immediate value:

- Incidents become explainable.
- Logs become grouped instead of noisy.
- AI has evidence instead of generic guesses.
- The architecture becomes ready for deployment markers, heartbeats, and SLOs.

Minimum implementation checklist:

- `SignalEvent` repository and API.
- `LogSource` repository and API.
- `LogEvent` and `LogFingerprint` repository.
- Background log tailer worker.
- Redaction utility.
- Fingerprint utility.
- Incident evidence API.
- AI context builder.
- Incident detail UI: Evidence tab and AI Brief tab.
- Logs UI: Error Groups and Live Tail.

## Safety Requirements

AI-native operations can become dangerous if the system overstates confidence. These safeguards are mandatory:

- Every AI conclusion must include evidence links.
- Every AI result must expose confidence and uncertainty.
- AI must distinguish observed facts from hypotheses.
- Destructive remediation requires human approval.
- Secrets must be redacted before storage and before AI calls.
- Tenant/user boundaries must be enforced before context retrieval.
- Prompt templates must be versioned.
- AI feedback must be tracked so bad analyses can be improved.
- Postmortem root cause remains human-approved.

## Product North Star

HealthOps should answer this question better than a dashboard:

> What broke, why is it likely broken, what evidence proves that, what changed recently, who is impacted, and what should we do next?

If every feature makes that answer faster, clearer, and safer, the application becomes genuinely AI-native instead of just AI-decorated.
