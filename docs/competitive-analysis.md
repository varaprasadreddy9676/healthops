# HealthOps Competitive Analysis & Practical Roadmap

> Purpose: decide what to build next, based on what would make HealthOps more useful in real operations.
>
> Last verified: 2026-05-17
>
> Scope: uptime monitoring, cron/heartbeat monitoring, status pages, incident response, self-hosted monitoring, and lightweight observability.

---

## Executive Takeaways

HealthOps should not try to become Datadog, New Relic, Grafana Cloud, or a full APM suite. That market is expensive because it includes logs, traces, RUM, profiling, security, cloud inventory, and huge ingestion pipelines. HealthOps wins by being smaller, cheaper, private, fast to operate, and useful during the first 15 minutes of an incident.

The practical wedge is:

1. Replace the small-team stack: UptimeRobot + Healthchecks/Cronitor + basic Statuspage + basic PagerDuty/Opsgenie.
2. Stay self-hosted and predictable: no per-host, per-seat, per-GB, or per-incident surprise.
3. Make RCA useful: collect local evidence from HTTP, MySQL, SSH, command checks, logs, and feed it into BYOK/local AI.
4. Be honest about the market: AI RCA and SSH checks both exist elsewhere. HealthOps' differentiator is the combination of local/BYOK AI, agentless SSH/MySQL context, simple deployment, and low cost.
5. Build the boring missing features first. SSL expiry, domain expiry, heartbeat, DNS, ping, maintenance windows, flapping control, and status pages will change adoption more than advanced ML or dashboards.

The strongest short-term roadmap is:

1. Monitoring completeness: SSL, domain expiry, heartbeat, DNS, ping.
2. Alert trust: maintenance windows, consecutive failure threshold, recovery confirmation, flapping suppression.
3. Customer communication: public status page, component mapping, incident history, uptime calendar, status badge.
4. Team adoption: multi-user/RBAC, TOTP, API tokens, escalation policies, Slack/Teams ack.
5. HealthOps-specific leverage: AI postmortems, evidence-rich runbooks, approved remediation execution, importers from UptimeRobot/Healthchecks/Opsgenie.

---

## Current HealthOps Baseline

| Capability | Current strength |
|---|---|
| HTTP/API checks | Status code, response body substring, latency threshold |
| TCP checks | Connectivity and latency |
| Process checks | `ps`-based process existence matching |
| Command checks | Shell command execution with exit code and output validation |
| Log file freshness | File mtime freshness checks |
| MySQL monitoring | Deep MySQL status/variable collection, deltas, slow query rate, lock waits, buffer hit rate, alert rules |
| SSH server monitoring | Agentless CPU, memory, disk, load, process, network, and disk I/O collection over SSH |
| Incident lifecycle | Open, acknowledge, resolve, evidence snapshots, audit trail |
| AI RCA | BYOK providers: OpenAI, Anthropic, Gemini, Ollama, custom OpenAI-compatible endpoints |
| Notification channels | Slack, email, Telegram, PagerDuty, Discord, webhook |
| Automation engine | Propose and approve/reject remediation actions |
| Metrics/export | Prometheus `/metrics`, analytics, CSV/JSON export |
| Deployment | Self-hosted, low-cost VPS, open source |

Important positioning correction:

| Old claim to avoid | Better claim |
|---|---|
| "Only tool with SSH server checks" | "Modern self-hosted tool combining agentless SSH server checks with incident evidence and BYOK/local AI." Zabbix has agentless SSH checks. |
| "Only tool with AI RCA" | "AI RCA without SaaS lock-in: BYOK, local Ollama, and incident evidence from private infrastructure." Better Stack, Checkly, Datadog, New Relic, PagerDuty, Rootly, and incident.io all have AI incident features. |
| "Replaces Datadog" | "Replaces the small-team uptime, heartbeat, status page, and basic on-call stack; can complement Datadog/Grafana for teams that do not need full APM." |

---

## Verified Competitor Snapshot

Pricing and packaging change often. These notes were checked from official pages on 2026-05-17.

