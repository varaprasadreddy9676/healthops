# AI Feature Roadmap

## Phase 0: Data Foundation
- Build the non-AI monitoring core first: checks, logs, metrics, incidents, and server inventory.
- Keep MongoDB as a best-effort mirror and preserve local file fallback so the app stays up if MongoDB is down.
- Normalize signals into one event shape: `server`, `service`, `timestamp`, `severity`, `category`, `message`, `raw_payload`.
- Add log ingestion with redaction for secrets, tokens, passwords, and connection strings.
- Show raw health, recent logs, and recent incidents before introducing AI.
- Make AI optional from day one so the product remains useful with no provider key configured.

## Phase 1: AI Incident Summaries
- Trigger AI only when a rule-based alert fires.
- Generate a short incident summary from the last few minutes of context.
- Show what failed, when it started, which servers are affected, and which signals changed first.
- Add an AI summary card on incident pages and server detail pages.
- If no AI key is configured, skip summaries and show the plain incident timeline instead.

## Phase 2: Log Intelligence
- Cluster repeated errors and stack traces into one incident family.
- Detect when many log lines are really the same underlying failure.
- Add AI-assisted labels such as DB auth, timeout, thread exhaustion, slow query, network, or app bug.
- Show grouped error buckets, recurring exception families, and similar incidents in the UI.

## Phase 3: Correlated Root-Cause Analysis
- Correlate CPU, memory, IOPS, thread growth, request latency, and error spikes.
- Ask AI for a ranked hypothesis list with confidence, not a single verdict.
- Add an incident timeline and a "why this alert happened" panel.
- Package multi-signal context before the AI call so the model sees the full picture.

## Phase 4: Natural-Language Ops Assistant
- Let users ask questions like "why is prod slow?" or "show SQL auth failures today."
- Answer from telemetry, not model memory.
- Support queries over checks, logs, incidents, and server groups.
- Add a search bar and chat-like investigation panel.
- Hide this assistant entirely when no AI provider key is configured.

## Phase 5: Tuning and Recommendations
- Suggest threshold changes based on recent behavior.
- Recommend checks for services that need log monitoring, heartbeat checks, or synthetic transactions.
- Warn when a server is "up but stuck" based on missing logs, stalled work, or frozen request flow.
- Keep humans in control of config changes.
- Only surface recommendations when AI is enabled; otherwise keep the plain rule-based workflow.

## Phase 6: Assisted Automation
- Suggest actions like restart, drain node, rotate credentials, or inspect slow queries.
- Do not auto-execute by default.
- Require explicit human approval for any remediation.
- Log every AI recommendation and every human approval for auditability.
- Keep automation disabled unless AI is enabled and the tenant has explicitly opted in.

## BYOK AI Mode
- AI must be a bring-your-own-key feature, not a hard dependency.
- Support per-tenant or per-environment provider keys.
- Store provider settings separately from monitoring config.
- If the key is missing, invalid, or revoked, the system must degrade to plain monitoring without breaking alerts or dashboards.
- Add a connection test and a visible "AI off" state in the UI.

## What AI Should Not Own
- Liveness checks
- CPU and memory thresholds
- Port checks
- Exact alert firing
- Fail-safe application state

## Practical Order
- Start with Phases 0 to 2.
- Add Phase 3 once enough telemetry exists.
- Add Phases 4 and 5 after the data model and dashboards are stable.
- Keep Phase 6 optional and tightly controlled.
