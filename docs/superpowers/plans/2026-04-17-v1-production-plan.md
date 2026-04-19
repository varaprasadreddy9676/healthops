# Medics Health Check V1 Production Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the current Go backend prototype into a production-grade single-tenant monitoring product with strong backend reliability, core operator workflows, and a minimal but usable UI.

**Architecture:** Keep the Go backend as the only backend runtime. Replace the current blob-style state model with a clearer domain model for check definitions, latest status, run history, incidents, alerts, and audit records. Build a minimal frontend against stable v1 APIs instead of continuing prototype-only backend work.

**Tech Stack:** Go 1.22 backend, MongoDB for durable persistence, local fallback storage only if explicitly kept, HTTP JSON APIs, React/Next.js frontend (to be selected during implementation), Prometheus metrics, structured logs, email and webhook alerting.

---

## 1. Current Repo State

This is the starting point as of plan creation.

### Backend exists today
- Go service entrypoint: `backend/cmd/healthops/main.go`
- Core package: `backend/internal/monitoring/`
- Supported check types: `api`, `tcp`, `process`, `command`, `log`
- HTTP endpoints for health, check CRUD, manual run, summary, results, dashboard aliases
- File-backed state store with optional best-effort Mongo mirror
- Good unit test base for current code

### Major limitations today
- No authentication or authorization on write APIs
- No incident model
- No alert engine or delivery channels
- No audit trail
- No stable storage model for production scale
- No proper frontend
- Current persistence rewrites large mutable snapshots and syncs full state
- Repo still contains legacy artifacts that should not remain if this is truly a new Go-only product

### Product decision for this plan
This plan assumes:
- This is a new product, not a migration project
- Legacy Node startup/config artifacts should be removed
- Go backend is the source of truth
- Single-tenant internal operations tool is the v1 target

---

## 2. V1 Product Definition

V1 is complete only when all of the following are true.

### Required product capabilities
- Operators can create, edit, disable, duplicate, and delete checks safely
- Checks run automatically on schedules and can be manually triggered
- Latest status and recent run history are stored durably
- Operators can receive alerts through at least email and webhook
- Operators can view, acknowledge, and resolve incidents
- All mutating actions are authenticated
- Dashboard and detail pages exist for daily operational use
- Backend exports its own metrics and logs clearly enough to operate and debug itself
- Test suite and release checklist are strong enough that another engineer can ship changes safely

### Explicitly out of scope for v1
- AI features
- Multi-tenant SaaS support
- Automated remediation
- Sophisticated analytics beyond health trends and incident history
- Broad plugin ecosystem

---

## 3. Delivery Principles

Use these rules throughout execution.

### General rules
- Keep changes incremental and releasable
- Do not add features on top of the current state blob design unless required for temporary compatibility
- Prefer stable API contracts over fast endpoint proliferation
- Protect mutating actions before adding more write features
- Add tests before or alongside every behavioral change

### Testing rules
- Every backend feature must ship with unit tests
- Every domain workflow must ship with integration tests
- Critical operator flows must be covered end-to-end before release
- Run `go test ./...` and `go test -race ./...` before claiming backend completion
- Frontend changes must be smoke-tested against real backend responses

### Git rules
- Small commits
- One focused concern per commit when practical
- Keep plan progress visible in commit messages and PR descriptions

---

## 4. Recommended Delivery Order

Follow these phases in order.

1. Phase 0: Repo cleanup and v1 scope lock
2. Phase 1: Domain and storage redesign
3. Phase 2: Security and API hardening
4. Phase 3: Scheduler and check execution hardening
5. Phase 4: Alerts and incidents
6. Phase 5: Frontend v1
7. Phase 6: Operability and release hardening
8. Phase 7: Final verification and launch readiness

Do not start frontend work before the backend domain model and API contracts are stable enough to consume.

---

## 5. Target File Structure

This is the intended direction, not necessarily the exact final structure.

