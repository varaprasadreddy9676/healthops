# HealthOps

**Open-source infrastructure monitoring with a modern UI, AI-powered incident analysis, and MySQL deep monitoring.**

HealthOps is a single-binary Go backend + React frontend that monitors your servers, APIs, databases, and services — then alerts you when things go wrong and uses AI to help you fix them.

## Demo

<div align="center">

https://github.com/user-attachments/assets/healthops-demo.mp4

*Dashboard → Health Checks → Incidents → Analytics → AI Analysis → MySQL Monitoring → Dark Mode*

</div>

<details>
<summary>📸 Screenshots</summary>

![Dashboard](docs/screenshots/dashboard.png)

</details>

## Why HealthOps?

- **Zero-config demo** — Run one script, get a full monitoring dashboard plus real demo targets on a local port
- **7 check types** — HTTP APIs, TCP ports, processes, commands, logs, MySQL databases, SSH remote servers
- **AI incident analysis** — Bring your own key (OpenAI, Anthropic, Google, Ollama) to auto-analyze incidents
- **Beautiful UI** — React + Tailwind dashboard with real-time SSE updates, charts, and dark mode
- **Single binary app** — Go backend serves the React UI; Docker Compose adds MongoDB persistence and realistic demo targets
- **62+ API endpoints** — Full REST API for automation and integration
- **Production-ready** — JWT auth, user management, notification channels, audit logging, Prometheus metrics, retention cleanup, encrypted AI keys

## Screenshots

| Dashboard | Health Checks | Incidents |
|-----------|--------------|-----------|
| ![Dashboard](docs/screenshots/dashboard.png) | ![Checks](docs/screenshots/checks.png) | ![Incidents](docs/screenshots/incidents.png) |

| Servers | Analytics | Settings |
|---------|-----------|----------|
| ![Servers](docs/screenshots/servers.png) | ![Analytics](docs/screenshots/analytics.png) | ![Settings](docs/screenshots/settings.png) |

| AI Analysis | MySQL Monitoring |
|-------------|-----------------|
| ![AI Analysis](docs/screenshots/ai-analysis.png) | ![MySQL](docs/screenshots/mysql.png) |

## Quick Start

### Production

Run HealthOps with MongoDB persistence:

```bash
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD='change-this-strong-password' docker compose up -d --build
```