| Competitor | What matters | Practical implication for HealthOps |
|---|---|---|
| UptimeRobot | Free plan has 5-minute interval and basic status pages. Paid plans add 60-second checks, API/UDP/multi-location/slow-response/DNS monitoring, SSL/domain expiry, heartbeat monitoring, and fuller status pages. Team plan currently shows 100 monitors and around $29-38/month depending billing. [Source](https://uptimerobot.com/pricing/) | HealthOps needs SSL, domain, heartbeat, DNS, ping, and status page before it can honestly replace UptimeRobot for a team. |
| Better Stack | Free includes 10 monitors/heartbeats and 1 status page. Responder is about $29 annual or $34 monthly per responder. Includes on-call, unlimited phone/SMS alerts, smart incident merging, AI postmortems, maintenance windows, SSL, TLD expiry, DNS, status pages, and heartbeats. [Source](https://betterstack.com/pricing) | Do not try to beat Better Stack feature-for-feature. Win with self-hosting, privacy, local/BYOK AI, agentless SSH/MySQL evidence, and no per-responder billing. |
| Healthchecks.io | Focused heartbeat monitoring. Business is $20/month for 100 jobs or $16/month annually; Business Plus is $80/month for 1000 jobs or $64/month annually. [Source](https://healthchecks.io/pricing/) | A good heartbeat MVP can replace this quickly. This is one of the highest ROI features. |
| Cronitor | Free includes 5 monitors. Business is usage based: $2/month per monitor plus $5/month per user, with cron, heartbeat, website/API monitoring, automatic reports, and integrations. [Source](https://cronitor.io/pricing) | Heartbeat plus simple job logs and failure reasons is enough for many teams. |
| Atlassian Statuspage | Public pages: Free, Hobby $29/month, Startup $99/month, Business $399/month, Enterprise $1,499/month. Private pages start at $79/month. Audience-specific pages start at $300/month. [Source](https://www.atlassian.com/software/statuspage) | A basic public status page is valuable immediately. Subscribers, private pages, and audience-specific pages can wait. |
| PagerDuty | Free up to 5 users. Professional is $21/user/month annual or $25 monthly. Business is $41/user/month annual or $49 monthly. PagerDuty Advance AI starts at $415/month; AIOps starts at $699/month. [Source](https://www.pagerduty.com/pricing/) | HealthOps can replace only basic paging after escalation policies, schedules, ack/resolve workflows, and delivery reliability exist. |
| Opsgenie | New sales ended June 4, 2025; support ends April 5, 2027. Historical plans showed Essentials $9.45/user/month, Standard $19.95, Enterprise $31.90 annual. [Source](https://www.atlassian.com/software/opsgenie/pricing) | Opsgenie migration is a real market window. Import schedules/escalations later; first build basic on-call well. |
| incident.io | Basic is free and includes Slack/Teams response, single-team on-call, status page, and essential automation. Team is $15/user/month annual or $19 monthly for incident response, plus $10/user/month for on-call. Pro is $25/user/month plus $20/user/month for on-call. [Source](https://incident.io/pricing) | Modern incident tools are Slack/Teams-first. Two-way Slack/Teams ack matters more than another dashboard widget. |
| FireHydrant | Platform Pro is $9,600/year for growing teams, with up to 20 responders, runbooks, Slack/Teams chatbot, unlimited public status pages, on-call scheduling, escalation policies, SMS/voice/push, webhooks, and alert rules. Enterprise adds AI summaries, transcripts, retros, follow-ups, private incidents/pages, analytics. [Source](https://firehydrant.com/pricing/) | Not the first replacement target. Useful as a reference for runbooks, retrospectives, and incident workflow maturity. |
| Rootly | Essentials is $20/user/month and includes incident response, AI chat, AI similar incidents, AI scribe, retrospectives, status page, mobile app, metrics, and SSO/SAML. Products can be purchased standalone. [Source](https://rootly.com/pricing) | AI incident management is mainstream. HealthOps should emphasize private evidence and local/BYOK AI, not generic AI claims. |
| Datadog | Infrastructure Pro is $15/host/month annual; Database Monitoring is $70/database host/month annual; Synthetic API Tests are $5 per 10K API test runs; On-Call is $20/seat/month; Incident Management is $30/seat/month; Bits AI SRE investigations are $500 per 20 investigations annual. [Source](https://www.datadoghq.com/pricing/list/) | Do not chase Datadog breadth. Use cost predictability and privacy as the contrast. |
| New Relic | Usage model combines data and users. Standard full platform users list at $99/user/month after the first promotional user; Pro is higher. New Relic has AIOps and AI assistant capabilities. [Source](https://docs.newrelic.com/docs/licenses/license-information/usage-plans/new-relic-usage-plan/) | HealthOps is not a New Relic replacement unless the buyer only needs basic infra/service health and RCA. |
| Grafana Cloud | Pro starts at $19/month plus usage. Synthetics include API/browser test execution pricing. Incident Response & Management is $20/active IRM user after included usage. [Source](https://grafana.com/pricing/) | Integrate with Prometheus/Grafana instead of rebuilding Grafana dashboards. |
| Checkly | Synthetic monitoring platform with status pages and AI root cause analysis; automated RCA can trigger when checks fail. [Source](https://www.checklyhq.com/pricing/) | AI RCA is becoming expected in synthetic monitoring. HealthOps needs better local evidence, not just AI text. |
| Uptime Kuma | Open-source self-hosted monitoring supports HTTP(s), TCP, keyword, JSON query, websocket, ping, DNS record, push, Steam, Docker containers, 20-second intervals, multiple status pages, custom domains, ping chart, and certificate info. [Source](https://github.com/louislam/uptime-kuma) | This is the closest "simple self-hosted" competitor. HealthOps must close check/status gaps, then win on incidents, AI RCA, MySQL, and SSH. |
| OneUptime | Open source, self-host or cloud. Free includes 1 status page, 100 subscribers, unlimited manual monitors, incident management, logs/traces/metrics. Growth is $22/month, Scale $99/month. Active monitors are $1/month each. [Source](https://oneuptime.com/pricing) | This is the strongest open-source breadth competitor. HealthOps should be leaner, easier to run, and more evidence/RCA focused. |
| Zabbix | Supports agentless SSH checks; Zabbix agent is not needed for SSH checks. [Source](https://www.zabbix.com/documentation/6.4/en/manual/config/items/itemtypes/ssh_checks) | Do not claim SSH checks are unique. Claim simpler UX and AI-supported incident context. |
| Prometheus Alertmanager | Handles alerts, grouping, inhibition, silencing, and alert fanout patterns; has controls to avoid receiver flooding. [Source](https://prometheus.io/docs/alerting/latest/alertmanager/) | Accept Alertmanager webhooks rather than trying to replace Prometheus for metric alerting. |

---

## Where HealthOps Can Win

### 1. Small Teams With Real Infrastructure

Target buyer: startups, agencies, freelancers, small SaaS teams, and self-hosted operators with 5-50 services, a database, a few servers, and no dedicated SRE team.

What they need:

- "Tell me when the API, cron job, SSL cert, domain, database, or server is about to break."
- "Do not wake me for one flaky check."
- "Give me one page I can send customers during an outage."
- "Tell me what to check first."
- "Do not require an agent on every server."
- "Do not become another $300/month tool."

HealthOps should optimize for this workflow, not enterprise observability.

### 2. Private RCA

Competitors now have AI, but most require data to live in their SaaS platform and often price AI as an add-on or usage meter. HealthOps can make a stronger practical promise:

- local Ollama option,
- BYOK providers,
- encrypted keys,
- incident evidence from SSH/MySQL/command/log checks,
- no telemetry ingestion bill,
- no vendor data retention concern.

The output must be concrete. The product should show:

- likely root cause,
- confidence level,
- evidence used,
- commands to verify,
- suggested runbook steps,
- past similar incidents,
- what changed recently if HealthOps knows it.

### 3. Agentless SSH + MySQL Evidence

Datadog/New Relic/Grafana are stronger if the team installs agents and sends telemetry. HealthOps can win when:

- servers are client-owned,
- installing agents is not allowed,
- the operator only has SSH,
- database credentials can be read from env/config,
- the team wants enough signal for triage, not full observability.

This should be framed as "fast, low-friction ops context" rather than "enterprise telemetry."

---

## Practical Feature Roadmap

### Phase 1: Monitoring Completeness

Goal: honestly replace UptimeRobot + Healthchecks/Cronitor for small teams.

| Feature | Priority | Effort | Why it matters | MVP scope |
|---|---:|---:|---|---|
| SSL certificate monitoring | P0 | XS | Common outage cause, easy win, expected by uptime tools. | TLS handshake, leaf expiry, chain validity, issuer/subject, alert thresholds at 30/14/7 days. |
| Domain expiry monitoring | P0 | S | Teams forget renewals; high perceived value. | WHOIS/RDAP lookup, expiry parsing, alert thresholds, registrar/name server display. |
| Heartbeat/cron monitoring | P0 | S | Replaces a separate paid tool; catches silent job failures. | Unique ping URL, expected interval, grace period, last ping, missed/late/failure status, optional log payload. |
| DNS record monitoring | P0 | S | Catches DNS misconfig, bad deploys, hijack symptoms. | A/AAAA/CNAME/TXT/MX expected values, TTL display, compare actual vs expected. |
| ICMP ping monitoring | P0 | XS | Basic reachability check, expected feature. | `ping` or Go ICMP fallback, packet loss, latency, timeout. |
| Keyword negative match | P1 | XS | Useful for "must not contain error/maintenance" checks. | Extend current `responseContains` with `responseNotContains`. |
| SMTP/IMAP/POP banner check | P2 | S | Useful for agencies/MSPs with mail servers. | TCP connect plus greeting/banner validation. |
| Redis/Postgres/Mongo monitoring | P2 | M | Extends the deep DB-monitoring story. | Start with Redis INFO/PING and Postgres basic stats before MongoDB. |
| Docker container health | P2 | S | Uptime Kuma already has Docker; useful for self-hosters. | Docker socket/API container status, restart count, health status, OOM events. |

Phase 1 should ship before browser transactions, SNMP, Kubernetes maps, or anomaly detection. It closes the biggest "why can't it monitor this basic thing?" objections.

### Phase 2: Alert Trust

Goal: reduce noise so users trust HealthOps alerts.

| Feature | Priority | Effort | Why it matters | MVP scope |
|---|---:|---:|---|---|
| Maintenance windows | P0 | S | Every serious ops team expects planned downtime suppression. | One-time and recurring windows, check/tag scope, suppress notifications without deleting results. |
| Consecutive failure threshold | P0 | XS | Prevents alerting on one bad packet/request. | Open incident only after N failures; configurable per check. |
| Recovery confirmation | P0 | XS | Prevents instant resolve/reopen loops. | Resolve only after N successes. |
| Flapping suppression | P0 | S | Biggest quality-of-life fix for uptime tools. | If state changes repeatedly within a window, mark flapping and reduce notifications. |
| Alert grouping | P1 | M | One server outage should not create 20 separate pages. | Group incidents by host/tag/root dependency within a time window. |
| Notification delivery logs | P1 | S | Debugging failed alerts is operationally important. | Store channel, attempt time, status, response/error, retry count. |
| Snooze/silence | P1 | S | Faster than creating a maintenance window. | Snooze check/tag for 1h/4h/24h/custom with audit entry. |

This phase matters more than advanced channels. A single noisy week can make users remove a monitoring tool.

### Phase 3: Customer Communication

Goal: replace basic Statuspage/UptimeRobot status pages.

| Feature | Priority | Effort | Why it matters | MVP scope |
|---|---:|---:|---|---|
| Public status page | P0 | M | Visible customer trust feature; easy to understand. | Unauthenticated `/status`, components, current state, active incident banner, 90-day history. |
| Component mapping | P0 | S | Users need "API", "Dashboard", "Database", not raw check names only. | Map checks to components; component status follows worst mapped check. |
| Incident history timeline | P0 | S | Basic expectation for status pages. | Public resolved incidents with start/end/duration and public summary. |
| Scheduled maintenance posts | P1 | S | Needed for credible customer communication. | Public maintenance entries tied to maintenance windows. |
| Uptime calendar | P1 | S | Visual SLA proof for customers and management. | 90-day grid by component/check. |
| Status badge | P1 | XS | Useful in README and customer docs. | SVG badge endpoint: passing/degraded/down. |
| Email subscribers | P2 | M | Valuable but adds deliverability and unsubscribe complexity. | Double opt-in, unsubscribe, incident/maintenance notifications. |
| Custom domain | P2 | S | Expected by serious users. | Document reverse proxy config first; automate later. |
| Private status page | P2 | S | Useful for internal services. | Password or token-gated status page. |

Do not start with audience-specific pages, custom CSS/JS, subscriber analytics, or white-labeling. Those are enterprise Statuspage features, not first-order HealthOps value.

### Phase 4: Team Adoption

Goal: make HealthOps safe for a team, not just one operator.

| Feature | Priority | Effort | Why it matters | MVP scope |
|---|---:|---:|---|---|
| Multi-user accounts | P0 | M | Blocks team adoption. | Users, password hash, sessions, invitation or admin-created users. |
| RBAC | P0 | M | Prevents read-only users from changing checks/secrets. | Admin, operator, read-only, API token. |
| TOTP 2FA | P1 | S | Important once multiple users exist. | TOTP enrollment/recovery codes. |
| API tokens | P1 | S | Needed for automation/importers. | Per-user or service token, scopes, last-used, revoke. |
| Escalation policies | P1 | M | Replaces basic PagerDuty/Opsgenie behavior. | Channel/person sequence, delays, stop on ack. |
| On-call schedules | P1 | M | Needed for real paging. | Weekly rotation, overrides, current on-call, timezone support. |
| Slack/Teams interactive ack | P1 | M | Operators live in chat during incidents. | Ack/resolve buttons or slash commands; audit trail. |
| Runbooks on alerts | P1 | S | Makes alerts actionable. | Markdown runbook per check/tag included in incident and notification. |
| Microsoft Teams channel | P1 | S | Required for many business teams. | Adaptive card webhook with ack link. |

Team adoption is the point where HealthOps can start claiming it replaces basic PagerDuty/Opsgenie for small teams. Before this phase, it replaces monitoring tools, not on-call tools.

### Phase 5: HealthOps-Specific Leverage

Goal: build features that are hard for generic uptime tools to copy.

| Feature | Priority | Effort | Why it matters | MVP scope |
|---|---:|---:|---|---|
| AI postmortem generation | P1 | S | Directly uses incident timeline and evidence. | Generate summary, impact, timeline, root cause, detection, resolution, follow-ups. |
| Evidence-based AI output | P1 | M | Prevents generic AI fluff. | AI response must cite check result IDs, MySQL metrics, SSH metrics, log excerpts, and timestamps. |
| Similar incident lookup | P1 | M | Repeated outages become faster to solve. | Search previous incidents by check, error text, tags, AI category. |
| Approved remediation execution | P2 | M | Automation engine exists; execution creates real differentiation. | Only approved commands, scoped targets, dry run, timeout, audit log, rollback notes. |
| AI-generated runbook draft | P2 | M | Converts RCA into reusable operational knowledge. | Suggest runbook after incident; user edits and saves. |
| UptimeRobot/Healthchecks import | P2 | M | Reduces switching friction. | Import monitor exports/API data into HealthOps checks. |
| Opsgenie import | P2 | M | Timely because Opsgenie support ends in 2027. | Import schedules/escalation policies if Team Adoption phase exists. |
| Alertmanager ingestion | P2 | S | Lets Prometheus users centralize incidents. | Webhook receiver, label mapping, dedup key, incident creation. |

---

## Feature Impact Matrix

Score = impact x effort leverage. Effort leverage: 5 means cheap, 1 means expensive.

| Feature | Impact | Effort leverage | Score | Why |
|---|---:|---:|---:|---|
| SSL certificate monitoring | 5 | 5 | 25 | High anxiety, low implementation cost. |
| Heartbeat/cron monitoring | 5 | 4 | 20 | Replaces dedicated tools and catches silent failures. |
| Maintenance windows | 5 | 4 | 20 | Standard expectation and large noise reducer. |
| Consecutive failure/recovery thresholds | 5 | 5 | 25 | Tiny implementation, major trust improvement. |
| Domain expiry monitoring | 4 | 4 | 16 | Strong practical value, slightly messy parsing. |
| DNS monitoring | 4 | 4 | 16 | Common outage cause and easy to explain. |
| Public status page | 5 | 3 | 15 | Converts HealthOps from internal tool to customer-facing trust layer. |
| Flapping suppression | 5 | 3 | 15 | Reduces alert fatigue. |
| Multi-user/RBAC | 5 | 3 | 15 | Required for team adoption. |
| Runbooks attached to alerts | 4 | 4 | 16 | Makes every alert more actionable. |
| AI postmortem generation | 4 | 4 | 16 | Leverages existing AI and incident timeline. |
| Escalation policies | 5 | 3 | 15 | Needed to replace on-call tools. |
| Slack/Teams ack | 4 | 3 | 12 | Matches modern incident workflow. |
| Uptime calendar/status badge | 3 | 5 | 15 | Cheap trust feature. |
| Notification delivery logs | 4 | 4 | 16 | Helps debug alerting, prevents silent notification failures. |
| API tokens | 4 | 4 | 16 | Enables automation and importers. |
| Redis monitoring | 4 | 3 | 12 | Natural extension of deep DB monitoring. |
| Postgres monitoring | 4 | 2 | 8 | Valuable but more work to do well. |
| Approved remediation execution | 5 | 2 | 10 | Powerful but needs security controls. |
| Browser transaction monitoring | 3 | 1 | 3 | Expensive to run and maintain; defer. |
| Custom dashboard builder | 2 | 1 | 2 | Grafana already solves this. |

---

## Replacement Map

| Tool/category | Can HealthOps replace today? | After Phase 1-2 | After Phase 3-4 | Notes |
|---|---|---|---|---|
| Basic HTTP/TCP uptime monitoring | Partial | Yes | Yes | Missing common check types today. |
| UptimeRobot | No | Mostly | Yes for small teams | Multi-location/global probes remain a SaaS advantage. |
| Healthchecks.io | No | Yes | Yes | Heartbeat MVP is enough for many teams. |
| Cronitor | No | Mostly | Yes | Automatic reports and richer job logs can come later. |
| Atlassian Statuspage public page | No | No | Basic replacement | Subscribers/private/audience-specific pages are later. |
| Better Stack | No | Partial | Partial | Better Stack remains stronger for SaaS on-call/status workflows; HealthOps wins on self-hosted privacy and local evidence. |
| PagerDuty basic on-call | No | No | Basic replacement | Needs schedules, escalation, ack, reliable SMS/voice story. |
| Opsgenie | No | No | Basic replacement | Migration tooling would matter because support ends 2027-04-05. |
| Uptime Kuma | Partial | Competitive | Stronger for RCA | Uptime Kuma already has more check/status basics. HealthOps wins only after closing basics and emphasizing incidents/AI/MySQL/SSH. |
| OneUptime | No | Partial | Niche replacement | OneUptime is broader. HealthOps should be simpler and deeper for server/database incident triage. |
| Datadog/New Relic/Grafana Cloud | No | No | No | Do not claim full replacement. Claim lower-cost alternative for teams that do not need APM/log/traces/RUM. |
| Zabbix/Nagios | Partial | Stronger for small teams | Stronger for small teams | Zabbix remains broader; HealthOps should win on UX, deployment, and AI-assisted triage. |

---

## What Not To Build Yet

| Feature | Decision | Why |
|---|---|---|
| Real User Monitoring | Skip | Different product: JS SDK, session volume, privacy, replay, frontend analytics. |
| Distributed tracing/APM | Skip | Requires app instrumentation and high-volume ingestion. Integrate with OpenTelemetry/Grafana instead. |
| Browser transaction monitoring | Defer | Headless browser infra, screenshots, artifacts, worker scaling, and flake management are a product by themselves. |
| SNMP/network device suite | Defer | Useful for enterprise/network teams but not the current wedge. |
| Kubernetes topology map | Defer | High complexity and pushes HealthOps toward Datadog/Grafana territory. |
| Native mobile app | Skip for now | PWA plus push/Slack/Teams is enough until on-call usage is proven. |
| Custom dashboard builder | Skip | Grafana is the right dashboard answer. |
| Terraform provider | Defer | Useful only after API and check schema stabilize. |
| Marketplace/integration directory | Skip | Build the 5 integrations users actually need first. |
| Generic AI chat | Defer | AI should be tied to incident evidence and runbooks, not a broad chatbot. |

---

## MVP Specs That Would Make A Difference

### Heartbeat Monitoring

Minimum useful version:

- `GET` and `POST /api/v1/heartbeats/{token}` unauthenticated ping endpoint.
- Expected interval plus grace period.
- Optional job state: success, fail, start, complete.
- Optional text/log payload with size limit.
- Last ping, next expected ping, current state, and missed count.
- Incident opens when no ping arrives within interval + grace.
- Notifications include "last seen", "expected every", and last payload summary.

This replaces many Healthchecks/Cronitor use cases without complex scheduling UI.

### Status Page

Minimum useful version:

- Public unauthenticated route.
- Components mapped to checks.
- Component state: operational, degraded, major outage, maintenance.
- Active incident banner.
- Resolved incident history.
- 90-day uptime per component.
- Scheduled maintenance announcements.
- SVG status badge.

Do not start with subscribers, private pages, custom CSS, or custom domains. Those are second iteration features.

### Alert Noise Control

Minimum useful version:

- `failuresToOpen` per check.
- `successesToResolve` per check.
- `notificationCooldown`.
- `maintenanceWindowIds` or tag-based maintenance scope.
- Flapping detector: if check changes state more than N times in M minutes, suppress repeated notifications and show "flapping" in UI.
- Group incidents by shared `groupKey` such as host, tag, component, or dependency.

This directly improves trust. It is more valuable than adding another notification channel.

### Team/RBAC

Minimum useful version:

- Admin can create/invite users.
- Roles: admin, operator, read-only.
- API tokens with scopes.
- TOTP for admins/operators.
- Audit log for check changes, auth changes, incident actions, and secret changes.

This is enough for small team adoption without building enterprise SSO.

---

## Positioning

### Recommended one-liner

HealthOps is a self-hosted monitoring and incident triage platform for small teams that need uptime checks, server/database context, and private BYOK AI RCA without SaaS observability bills.

### Best homepage comparison

| Need | Typical stack | HealthOps answer |
|---|---|---|
| Website/API uptime | UptimeRobot/Better Stack | HTTP, TCP, ping, DNS, SSL, domain checks |
| Cron job failures | Healthchecks/Cronitor | Heartbeat checks |
| Public communication | Statuspage | Built-in status page |
| Basic incident workflow | PagerDuty/Opsgenie | Incidents, ack/resolve, escalation, runbooks |
| Server triage | SSH + manual commands | Agentless SSH evidence |
| Database triage | Manual MySQL inspection | MySQL metrics, rules, evidence snapshots |
| RCA help | Human investigation or SaaS AI add-on | BYOK/local AI with incident evidence |

### Claims to avoid

- Do not claim HealthOps is the only AI monitoring product.
- Do not claim HealthOps is the only SSH monitoring product.
- Do not claim HealthOps fully replaces Datadog/New Relic/Grafana.
- Do not lead with "$6 VPS replaces everything" without qualifying the use case.

### Claims that are defensible

- Self-hosted, open-source monitoring with predictable infrastructure cost.
- BYOK and local AI options for incident RCA.
- Agentless SSH server context without installing a host agent.
- Deep MySQL evidence captured at incident time.
- Designed to replace the small-team uptime, heartbeat, status page, and basic on-call stack.

---

## Recommended Next 10 Features

In order:

1. Consecutive failure/recovery thresholds.
2. Maintenance windows.
3. SSL certificate monitoring.
4. Heartbeat/cron monitoring.
5. Domain expiry monitoring.
6. DNS monitoring.
7. Public status page with components and incident history.
8. Multi-user accounts plus RBAC.
9. Notification delivery logs.
10. AI postmortem generation from incident timeline and evidence.

This order is intentionally practical. It first fixes trust and monitoring coverage, then adds customer-facing value, then team adoption, then AI leverage.

---

## Source Index

- UptimeRobot pricing and feature matrix: https://uptimerobot.com/pricing/
- Better Stack pricing and feature matrix: https://betterstack.com/pricing
- Healthchecks.io pricing: https://healthchecks.io/pricing/
- Cronitor pricing: https://cronitor.io/pricing
- Atlassian Statuspage pricing: https://www.atlassian.com/software/statuspage
- PagerDuty pricing: https://www.pagerduty.com/pricing/
- Opsgenie end-of-sale/support and historical pricing: https://www.atlassian.com/software/opsgenie/pricing
- incident.io pricing: https://incident.io/pricing
- FireHydrant pricing: https://firehydrant.com/pricing/
- Rootly pricing: https://rootly.com/pricing
- Datadog pricing list: https://www.datadoghq.com/pricing/list/
- New Relic usage plan: https://docs.newrelic.com/docs/licenses/license-information/usage-plans/new-relic-usage-plan/
- Grafana Cloud pricing: https://grafana.com/pricing/
- Checkly pricing and AI RCA: https://www.checklyhq.com/pricing/
- Uptime Kuma GitHub feature list: https://github.com/louislam/uptime-kuma
- OneUptime pricing: https://oneuptime.com/pricing
- Zabbix agentless SSH checks: https://www.zabbix.com/documentation/6.4/en/manual/config/items/itemtypes/ssh_checks
- Prometheus Alertmanager docs: https://prometheus.io/docs/alerting/latest/alertmanager/