### Backend
- `backend/cmd/healthops/main.go`: bootstrap, env loading, dependency wiring
- `backend/internal/monitoring/config.go`: config schema, validation, defaults
- `backend/internal/monitoring/types.go`: shared domain types only if still justified after refactor
- `backend/internal/monitoring/checks/`: check definitions, validation, execution helpers
- `backend/internal/monitoring/store/`: persistence interfaces and implementations
- `backend/internal/monitoring/readmodel/`: latest status and dashboard projections
- `backend/internal/monitoring/incidents/`: incident domain model and services
- `backend/internal/monitoring/alerts/`: rules, routing, cooldowns, delivery logs
- `backend/internal/monitoring/auth/`: auth middleware and token validation
- `backend/internal/monitoring/audit/`: audit record creation and retrieval
- `backend/internal/monitoring/http/`: handlers, request/response models, middleware
- `backend/internal/monitoring/metrics/`: Prometheus instrumentation

### Frontend
- `frontend/` becomes actual app
- dashboard page
- checks list and form pages
- incidents list and detail pages
- alert channel settings page

### Docs
- `ReadMe.md`: real quick-start and architecture summary
- `docs/`: ADRs, release checklist, runbook, API notes, plan files

---

## 6. Phase 0: Repo Cleanup And Scope Lock

### Task 0.1: Remove legacy artifacts and false startup paths

**Objective**
Make the repository clearly Go-only so future engineers do not follow dead paths.

**Files**
- Delete or archive: `start-health-check-engine.sh`
- Delete or archive: `config/config.json`
- Update: `ReadMe.md`
- Update: `CLAUDE.md`
- Update: `AGENTS.md` only if repo instructions now misstate reality
- Update: `backend/cmd/healthops/main.go`

**Implementation**
- Remove root startup script that points to missing Node runtime
- Remove legacy config if not needed
- Change default config resolution so only Go config paths are used
- Rewrite docs to describe only current Go product

**Tests after implementation**
- Start backend with no custom env vars: `cd backend && go run ./cmd/healthops`
- Verify config loads from intended path only
- Verify docs mention only supported startup flow
- Verify no repo file references deleted Node startup path
  - Suggested command: `rg -n "start-health-check-engine|config/config.json|legacy Node" .`

**Done criteria**
- No dead startup script remains
- No dead config path remains in runtime code
- README and internal docs match repo reality

### Task 0.2: Write v1 ADRs before deep refactor

**Objective**
Freeze key decisions so implementation stays coherent.

**Create**
- `docs/decisions/ADR-001-v1-scope.md`
- `docs/decisions/ADR-002-primary-persistence-model.md`
- `docs/decisions/ADR-003-auth-strategy.md`
- `docs/decisions/ADR-004-incident-and-alert-domain.md`

**Implementation**
Document:
- single-tenant scope
- primary persistence choice
- auth model
- incident and alert lifecycle

**Tests after implementation**
- Human review only
- Ensure each ADR lists alternatives and consequences
- Ensure each ADR is referenced by later implementation docs

**Done criteria**
- Core product decisions documented before code divergence grows

---

## 7. Phase 1: Domain And Storage Redesign

### Task 1.1: Replace blob `State` model with explicit domain records

**Objective**
Break apart the current `State{Checks, Results, LastRunAt, UpdatedAt}` model into production-safe records.

**Current code to replace or reduce dependency on**
- `backend/internal/monitoring/types.go`
- `backend/internal/monitoring/store.go`
- `backend/internal/monitoring/hybrid_store.go`
- `backend/internal/monitoring/mongo.go`

**Target records**
- `CheckDefinition`
- `CheckRun`
- `CheckLatestStatus`
- `Incident`
- `AlertDelivery`
- `AuditEvent`

**Implementation**
- Introduce explicit types and repository interfaces for each record category
- Remove assumption that the whole system state must be loaded and rewritten together
- Keep read models separate from write records

**Tests after implementation**
- Unit tests for each repository interface contract
- Persistence tests for create, update, list, delete, and filtered queries
- Restart test: write records, restart service, verify data remains correct
- Data growth test: large historical runs do not require full-state rewrite

**Done criteria**
- Backend no longer depends on one mutable state blob for normal operations

### Task 1.2: Choose and implement primary persistence model

