# HealthOps

> **Self-hosted infrastructure monitoring that runs on a $6/month server and replaces tools costing $300–500/month.**

[![License](https://img.shields.io/github/license/varaprasadreddy9676/healthops)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/varaprasadreddy9676/healthops/ci.yml?label=CI)](https://github.com/varaprasadreddy9676/healthops/actions)
[![Docker](https://img.shields.io/badge/Docker-ready-blue)](https://github.com/varaprasadreddy9676/healthops/pkgs/container/healthops)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8)](https://golang.org)

HealthOps monitors your servers, APIs, databases, and services — alerts you the moment something breaks, and uses AI to tell you why.

One Docker Compose command. No agents to install. No per-host fees. No seat limits.

---

> 📹 **Demo video coming soon.** To add one: record a screen capture, drag it into a new GitHub Issue, copy the asset URL, and paste it here.

---

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

---

## The problem with existing tools

Most monitoring tools are priced for enterprises — not the teams that need them most.

| Tool | Typical cost for a 5-person team | What you get |
|------|----------------------------------|--------------|
| Datadog | $300–500/month | Monitoring + alerts |
| New Relic | $250/month | Monitoring + alerts |
| PagerDuty | $100/month | On-call only, no monitoring |
| Better Uptime | $30–80/month | Basic uptime checks only |
| **HealthOps** | **$6–10/month** (your VPS) | **Monitoring + alerting + AI analysis + MySQL deep monitoring** |

A $6/month Hetzner or DigitalOcean server with 2GB RAM runs HealthOps comfortably for 50+ checks and a small team. Unlimited users, unlimited checks, unlimited notification channels — all included.

---

## Who uses HealthOps

### Startups and small teams who can't justify enterprise pricing

You have 5–15 servers, a handful of APIs, and a MySQL database. You need to know when things break — before your users do. You don't need a $400/month contract.

HealthOps runs alongside your stack on a small VPS. Configure 30 checks in 20 minutes. Get Slack alerts when anything fails. Done.

### Freelancers and agencies managing multiple clients

You're responsible for 10–30 client websites. One dashboard shows you everything. You know about the outage before your client calls. You look professional.

No per-site pricing. Add as many clients as you want.

### DevOps engineers who hate noisy alerts

Alert fatigue is real. HealthOps groups related failures into a single incident, deduplicates notifications across channels, and respects cooldowns. You get one Slack message — not thirty.

And when something does fire, the AI analysis tells you exactly what happened and what to check first.

### Self-hosted SaaS operators who care about data sovereignty

Your monitoring data doesn't need to live in a vendor's cloud. HealthOps is fully self-hosted — your data stays on your servers, behind your firewall. GDPR, SOC 2, internal compliance — all simpler when you own the stack.

---

## Real-world scenarios

### "My checkout API went down at 2am and I didn't know for 45 minutes"

Set up an API check on your checkout endpoint. When it fails:
- HealthOps creates an incident immediately
- Slack/PagerDuty fires within 60 seconds
- The AI analyzes the failure pattern and tells you whether it's a database connection issue, a timeout spike, or an upstream dependency

No more waking up to angry customer emails.

---

### "Our MySQL database is slow but we don't know why"

HealthOps's MySQL deep monitoring runs `SHOW GLOBAL STATUS` every 30 seconds and tracks:
- Connection pool utilization (how close to max_connections?)
- Slow query rate (queries/sec taking >1s)
- Lock wait time (are writes blocking reads?)
- InnoDB buffer pool hit rate (is MySQL actually using RAM?)
- Replication lag (if you have replicas)

When connection utilization hits 85%, you get a warning. At 95%, a critical alert fires. You fix it before the database starts rejecting connections.

---

### "We have 5 microservices and I'm manually SSHing into servers to check if processes are running"

Add a process check for each service:

| Service | Check type | Target |
|---------|-----------|--------|
| API server | `process` | `node` |
| Background worker | `process` | `worker.py` |
| nginx | `process` | `nginx` |
| Redis | `tcp` | `localhost:6379` |
| Your app's API | `api` | `https://api.myapp.com/health` |

If any process dies, you get a Slack message in under 60 seconds. No more SSH loops.

---

### "A client called to say their website is down and I had no idea"

Add an API check for each client's domain. Set the interval to 60 seconds. Configure a separate Slack channel (or email) per client if you want.

When their site goes down, you know before they do. When it recovers, you get a resolution alert so you know the issue cleared — without having to check manually.

---

### "I deployed a change and response times went up but I didn't catch it for hours"

Set a `warningThresholdMs` on your API checks. If your endpoint normally responds in 150ms and it starts taking 800ms after a deploy, HealthOps fires a warning alert immediately.

The analytics page shows you response time trends over the last 7 days — so you can see exactly when the degradation started and correlate it with your deployment.

---

### "We have 3 monitoring tools and none of them talk to each other"

HealthOps replaces:

- **UptimeRobot / Better Uptime** — API and TCP checks ✓
- **Basic server monitoring (htop over SSH)** — SSH-based CPU, memory, disk, load monitoring ✓
- **Manual MySQL inspection** — MySQL deep monitoring with 9 built-in alert rules ✓
- **PagerDuty** (for small teams) — notification channels with deduplication ✓
- **AI RCA tools** — incident analysis with your own API key ✓

One dashboard. One place to look. One tool to maintain.

---

## Why it runs on minimal infrastructure

Most monitoring tools run heavy agents on every server you monitor. HealthOps is different:

- **Agentless** — HealthOps reaches out to your services over HTTP, TCP, SSH, or MySQL connections. Nothing runs on your monitored servers (except an SSH key file for remote checks).
- **Go backend** — the entire backend is a single compiled binary using ~50–80MB RAM at rest.
- **Efficient scheduler** — checks run on per-check timers, not a global polling loop. 100 checks at 60-second intervals generates 100 requests/minute — trivial for any server.
- **File-first storage** — works without MongoDB. Add MongoDB when you want persistence across restarts and a larger team.

**Tested resource usage:**

| Setup | RAM | CPU (idle) | Disk |
|-------|-----|-----------|------|
| 20 checks, file storage | ~60 MB | <1% | <100 MB |
| 50 checks, MongoDB | ~120 MB | <2% | ~1 GB/month (with 7-day retention) |
| 100 checks, MongoDB | ~180 MB | <3% | ~2 GB/month |

A $6/month Hetzner CX11 (2 vCPU, 2GB RAM) handles 100+ checks with room to spare.

---

## Quick Start

### Option A — Try the Demo (no config needed)

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops
docker compose -f compose.demo.yaml up -d --build
```

Open **http://localhost:18080** and log in with `admin` / `healthops-demo-admin`.

The demo includes HealthOps, MongoDB, MySQL, Redis, nginx, two Linux SSH targets, a controllable API, a log emitter, and a built-in AI provider. AI incident analysis works immediately — no external API key needed.

Trigger realistic failure scenarios:

```bash
scripts/demo-scenario.sh api-slow    # API response times spike
scripts/demo-scenario.sh api-down    # API goes offline → incident fires
scripts/demo-scenario.sh mysql-load  # MySQL under load → alert triggers
scripts/demo-scenario.sh rca         # AI root-cause analysis runs
scripts/demo-scenario.sh recover     # everything goes green
```

Stop and wipe demo data:

```bash
docker compose -f compose.demo.yaml down -v
```

---

### Option B — Production Setup (persistent data)

**Step 1 — Create a `.env` file:**

```bash
cat > .env << 'EOF'
# Required: admin password for first login
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD=choose-a-strong-password-here

# Recommended: your public URL (needed for email notification links)
# HEALTHOPS_PUBLIC_URL=https://healthops.yourcompany.com
EOF
```

**Step 2 — Clone and start:**

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops
docker compose up -d --build
```

**Step 3 — Open** http://localhost:8080 — log in with `admin` / your password.

> **Data persists** in Docker volumes. Restart any time with `docker compose up -d`.

**Startup timeline:**

```
Building image...       (2–5 min first run, ~15 sec after)
Starting MongoDB...     (~10 seconds)
Starting HealthOps...   (~5 seconds)
Ready at http://localhost:8080
```

```bash
curl http://localhost:8080/healthz
# → {"success":true,"data":{"status":"ok"}}
```

---

## Monitor Your First Check in 5 Minutes

1. Log in → **Checks** → **Add Check**
2. Fill in:

   | Field | Example |
   |-------|---------|
   | Name | My API |
   | Type | `api` |
   | Target | `https://yoursite.com/healthz` |
   | Interval | `60` seconds |

3. **Save** — check runs immediately, dashboard updates in real time

**All check types:**

| Type | Use for | Example target |
|------|---------|---------------|
| `api` | HTTP endpoints, REST APIs, websites | `https://api.myapp.com/health` |
| `tcp` | Ports — databases, Redis, any TCP service | `db.internal:5432` |
| `process` | Running processes on monitored servers | `nginx` |
| `command` | Custom shell scripts, disk checks, anything | `df -h \| grep '9[0-9]%'` |
| `log` | Log file freshness (has it been written to recently?) | `/var/log/app.log` |
| `mysql` | MySQL/MariaDB health and performance | DSN via env var |
| `ssh` | Remote server CPU, memory, disk, load | SSH credentials in config |

---

## Set Up Slack Alerts in 3 Steps

1. Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **Incoming Webhooks** → create a webhook → copy the URL
2. In HealthOps: **Settings** → **Notification Channels** → **Add Channel** → **Slack** → paste URL
3. Click **Test** → **Save**

Done. You'll get a Slack message every time a check fails. Same flow for **Email**, **Discord**, **Telegram**, **PagerDuty**, and custom **Webhooks**.

---

## System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| RAM | 512 MB | 1–2 GB |
| Disk | 5 GB | 20 GB |
| Docker | 24.0+ | Latest |
| Docker Compose | v2.20+ | Latest |
| OS | Linux, macOS, Windows (WSL2) | Linux |

**Works on:** Hetzner CX11 ($6/mo) · DigitalOcean Droplet · Linode Nanode · AWS t3.micro (free tier) · Any VPS with Docker

---

## Prerequisites

```bash
docker --version        # Must be 24.x or later
docker compose version  # Must be v2.x or later
```

Install Docker: [docs.docker.com/get-docker](https://docs.docker.com/get-docker/)

---

## Features

### Health Check Types

| Type | What it monitors |
|------|-----------------|
| `api` | HTTP/HTTPS endpoints — status code, response body, latency |
| `tcp` | Port connectivity and latency |
| `process` | Process existence by name or command |
| `command` | Shell command exit code and output |
| `log` | Log file modification time |
| `mysql` | MySQL health, performance, and replication |
| `ssh` | Remote server CPU, memory, disk, load, top processes |

### Notification Channels

- **6 channel types** — Email, Slack, Discord, Telegram, Webhooks, PagerDuty
- **Smart filters** — route by severity, check, type, server, or tag
- **Deduplication** — one notification per incident, not one per check run
- **Test button** — verify before going live
- **Resolution alerts** — know when incidents clear

### Incident Management

- Auto-created from configurable alert rules
- Lifecycle: open → acknowledge → resolve
- Evidence snapshots at creation time
- MTTA/MTTR analytics

### Alert Rules

- 5 built-in rules (critical failure, warning failure, high failure rate, extended downtime, response time degradation)
- Configurable thresholds, cooldowns, consecutive-breach logic
- Per-check or global scope

### AI-Powered Analysis (BYOK)

| Provider | Models |
|----------|--------|
| OpenAI | GPT-4, GPT-3.5 |
| Anthropic | Claude |
| Google | Gemini |
| Ollama | Local models (free, private) |
| Custom | Any OpenAI-compatible endpoint |

AI auto-analyzes new incidents. API keys encrypted with AES-256-GCM at rest.

### MySQL Deep Monitoring

- Tracks 30+ MySQL status variables every 30 seconds
- Computes delta rates (queries/sec, connections/sec, etc.)
- 9 built-in alert rules: connection utilization, slow queries, lock waits, replication lag, InnoDB buffer pool, and more
- Dedicated dashboard pages: connections, queries, threads, server stats

### Analytics & Observability

- 7-day uptime per check
- Response time charts with p50/p95/p99
- Failure rate trends
- Prometheus metrics at `/metrics`
- Audit log for all mutations
- Real-time SSE dashboard updates
- CSV/JSON export

---

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
│       └─────────────┴────────────┴────────────┘      │
│  ┌──────────────────────────────────────────────┐    │
│  │              Hybrid Store                    │    │
│  │   (File-based primary + optional MongoDB)    │    │
│  └──────────────────────────────────────────────┘    │
│  JWT Auth · Users · Notifications · Alert Rules      │
│  Audit · Prometheus · Validation · Deduplication     │
└──────────────────────────────────────────────────────┘
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Deployment Guide](docs/deployment-guide.md) | Full setup: Docker, bare metal, TLS/SSL, reverse proxy, notifications, SSH, MySQL |
| [API Reference](backend/docs/api-reference.md) | All 62+ REST endpoints |
| [OpenAPI Spec](docs/openapi.yaml) | Machine-readable OpenAPI 3.0 |
| [Runbook](docs/runbook.md) | Day-to-day ops, backup/restore, performance tuning |
| [Architecture Decisions](docs/) | ADRs for scope, persistence, auth, incidents |

---

## Project Layout

```
healthops/
├── backend/                  # Go backend (cmd/healthops = entrypoint)
├── frontend/                 # React + TypeScript + Vite
├── docker/                   # Init scripts, workload generators
├── docs/                     # Guides, ADRs, screenshots
├── .github/                  # CI/CD, issue templates
├── compose.yaml              # Production: HealthOps + MongoDB
├── compose.demo.yaml         # Demo with realistic targets
└── Dockerfile                # Multi-stage build
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | — | Admin password for first run |
| `HEALTHOPS_PUBLIC_URL` | — | Your public URL — required for email links |
| `STORAGE_BACKEND` | `file` | Set to `mongo` for MongoDB persistence |
| `MONGODB_URI` | — | MongoDB connection string |
| `MONGODB_DATABASE` | `healthops` | MongoDB database name |
| `DATA_DIR` | `data/` | Data directory for file storage |
| `CORS_ORIGIN` | — | CORS origin for custom domains |

See the [Deployment Guide](docs/deployment-guide.md) for full details.

---

## Testing

```bash
cd backend && go test ./... -race   # Backend tests with race detector
cd frontend && npm run typecheck     # Frontend type check
```

---

## Security

- JWT auth with role-based access (admin/viewer)
- Bcrypt passwords; login rate-limited (10 attempts/min per IP)
- Security headers on all responses (CSP, X-Frame-Options, etc.)
- AES-256-GCM encryption for AI API keys at rest
- SSH host key fingerprint verification (set `hostKeyFingerprint` in SSH checks)
- Command checks disabled by default

Found a vulnerability? See [SECURITY.md](SECURITY.md) for private disclosure.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23, `net/http` stdlib |
| Frontend | React 19, TypeScript, Vite 6, Tailwind CSS |
| Charts | Recharts |
| State | TanStack React Query |
| Storage | File-based (primary) + MongoDB (optional) |
| Monitoring | Prometheus client, SSE |
| Container | Docker multi-stage build |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). We welcome bug fixes, new check types, notification channels, and documentation improvements.

Report security issues privately — see [SECURITY.md](SECURITY.md).

---

## License

MIT — see [LICENSE](LICENSE).
