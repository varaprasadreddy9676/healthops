<div align="center">

# HealthOps

**Open-source, self-hosted infrastructure monitoring with AI-assisted incident investigation.**

[![License](https://img.shields.io/github/license/varaprasadreddy9676/healthops?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](backend/go.mod)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)](frontend/package.json)
[![Vite](https://img.shields.io/badge/Vite-8-646CFF?style=flat-square&logo=vite&logoColor=white)](frontend/package.json)
[![MongoDB](https://img.shields.io/badge/MongoDB-required-47A248?style=flat-square&logo=mongodb&logoColor=white)](compose.yaml)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat-square&logo=docker&logoColor=white)](compose.yaml)

Monitor APIs, ports, Linux servers, MySQL, logs, SSL, DNS, domains, and heartbeat jobs.
When something breaks, HealthOps creates an incident, collects evidence, generates an AI incident brief, and keeps the RCA trail tied back to concrete signals.

One Docker command for production. One Docker command for a realistic demo.

[Quick Start](#quick-start) | [Demo Scenarios](#demo-scenarios) | [AI Features](#ai-native-incident-workflow) | [API Docs](backend/docs/api-reference.md) | [Deployment Guide](docs/deployment-guide.md)

</div>

---

## What HealthOps Is

HealthOps is for teams that want a practical monitoring stack they can run themselves:

- Synthetic HTTP/API checks with latency thresholds and expected body matching
- TCP, ping, DNS, SSL, domain expiry, log freshness, command, process, and heartbeat checks
- Agentless Linux server monitoring over SSH
- MySQL monitoring with connection, thread, process list, status variable, and query evidence
- Incident lifecycle management with acknowledgements, resolution, MTTA, and MTTR
- AI incident briefs with evidence citations, confidence factors, and an evidence ledger
- RCA reports, recommendations, remediation actions, notification routing, and audit logs
- MongoDB-backed persistence for checks, results, incidents, users, AI config, queues, and audit data

HealthOps is not a hosted SaaS. You own the data, deployment, keys, retention, and upgrade process.

---

## Screenshots

<table>
<tr>
<td width="50%"><img src="docs/screenshots/dashboard.png" alt="Dashboard" /></td>
<td width="50%"><img src="docs/screenshots/checks.png" alt="Health checks" /></td>
</tr>
<tr>
<td><b>Dashboard</b><br />Live health summary, incidents, response time, and recent activity.</td>
<td><b>Checks</b><br />Severity-sorted checks with status, latency, tags, and quick actions.</td>
</tr>
<tr>
<td width="50%"><img src="docs/screenshots/incidents.png" alt="Incidents" /></td>
<td width="50%"><img src="docs/screenshots/ai-analysis.png" alt="AI analysis" /></td>
</tr>
<tr>
<td><b>Incidents</b><br />Open, acknowledge, resolve, and inspect incident evidence.</td>
<td><b>AI Analysis</b><br />AI-assisted RCA, recommendations, and evidence-backed summaries.</td>
</tr>
<tr>
<td width="50%"><img src="docs/screenshots/servers.png" alt="Servers" /></td>
<td width="50%"><img src="docs/screenshots/mysql.png" alt="MySQL monitoring" /></td>
</tr>
<tr>
<td><b>Servers</b><br />Agentless SSH metrics for CPU, memory, disk, load, uptime, and processes.</td>
<td><b>MySQL</b><br />Database health, threads, connections, process list, samples, and alert rules.</td>
</tr>
<tr>
<td width="50%"><img src="docs/screenshots/analytics.png" alt="Analytics" /></td>
<td width="50%"><img src="docs/screenshots/settings.png" alt="Settings" /></td>
</tr>
<tr>
<td><b>Analytics</b><br />Uptime, failures, response time, incidents, and trend views.</td>
<td><b>Settings</b><br />Users, checks, AI providers, notifications, retention, and alert rules.</td>
</tr>
</table>

---

## Quick Start

### Production

Run HealthOps with MongoDB and persistent Docker volumes:

```bash
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD='change-this-to-a-strong-password' \
docker compose up -d --build
```

Open:

```text
http://localhost:8080
```

Login:

```text
username: admin
password: the password you set above
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Stop without deleting data:

```bash
docker compose down
```

Delete local production data:

```bash
docker compose down -v
```

### Demo

Run a full demo environment with seeded checks, Linux SSH targets, MySQL, Redis, nginx, a checkout API, log emitter, workload generator, and local mock AI provider:

```bash
docker compose -f compose.demo.yaml up -d --build
```

Open:

```text
http://localhost:18080
```

Login:

```text
username: admin
password: healthops-demo-admin
```

Run the smoke test:

```bash
scripts/demo-scenario.sh smoke
```

Stop and delete demo data:

```bash
docker compose -f compose.demo.yaml down -v
```

---

## Demo Scenarios

The demo is designed to show realistic operational failure modes, not just green dashboards.

```bash
# Basic validation
scripts/demo-scenario.sh smoke
scripts/demo-scenario.sh persistence
scripts/demo-scenario.sh mongo-outage

# API incidents
scripts/demo-scenario.sh api-slow
scripts/demo-scenario.sh api-down
scripts/demo-scenario.sh api-flaky
scripts/demo-scenario.sh recover

# AI incident workflow
scripts/demo-scenario.sh rca

# Logs and security evidence
scripts/demo-scenario.sh log-spike
scripts/demo-scenario.sh log-storm
scripts/demo-scenario.sh crash-logs
scripts/demo-scenario.sh sshd-bruteforce
scripts/demo-scenario.sh disk-pressure
scripts/demo-scenario.sh cert-expiry

# MySQL evidence
scripts/demo-scenario.sh mysql-load
```

The default demo uses a local mock OpenAI-compatible provider, so no external AI key is required.

To test a real OpenRouter provider and Slack notification path:

```bash
SLACK_WEBHOOK_URL='https://hooks.slack.com/services/...' \
OPENROUTER_API_KEY='sk-or-...' \
OPENROUTER_MODEL='openai/gpt-4o-mini' \
scripts/demo-scenario.sh configure-real-integrations

scripts/demo-scenario.sh real-incident
```

Real AI provider keys are stored through the HealthOps API/UI and encrypted at rest in the Mongo-backed AI configuration store. Do not commit real keys to `.env`, compose files, or source control.

---

## AI-Native Incident Workflow

HealthOps treats AI as part of incident handling, but keeps the AI grounded in evidence.

```text
Check/log/metric fails
  -> incident opens or updates
  -> evidence providers collect bounded context
  -> AI analysis queue runs
  -> incident brief is generated
  -> evidence ledger shows what supports, contradicts, or is missing
  -> RCA and remediation workflows keep the audit trail
```

Implemented AI features:

- Automatic incident analysis through BYOK providers
- OpenAI, Anthropic, Google Gemini, Ollama, and custom OpenAI-compatible providers
- OpenRouter support through the custom provider type
- AES-256-GCM encryption for provider API keys at rest
- Incident evidence collection from checks, MySQL snapshots, server metrics, audit events, and incident history
- AI Incident Brief with likely cause, impact, next actions, timeline, confidence factors, and citations
- Evidence ledger with `supported`, `unsupported`, `contradicted`, and `missing` signal categories
- RCA report generation for incidents
- AI recommendations and remediation workflows
- Graceful evidence-only briefs when AI is not configured or temporarily unavailable

The evidence ledger is intentionally deterministic. It helps operators see whether an AI claim is backed by collected signals, contradicted by healthy signals, unsupported because evidence was capped, or missing because a provider had no data.

---

## Supported Checks

| Type | What it monitors | Example |
|------|------------------|---------|
| `api` | HTTP/HTTPS status, body matching, latency | `https://api.example.com/health` |
| `tcp` | Port connectivity and response time | `db.internal:5432` |
| `ping` | ICMP reachability and latency | `10.0.0.10` |
| `dns` | DNS record resolution and TTL | `api.example.com` |
| `ssl` | Certificate expiry and TLS chain validity | `api.example.com:443` |
| `domain` | Domain registration expiry | `example.com` |
| `heartbeat` | Cron/job liveness through inbound pings | Backup job ping URL |
| `log` | Log file freshness | `/var/log/app.log` |
| `process` | Process existence over SSH | `nginx` |
| `command` | Command exit status over SSH | `systemctl is-active nginx` |
| `ssh` | Linux CPU, memory, disk, load, uptime, processes | SSH target |
| `mysql` | MySQL status variables, connections, threads, process evidence | DSN via environment variable |

Command checks and remediation actions are intentionally controlled because they can execute commands on monitored systems.

---

## Notifications

HealthOps supports:

- Slack
- Email/SMTP
- Discord
- Telegram
- PagerDuty
- Generic webhooks

Notification features:

- Severity filters
- Tag filters
- Cooldown windows
- Resolve notifications
- Test-send before saving
- Notification history and audit trails

---

## Persistence Model

MongoDB is required.

HealthOps stores runtime state in MongoDB:

- users and sessions
- checks and servers
- check results
- incidents and incident snapshots
- notification channels and delivery logs
- AI providers, AI queue, and AI outputs
- RCA reports, recommendations, remediation actions, and audit logs

`backend/config/default.json` and `backend/config/demo.json` are seed configs. After the first run, checks and runtime configuration are managed through MongoDB by the API/UI and survive restarts.

There is no JSON/JSONL production persistence path.

---

## Configuration

Production compose uses these key variables:

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | Yes, first run | empty | Initial admin password |
| `HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL` | No | `admin@healthops.local` | Admin email |
| `HEALTHOPS_BOOTSTRAP_ADMIN_RESET` | No | `false` | Reset admin password on startup; keep false in production |
| `HEALTHOPS_PORT` | No | `8080` | Host port for the dashboard |
| `HEALTHOPS_BIND` | No | `0.0.0.0` | Bind address; use `127.0.0.1` behind a local reverse proxy |
| `HEALTHOPS_PUBLIC_URL` | No | empty | Public URL used in notification links |
| `MONGODB_DATABASE` | No | `healthops` | Mongo database name |
| `MONGODB_COLLECTION_PREFIX` | No | `healthops` | Mongo collection prefix |
| `MYSQL_DSN` | No | empty | External MySQL DSN for monitored MySQL targets |
| `HEALTHOPS_SMTP_PASS` | No | empty | SMTP password for email notifications |

See [.env.example](.env.example) and [Deployment Guide](docs/deployment-guide.md) for the full list.

AI providers are normally configured in **Settings -> AI**. Bootstrap AI environment variables exist for demos and automation, but real keys should be kept out of committed files.

---

## Production Checklist

Before putting HealthOps on a real network:

- Set a strong `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` on first run
- Put HealthOps behind TLS, either through your reverse proxy or ingress
- Set `HEALTHOPS_PUBLIC_URL`
- Keep MongoDB on a private network
- Back up Docker volumes or MongoDB directly
- Configure at least one notification channel
- Configure AI provider keys through Settings, not committed files
- Keep `HEALTHOPS_BOOTSTRAP_ADMIN_RESET=false`
- Restrict command checks and remediation permissions to trusted operators
- Review SSH host key fingerprints for monitored servers
- Run `scripts/demo-scenario.sh smoke` against demo before exposing production

Backup and restore helpers:

```bash
scripts/healthops-mongo-backup.sh --help
scripts/healthops-mongo-restore.sh --help
```

---

## Architecture

```text
Browser
  |
  | REST + SSE
  v
Go backend
  |-- scheduler and check runners
  |-- incident manager
  |-- evidence providers
  |-- AI queue and provider adapters
  |-- notification engine
  |-- remediation registry
  |-- Prometheus metrics
  v
MongoDB
```

| Layer | Technology | Why |
|-------|------------|-----|
| Backend | Go 1.25 | Single static service, low memory use, simple deployment |
| HTTP | Go `net/http` | Minimal dependency surface and predictable behavior |
| Frontend | React 19 + TypeScript | Typed, component-based dashboard UI |
| Build | Vite 8 | Fast frontend builds and dev server |
| Styling | Tailwind CSS | Consistent utility-first product UI |
| Server state | TanStack Query | Cache, loading, retry, and invalidation for API data |
| Charts | Recharts | Uptime, latency, and trend visualizations |
| Icons | Lucide React | Consistent operational icon set |
| Storage | MongoDB 7 | Document model for checks, incidents, evidence, AI config, and audit data |
| Runtime | Docker Compose | One-command local, demo, and production deployments |
| Observability | `/metrics`, `/healthz`, SSE | Prometheus scraping, health checks, live UI updates |

---

## Resource Usage

Typical small deployment estimates:

| Setup | RAM | CPU idle | Disk/month |
|-------|-----|----------|------------|
| 20 checks + MongoDB | ~100-200 MB | <2% | <1 GB |
| 50 checks + MongoDB | ~200-400 MB | <3% | ~1-2 GB |
| 100 checks + MongoDB | ~400-700 MB | <5% | ~2-5 GB |

Actual usage depends on retention, check interval, MySQL snapshot volume, log ingestion volume, and AI/RCA frequency.

---

## Project Layout

```text
healthops/
|-- backend/                 Go service
|   |-- cmd/healthops/       Service entrypoint
|   |-- internal/monitoring/ Core monitoring, AI, incidents, repositories
|   |-- config/              First-run seed configs
|   `-- docs/                Backend API reference
|-- frontend/                React + TypeScript dashboard
|-- docker/                  Demo API, demo AI provider, Linux SSH targets, MySQL workload
|-- docs/                    Deployment, runbook, ADRs, OpenAPI, roadmap
|-- scripts/                 Demo scenarios and Mongo backup/restore helpers
|-- compose.yaml             Production compose
|-- compose.demo.yaml        Realistic demo compose
`-- Dockerfile               Multi-stage app build
```

---

## Development

Backend:

```bash
cd backend
go test ./...
go fmt ./...
go run ./cmd/healthops
```

Frontend:

```bash
cd frontend
npm install
npm run typecheck
npm run build
npm run dev
```

Full app through Docker:

```bash
docker compose up -d --build
docker compose -f compose.demo.yaml up -d --build
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Deployment Guide](docs/deployment-guide.md) | Docker, TLS, reverse proxy, production operations |
| [API Reference](backend/docs/api-reference.md) | REST API reference |
| [OpenAPI Spec](docs/openapi.yaml) | Machine-readable API spec |
| [Runbook](docs/runbook.md) | Backups, restore, maintenance, troubleshooting |
| [AI-Native Operations Roadmap](docs/ai-native-operations-roadmap.md) | Longer-term AI-native platform plan |
| [Architecture Decisions](docs/decisions/) | ADRs for persistence, auth, incidents, AI |

---

## Security

- JWT authentication
- Role-based access for admin and ops users
- Bcrypt password hashing
- Login rate limiting
- Security headers
- AES-256-GCM encryption for AI API keys and inline credentials at rest
- SSH host key fingerprint support
- Command checks disabled unless explicitly allowed
- Registry-gated remediation actions
- MongoDB-backed audit trail

Found a vulnerability? See [SECURITY.md](SECURITY.md).

---

## License

[MIT](LICENSE)