**Objective**
Move from prototype storage to a production-safe primary data model.

**Decision to make**
Choose one of:
- MongoDB as primary store
- Local file store for dev only, MongoDB for prod

For v1 production, recommended choice is:
- MongoDB primary in production
- Optional local file store only for local development and tests

**Implementation**
- Create clean repository interfaces
- Implement Mongo repositories for all v1 domain records
- If file store remains, scope it to development mode only
- Remove full-state sync mirror pattern

**Tests after implementation**
- Repository integration tests against Mongo test instance
- Query performance smoke test on latest-status and recent-run queries
- Failure test when Mongo unavailable at startup
- Reconnect behavior if Mongo drops and returns

**Done criteria**
- Production path uses one clear primary persistence strategy
- No best-effort full-state mirror dependency remains for core correctness

### Task 1.3: Build latest-status and dashboard read model

**Objective**
Serve dashboard queries from explicit current-state projections instead of recomputing from raw history on every request.

**Implementation**
- Maintain latest status per check after each run
- Maintain summary projections by application, server, and tag
- Maintain recent failing checks and open incident counts

**Tests after implementation**
- Unit tests for projection updates on healthy, warning, critical transitions
- Verify repeated runs update latest status correctly
- Verify summary counts stay correct after check deletion or disable
- Verify dashboard endpoints do not require scanning full run history

**Done criteria**
- Dashboard can render from read model without full historical recomputation

### Task 1.4: Add data retention and pagination model

**Objective**
Make historical data bounded and queryable.

**Implementation**
- Retain latest status indefinitely unless check deleted
- Retain run history for configurable days
- Add pagination and filtering on historical endpoints
- Add cleanup job if storage engine needs explicit pruning

**Tests after implementation**
- Verify old runs prune correctly
- Verify latest status survives run-history cleanup
- Verify paginated results stable and ordered
- Verify large history queries do not return unbounded payloads

**Done criteria**
- Historical queries are bounded, ordered, and configurable

---

## 8. Phase 2: Security And API Hardening

### Task 2.1: Add authentication middleware for mutating endpoints

**Objective**
Protect the product before adding more write features.

**Scope**
Protect at minimum:
- create check
- update check
- delete check
- manual run trigger
- incident acknowledge/resolve
- alert channel configuration

**Recommended v1 auth**
- Static API key or signed service token from env/config for internal tool use

**Implementation**
- Build middleware package
- Define authenticated actor model
- Attach actor identity to audit events
- Return proper 401/403 responses

**Tests after implementation**
- Unit tests for valid token, invalid token, missing token
- API integration tests for protected endpoints
- Verify read-only endpoints behave as intended
- Verify audit records include actor identity

**Done criteria**
- No mutating endpoint is callable anonymously

### Task 2.2: Harden request validation and config validation

**Objective**
Reject malformed or risky inputs early.

**Implementation**
Add validation for:
- ID length and character set
- name length
- tag count and tag length
- allowed check types
- timeout upper bounds
- interval lower and upper bounds
- command checks gated by config
- HTTP URL validation
- regex compilation if regex checks added
- owner/contact fields if introduced

**Tests after implementation**
- Table-driven validation tests
- API tests for malformed JSON
- API tests for overlong fields
- API tests for invalid combinations such as `api` type with missing URL

**Done criteria**
- Invalid config and malformed requests fail with clear 4xx errors

### Task 2.3: Stabilize v1 API contracts

**Objective**
Expose a clean backend contract before frontend implementation.

**Target endpoints**
- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `GET /api/v1/checks`
- `POST /api/v1/checks`
- `PUT /api/v1/checks/{id}`
- `DELETE /api/v1/checks/{id}`
- `POST /api/v1/checks/{id}/test`
- `POST /api/v1/runs`
- `GET /api/v1/status`
- `GET /api/v1/checks/{id}/status`
- `GET /api/v1/checks/{id}/runs`
- `GET /api/v1/incidents`
- `POST /api/v1/incidents/{id}/acknowledge`
- `POST /api/v1/incidents/{id}/resolve`
- `GET /api/v1/alerts/deliveries`
- `GET /api/v1/audit`