Open [http://localhost:8080](http://localhost:8080) and log in with:

| Field | Value |
|-------|-------|
| Username | `admin` |
| Password | the password you provided in `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` |

MongoDB is internal to the Docker network and is not published to the host. To stop the stack:

```bash
docker compose down
```

### Demo

Run the full demo stack with realistic monitoring targets:

```bash
docker compose -f compose.demo.yaml up -d --build
```

Open [http://localhost:18080](http://localhost:18080) and log in with `admin` / `healthops-demo-admin`.

The demo starts HealthOps, MongoDB, MySQL, Redis, nginx, two Linux SSH targets, a controllable checkout API, a log emitter, a MySQL workload generator, and a local OpenAI-compatible demo AI provider. AI incident briefs and RCA work immediately without an external API key.

Trigger real scenarios:

```bash
scripts/demo-scenario.sh api-slow
scripts/demo-scenario.sh api-down
scripts/demo-scenario.sh api-flaky
scripts/demo-scenario.sh log-spike
scripts/demo-scenario.sh mysql-load
scripts/demo-scenario.sh rca
scripts/demo-scenario.sh recover
```

To stop and delete demo data:

```bash
docker compose -f compose.demo.yaml down -v
```

### Verify

```bash
# Production
curl http://localhost:8080/healthz

# Demo
curl http://localhost:18080/healthz
```

## Features

### Health Check Types

| Type | What it monitors | Example |
|------|-----------------|---------|
| `api` | HTTP/HTTPS endpoints | REST APIs, health endpoints, websites |
| `tcp` | Port connectivity | Database ports, service ports |
| `process` | Running processes | nginx, node, postgres |
| `command` | Shell command output | Custom scripts, disk checks |
| `log` | Log file freshness | App logs, system logs |
| `mysql` | MySQL database health | Connections, queries, replication |
| `ssh` | Remote server checks | SSH-based remote monitoring |

Each check supports: custom intervals, retries, cooldowns, timeout, and warning thresholds.

### Authentication & User Management

- **JWT token-based authentication** — Login via `/api/v1/auth/login`, receive a signed JWT
- **User management** — Create, update, delete users with admin/viewer roles
- **Role-based access** — Admins can mutate; viewers are read-only
- **Demo credentials** — Docker demo bootstraps `admin` / `healthops-demo-admin`; production deployments must set their own bootstrap password

### Notification Channels

Multi-channel alerting with smart filtering:
- **6 channel types** — Email, Slack, Discord, Telegram, Webhooks, PagerDuty
- **Smart filters** — Route alerts by severity, specific checks, check types, servers, and tags
- **Professional HTML emails** — Enterprise-grade email template with severity-coded headers, incident stats, and dashboard links (with plain text fallback)
- **Deduplication** — Incident-level dedup prevents duplicate notifications for the same incident + channel; cooldown per check
- **Test notifications** — Send test alerts from the UI before going live
- **Resolution alerts** — Optionally notify when incidents resolve

### Incident Management

- Auto-created incidents from configurable alert rules
- Full lifecycle: **open → acknowledge → resolve**
- Evidence snapshots captured at incident creation
- MTTA/MTTR analytics

### Alert Rules

- **5 default alert rules** out of the box (critical check failure, warning check failure, high failure rate, extended downtime, response time degradation)
- Configurable thresholds, cooldowns, and consecutive-breach logic
- Per-check or global scope
- File-persisted at `data/alert_rules.json`

### AI-Powered Analysis (BYOK)

Bring your own API key from any provider:
- **OpenAI** (GPT-4, GPT-3.5)
- **Anthropic** (Claude)
- **Google** (Gemini)
- **Ollama** (local models)
- **Custom** (any OpenAI-compatible API)

AI auto-analyzes new incidents with configurable prompt templates. API keys are AES-256-GCM encrypted at rest.

### MySQL Deep Monitoring

- Live `SHOW GLOBAL STATUS/VARIABLES` collection
- Delta computation for rate metrics
- 9 built-in alert rules (connection utilization, slow queries, lock waits, replication lag, etc.)
- Dedicated sub-pages: connections, queries, threads, server stats

### Server Management

- Add and manage remote servers
- SSH-based health checks (process, command, connectivity)
- Server grouping and tagging

### Analytics & Observability

- **Uptime tracking** per check with 7-day trends
- **Response time charts** with percentile breakdowns
- **Failure rate** analysis
- **Prometheus metrics** at `/metrics` (check runs, failures, durations, HTTP requests)
- **Audit logging** for all mutations
- **SSE real-time events** for live dashboard updates
- **CSV/JSON export** for incidents and results

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   React Frontend                     │
│        (Vite + TypeScript + Tailwind + Recharts)     │
└──────────────────────┬──────────────────────────────┘
                       │ REST API + SSE
┌──────────────────────┴──────────────────────────────┐
│                   Go Backend                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ │
│  │Scheduler │ │ Runner   │ │ Incident │ │  AI    │ │
│  │(per-check│ │(7 types) │ │ Manager  │ │Service │ │
│  │ timers)  │ │          │ │          │ │(BYOK)  │ │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └───┬────┘ │
│       │             │            │            │      │
│  ┌────┴─────────────┴────────────┴────────────┴───┐  │
│  │              Hybrid Store                       │  │
│  │    (File-based primary + optional MongoDB)      │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  JWT Auth · Users · Notifications · Alert Rules      │
│  Audit · Prometheus · Validation · Deduplication     │
└──────────────────────────────────────────────────────┘
```

## Documentation

| Document | Description |
|----------|-------------|
| [Deployment Guide](docs/deployment-guide.md) | **Start here** — Complete setup for dev, Docker, and production. Covers ports, env vars, TLS/SSL, domains, notifications, SSH, MySQL, troubleshooting, and log debugging |
| [API Reference](backend/docs/api-reference.md) | Full reference for all 62+ REST endpoints |
| [Operational Runbook](docs/runbook.md) | Day-to-day operations, backup/restore, performance tuning |
| [Architecture Decisions](docs/) | ADRs for scope, persistence, auth, incidents |

## Project Layout

```
healthops/
├── backend/                  # Go backend service
│   ├── cmd/healthops/        # Main entrypoint
│   ├── cmd/loadtest/         # Load testing tool
│   ├── internal/monitoring/  # Core modules (50+ files)
│   ├── config/default.json   # Default check configuration
│   └── docs/                 # API reference, specs, runbook
├── frontend/                 # React + TypeScript + Vite
│   └── src/
│       ├── pages/            # 11 pages + 4 MySQL sub-pages
│       ├── components/       # Reusable UI components
│       ├── api/              # API client modules
│       └── hooks/            # SSE, export hooks
├── docker/                   # Docker configs & MySQL init
├── docs/                     # ADRs, runbook, plans
├── compose.yaml              # Production stack: HealthOps + MongoDB
├── compose.demo.yaml         # Demo stack with realistic targets and scenarios
├── Dockerfile                # Multi-stage build (frontend + backend)
└── ReadMe.md                 # This file
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_PATH` | `config/default.json` | Check configuration file |
| `DATA_DIR` | `data/` | Data directory for JSONL repos |
| `FRONTEND_DIR` | — | Path to frontend dist folder |
| `STORAGE_BACKEND` | `file` | Set to `mongo` for production Docker persistence |
| `MONGODB_URI` | — | MongoDB connection string |
| `MONGODB_DATABASE` | `healthops` | MongoDB database name |
| `MONGODB_COLLECTION_PREFIX` | `healthops` | MongoDB collection prefix |
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | — | Required for the first Docker production admin user |
| `CORS_ORIGIN` | — | Allowed CORS origin (for custom domains) |
| `MYSQL_DSN` / check `dsnEnv` | — | MySQL DSN for mysql checks |

See the [Deployment Guide](docs/deployment-guide.md) for full details on all configuration options, notification setup, SSH servers, TLS, and troubleshooting.

### Check Configuration

Checks are seeded from `backend/config/default.json` on the first run, then managed via the API (`/api/v1/checks`) and persisted in MongoDB when using Docker. Each check supports:

```json
{
  "id": "my-api",
  "name": "My API",
  "type": "api",
  "target": "https://api.example.com/healthz",
  "intervalSeconds": 30,
  "timeoutSeconds": 10,
  "retryCount": 2,
  "retryDelaySeconds": 5,
  "cooldownSeconds": 60,
  "warningThresholdMs": 500,
  "server": "prod-server-1",
  "tags": ["production", "critical"]
}
```

## API Overview

HealthOps exposes 62+ REST endpoints. Full reference: [`backend/docs/api-reference.md`](backend/docs/api-reference.md)

**Core:** `/healthz`, `/readyz`, checks CRUD, manual runs, summary, results, dashboard

**Auth & Users:** login, user CRUD, role management

**Incidents:** list, detail, acknowledge, resolve, evidence snapshots

**Notifications:** channel CRUD, toggle, test, smart filters

**MySQL:** samples, deltas, health card, time-series, AI questions

**AI (BYOK):** config, providers, prompts, analyze, health check, results queue

**More:** alert rules, analytics (uptime/response-times/failure-rate/incidents), audit log, SSE events, Prometheus metrics, CSV exports

## Testing

```bash
cd backend
go test ./...           # All tests
go test ./... -race     # With race detector
go fmt ./...            # Format check
```

Load testing:

```bash
cd backend
go run ./cmd/loadtest -scenario=query -duration=2m
```

## Security

- **JWT authentication** with role-based access control (admin/viewer)
- User management with secure password handling
- Command checks disabled by default (`allowCommandChecks=false`)
- AI API keys AES-256-GCM encrypted at rest
- Notification deduplication prevents alert storms
- Input validation on all API endpoints
- Secrets in env vars, never in config files

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23, `net/http` stdlib |
| Frontend | React 19, TypeScript, Vite 6, Tailwind CSS |
| Charts | Recharts |
| State | TanStack React Query |
| Storage | File-based (primary) + MongoDB (optional mirror) |
| Monitoring | Prometheus client, SSE |
| Container | Docker multi-stage build |

## Contributing

1. Fork the repo
2. Create a feature branch
3. Run tests: `cd backend && go test ./...`
4. Submit a PR

## License

Open source. See repository for details.