**Implementation**
- Normalize request and response models
- Add pagination and filters where required
- Add API docs in markdown or OpenAPI form

**Tests after implementation**
- Handler unit tests
- Integration tests with realistic payloads
- Response shape assertions for frontend-critical endpoints
- Contract review against UI needs before frontend work begins

**Done criteria**
- API surface stable enough that frontend implementation can begin without churn

---

## 9. Phase 3: Scheduler And Check Execution Hardening

### Task 3.1: Extend check definitions for real operator use

**Objective**
Support the most useful core monitoring workflows in v1.

**HTTP/API check additions**
- method
- headers
- optional request body
- expected status
- expected substring or regex
- timeout
- warning and critical latency thresholds
- optional redirect behavior
- optional TLS verification controls, explicit and limited

**Other check additions**
- TCP connect check
- process match mode: substring and regex
- command check with stdout/stderr capture caps
- log freshness check with optional pattern mode
- DNS resolve check
- TLS certificate expiry check

**Tests after implementation**
- Unit tests per check type
- Integration tests for each real execution path
- Timeout and cancellation tests
- Failure classification tests
- Metrics population tests

**Done criteria**
- Check model covers most practical internal ops monitoring needs

### Task 3.2: Per-check scheduling, retries, and cooldowns

**Objective**
Make execution policy realistic and predictable.

**Implementation**
Add per-check fields:
- interval
- timeout
- retry count
- retry backoff
- cooldown
- maintenance window or disable-until timestamp

**Tests after implementation**
- Verify per-check interval scheduling works
- Verify retries happen only for eligible failure types if such logic exists
- Verify maintenance window suppresses execution and alerts
- Verify cooldown suppresses duplicate alerts but not state updates
- Verify overlapping runs are prevented per check

**Done criteria**
- Scheduler behavior is controlled per check, not only globally

### Task 3.3: Failure classification and richer result records

**Objective**
Store enough detail for operators and incidents.

**Implementation**
Add to run record where useful:
- status
- failure class or reason code
- user-safe message
- latency
- selected metrics
- truncated raw output or response sample
- run ID
- check version or config hash

**Tests after implementation**
- Verify failures classify consistently
- Verify huge command output gets truncated safely
- Verify sensitive headers or secrets do not leak into stored payloads
- Verify result records are enough to render incident timeline

**Done criteria**
- Run history has operator-usable context without uncontrolled payload growth

### Task 3.4: Add manual test-run path for one check

**Objective**
Allow safe validation before saving or enabling checks.

**Implementation**
- Add `POST /api/v1/checks/{id}/test`
- Optionally support dry-run validation for unsaved form payloads later
- Separate test-run records from scheduled production runs if needed

**Tests after implementation**
- Verify test run executes only target check
- Verify test run does not mutate normal incident state unless explicitly intended
- Verify UI can display result immediately

**Done criteria**
- Operators can validate a check without waiting for scheduler cadence

---

## 10. Phase 4: Alerts And Incidents

### Task 4.1: Build alert rule engine v1

**Objective**
Turn health changes into actionable notifications.

**Minimum v1 rule behavior**
- Alert on warning and critical based on per-check policy
- Deduplicate repeated failures
- Cooldown to avoid spam
- Recovery notification when healthy again

**Channels for v1**
- email
- webhook

**Implementation**
- Add rule evaluation after each check or run batch
- Persist delivery attempts and outcomes
- Make channels configurable and testable

**Tests after implementation**
- Unit tests for dedupe, cooldown, recovery logic
- Delivery tests with mocked email and webhook providers
- Integration test: failure opens alert delivery, repeat failure does not spam, recovery sends resolution alert

**Done criteria**
- System can notify operators reliably and avoid duplicate noise

### Task 4.2: Build incident domain model v1

**Objective**
Represent ongoing failures as incidents instead of only raw results.

**Minimum fields**
- ID
- title or derived summary
- status: `open`, `acknowledged`, `resolved`
- openedAt
- acknowledgedAt
- resolvedAt
- impacted check IDs
- last observed status
- latest message
- notes

**Implementation**
- Open incident when alert-worthy failure begins
- Update incident while failure persists
- Resolve incident automatically on healthy recovery if policy allows
- Support manual acknowledgement and manual resolution notes

**Tests after implementation**
- Incident opens on first qualifying failure
- Repeated failures update same incident instead of opening duplicates
- Recovery resolves incident correctly
- Manual acknowledge changes state and records actor
- Incident listing returns proper filters and ordering

**Done criteria**
- Operators can track active failures as incidents with clear lifecycle

### Task 4.3: Add audit trail for operational actions

**Objective**
Make the system accountable and debuggable.

**Record at minimum**
- check create/update/delete
- manual run trigger
- manual test run
- incident acknowledge/resolve
- alert channel create/update/test
- auth failures if policy wants them audited

**Tests after implementation**
- Verify each action emits an audit event
- Verify actor identity and timestamp stored correctly
- Verify audit listing filters work

**Done criteria**
- Sensitive operational actions are traceable

---

## 11. Phase 5: Frontend V1

### Task 5.1: Select frontend stack and scaffold app

**Objective**
Turn `frontend/` from placeholder into actual operator UI.

**Recommended v1 direction**
- React or Next.js
- Keep it simple
- Avoid premature design-system complexity

**Implementation**
- Create real app shell
- Configure API client
- Configure environment handling
- Add route structure

**Tests after implementation**
- App boots locally
- API base URL configurable
- Smoke test landing/dashboard route

**Done criteria**
- Frontend runs locally against backend

### Task 5.2: Dashboard page

**Objective**
Provide default operator landing surface.

**Features**
- overall summary cards
- checks table with status, app, server, tag, last run, duration
- quick filters
- recent incidents panel
- recent failures panel or trend view

**Tests after implementation**
- Backend-driven smoke test with realistic data
- Empty-state rendering test
- Loading and error state test
- Mobile-width smoke test if responsive target is included

**Done criteria**
- Operator can understand current system health from one page

### Task 5.3: Checks management UI

**Objective**
Allow day-to-day configuration from UI.

**Features**
- list checks
- create check form
- edit check form
- enable/disable
- duplicate check
- delete check with confirmation
- test run button

**Tests after implementation**
- Form validation tests
- Create/edit/delete flow smoke tests
- Verify disabled checks display clearly
- Verify test-run result is visible to user

**Done criteria**
- Operator can manage checks without editing config files

### Task 5.4: Check detail page

**Objective**
Support investigation for one check.

**Features**
- current status
- recent run history
- latency trend if available
- linked incidents
- full definition view

**Tests after implementation**
- Check detail loads from real API
- Empty history and recent-failure states render correctly
- Incident links work

**Done criteria**
- Operator can debug one failing check without querying raw APIs manually

### Task 5.5: Incidents UI

**Objective**
Give operators a workflow for active issues.

**Features**
- incident list with filters
- incident detail
- acknowledge action
- resolve action with note
- affected checks view

**Tests after implementation**
- Incident listing with open/ack/resolved filters
- Ack/resolve action smoke tests
- Optimistic or refetch behavior works after status changes

**Done criteria**
- Operator can manage active incidents from UI

### Task 5.6: Alert channels UI

**Objective**
Make notification setup operable.

**Features**
- channel list
- create/edit email channel
- create/edit webhook channel
- test-send action
- recent delivery status view if time permits

**Tests after implementation**
- Channel validation tests
- Test-send smoke test against mocked backend response
- Error display on failed delivery

**Done criteria**
- Operator can configure and test alert delivery without manual DB edits

---

## 12. Phase 6: Operability And Release Hardening

### Task 6.1: Add Prometheus metrics and structured logs

**Objective**
Make the monitoring service observable as a service.

**Metrics to add**
- check runs total
- check failures total by type/status
- alert deliveries total by channel/result
- incident opens total
- scheduler lag
- check execution duration histogram
- backend request duration histogram

**Tests after implementation**
- Verify `/metrics` exposes expected counters and histograms
- Verify labels are bounded and sane
- Verify logs include request ID or run ID and check ID where relevant

**Done criteria**
- Service can be operated and debugged with standard telemetry

### Task 6.2: Add CI pipeline

**Objective**
Make regressions hard to merge.

**Minimum CI jobs**
- backend unit tests
- backend race tests
- backend lint or static analysis
- frontend build
- frontend tests if present

**Tests after implementation**
- Open PR or simulate CI locally where possible
- Ensure failing test blocks pipeline
- Ensure docs mention required checks

**Done criteria**
- Every major change path is guarded automatically

### Task 6.3: Add deployment assets

**Objective**
Make it possible for another engineer to run and ship the product.

**Create**
- Dockerfile for backend
- frontend build/deploy guidance
- `.env.example`
- optional `docker-compose.yml` for local full stack with Mongo

**Tests after implementation**
- Fresh clone local run using documented steps
- Container build succeeds
- Full stack boots with local compose if provided

**Done criteria**
- Another engineer can stand up the product using repo docs alone

### Task 6.4: Write runbook and release checklist

**Objective**
Ensure operational handoff is possible.

**Create**
- `docs/runbook.md`
- `docs/release-checklist.md`
- `docs/backup-and-restore.md`

**Must cover**
- startup
- env vars
- auth setup
- alert channel setup
- backup/restore
- retention policy
- troubleshooting failed checks
- troubleshooting failed alert deliveries
- release verification checklist

**Tests after implementation**
- Human dry run using docs only
- Verify every documented command actually works

**Done criteria**
- Docs are sufficient for handoff and on-call use

---

## 13. Phase 7: Final Verification And Launch Readiness

This phase is mandatory before calling v1 done.

### Task 7.1: Backend verification suite

**Run all backend verification**
- `cd backend && go test ./...`
- `cd backend && go test -race ./...`
- lint/static analysis command chosen by implementation team

**Pass criteria**
- all tests pass
- no race failures
- no ignored critical lint issues

### Task 7.2: Persistence and recovery verification

**Verify**
- start service with empty database
- create checks
- run checks
- restart service
- verify checks, latest status, incidents, and alert history still present as expected

**Pass criteria**
- no data corruption
- no required manual repair after restart

### Task 7.3: Security verification

**Verify**
- all mutating endpoints reject missing auth
- invalid auth rejected
- valid auth accepted
- sensitive secrets not returned in APIs or logs
- command checks disabled unless explicitly allowed

**Pass criteria**
- no write path accessible anonymously
- no obvious secret exposure in normal logs or responses

### Task 7.4: Alert and incident verification

**Verify**
- one warning incident and one critical incident flow end-to-end
- dedupe prevents notification spam
- acknowledge and resolve flow works from API and UI
- recovery notification works

**Pass criteria**
- operator lifecycle complete for common failure path

### Task 7.5: Frontend smoke verification

**Verify**
- dashboard loads
- checks CRUD works
- manual test run works
- incident list/detail works
- alert channel config and test-send works

**Pass criteria**
- common operator workflows work without using raw API tools

### Task 7.6: Performance and scale smoke verification

**Suggested v1 target**
- 50 to 200 checks
- normal polling intervals
- recent run history queries under acceptable latency for internal use

**Verify**
- scheduler keeps up under target check load
- dashboard remains usable
- historical API queries are paginated and responsive

**Pass criteria**
- no severe latency or correctness failure under target scale

---

## 14. End-To-End Test Matrix

Use this matrix after major milestones and again before release.

### A. Startup and health
- [ ] Backend starts with documented env vars only
- [ ] `/healthz` returns success
- [ ] `/readyz` reflects meaningful readiness state
- [ ] `/metrics` exposes metrics

### B. Check lifecycle
- [ ] Create HTTP check
- [ ] Create TCP check
- [ ] Create process or command check if enabled
- [ ] Disable check
- [ ] Re-enable check
- [ ] Duplicate check
- [ ] Delete check

### C. Execution behavior
- [ ] Scheduler executes checks on time
- [ ] Manual run works
- [ ] Manual single-check test works
- [ ] Timeout failure classified correctly
- [ ] Warning threshold classified correctly
- [ ] Recovery changes latest status correctly

### D. Alerts
- [ ] Warning sends alert when configured
- [ ] Critical sends alert when configured
- [ ] Cooldown prevents duplicates
- [ ] Recovery sends resolved notification
- [ ] Failed webhook delivery is recorded
- [ ] Failed email delivery is recorded

### E. Incidents
- [ ] Alert-worthy failure opens incident
- [ ] Repeated failure updates same incident
- [ ] Incident can be acknowledged
- [ ] Incident can be resolved
- [ ] Healthy recovery auto-resolves if policy says so

### F. UI
- [ ] Dashboard renders summary and table
- [ ] Check detail page shows latest status and recent runs
- [ ] Incident list and incident detail work
- [ ] Check form validates inputs
- [ ] Test-send alert channel works from UI

### G. Security
- [ ] Anonymous write call rejected
- [ ] Invalid token rejected
- [ ] Valid token accepted
- [ ] Secrets absent from standard logs and API payloads

### H. Durability
- [ ] Data survives service restart
- [ ] Data survives temporary backend error conditions as designed
- [ ] Retention cleanup does not break latest status or incidents

---

## 15. Recommended Commit And Delivery Breakdown

This is the recommended implementation slicing.

1. `chore: remove legacy startup and config artifacts`
2. `docs: add v1 ADRs and scope notes`
3. `refactor: introduce v1 domain models and store interfaces`
4. `feat: implement primary persistence repositories`
5. `feat: add latest status read model`
6. `feat: add auth middleware for mutating APIs`
7. `feat: stabilize v1 check and status APIs`
8. `feat: expand check definitions and scheduler policy`
9. `feat: add alert engine and delivery logging`
10. `feat: add incidents and audit trail`
11. `feat: scaffold frontend and dashboard`
12. `feat: add checks and incidents UI`
13. `feat: add alert channels UI`
14. `chore: add metrics, CI, deployment assets, runbook`
15. `chore: final release verification`

---

## 16. Risks To Watch During Execution

### Risk 1: Frontend starts too early
If frontend starts before API and persistence model stabilize, team will churn on contract changes.

**Mitigation**
Do not begin main UI implementation until Phase 2 API hardening is complete.

### Risk 2: Blob state lingers under new features
If incidents and alerts are added while still depending on current full-state rewrite approach, complexity and bugs will multiply.

**Mitigation**
Do storage redesign before adding alerts and incidents.

### Risk 3: Command checks create security hole
Command execution is high-risk even for internal tools.

**Mitigation**
Disable by default. Gate by explicit config. Validate and log carefully. Prefer other check types when possible.

### Risk 4: Alert spam reduces trust
Even good alerts become ignored if dedupe and cooldown are weak.

**Mitigation**
Implement dedupe and recovery flows before broad rollout.

### Risk 5: Docs drift from implementation
This repo already showed drift before planning.

**Mitigation**
Update docs in same commit as behavior changes wherever possible.

---

## 17. Final Exit Criteria

Do not mark v1 complete until all are true.

- [ ] Repo contains no dead legacy startup/config paths
- [ ] Persistence model is production-oriented and documented
- [ ] All mutating APIs require auth
- [ ] Scheduler supports practical check configuration
- [ ] Alerts send through at least email and webhook
- [ ] Incidents support open, acknowledge, resolve lifecycle
- [ ] Audit trail exists for sensitive actions
- [ ] Dashboard, checks UI, and incidents UI are usable
- [ ] Backend metrics and structured logs exist
- [ ] CI exists and passes
- [ ] Runbook and release checklist exist
- [ ] End-to-end test matrix passes
- [ ] Another engineer can clone repo, run product, and follow docs without tribal knowledge

---

## 18. Suggested Immediate Next Action

Start with Phase 0 and Phase 1 only. Do not jump into frontend or alerting first.

The single highest-value first implementation move is:
- redesigning persistence and domain boundaries so incidents, alerts, and UI can be built on stable foundations.

