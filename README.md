<div align="center">

# HealthOps

**Self-hosted infrastructure monitoring with automatic AI root-cause analysis.**

[![License](https://img.shields.io/github/license/varaprasadreddy9676/healthops?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)](https://react.dev)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat-square&logo=docker&logoColor=white)](https://github.com/varaprasadreddy9676/healthops/pkgs/container/healthops)
[![MongoDB](https://img.shields.io/badge/MongoDB-47A248?style=flat-square&logo=mongodb&logoColor=white)](https://www.mongodb.com)

Monitor servers, APIs, databases, and services. Get alerted when things break.
**AI writes the root-cause analysis before you open the dashboard.**

One `docker compose up`. No agents. No per-host fees. No seat limits.

[Try the Demo](#try-the-demo) · [Production Setup](#production-setup) · [API Docs](backend/docs/api-reference.md) · [Deployment Guide](docs/deployment-guide.md)

</div>

---

## Screenshots

<table>
<tr>
<td width="50%"><img src="docs/screenshots/dashboard.png" alt="Dashboard" /></td>
<td width="50%"><img src="docs/screenshots/checks.png" alt="Health Checks" /></td>
</tr>
<tr>
<td><b>Dashboard</b> — real-time overview of all checks, servers, and incidents</td>
<td><b>Health Checks</b> — severity-sorted list with response time indicators</td>
</tr>
<tr>
<td width="50%"><img src="docs/screenshots/incidents.png" alt="Incidents" /></td>
<td width="50%"><img src="docs/screenshots/ai-analysis.png" alt="AI Analysis" /></td>
</tr>
<tr>
<td><b>Incidents</b> — lifecycle management with MTTA/MTTR metrics</td>
<td><b>AI Analysis</b> — automatic root-cause analysis on every incident</td>
</tr>
<tr>
<td width="50%"><img src="docs/screenshots/servers.png" alt="Servers" /></td>
<td width="50%"><img src="docs/screenshots/mysql.png" alt="MySQL Monitoring" /></td>
</tr>
<tr>
<td><b>Servers</b> — agentless SSH monitoring (CPU, memory, disk, load)</td>
<td><b>MySQL</b> — deep monitoring with 30+ status variables and 9 alert rules</td>
</tr>
<tr>
<td width="50%"><img src="docs/screenshots/analytics.png" alt="Analytics" /></td>
<td width="50%"><img src="docs/screenshots/settings.png" alt="Settings" /></td>
</tr>
<tr>
<td><b>Analytics</b> — uptime, response time trends, failure rate analysis</td>
<td><b>Settings</b> — checks, users, AI providers, notifications, alert rules</td>
</tr>
</table>

---

## Why HealthOps

| | Datadog | New Relic | Nagios/Zabbix | **HealthOps** |
|---|:---:|:---:|:---:|:---:|
| HTTP / API checks | Yes | Yes | Yes | Yes |
| MySQL deep monitoring | Agent | Agent | Limited | **Agentless** |
| SSH server monitoring | No | No | Complex | **Modern UI** |
| Auto AI root-cause analysis | Enterprise | Enterprise | No | **Free (BYOK)** |
| Self-hosted | No | No | Yes | **Yes** |
| Cost (5-person team) | $300-500/mo | $250/mo | Your time | **$6-10/mo** |

---

## Quick Start

### Try the Demo

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops
docker compose -f compose.demo.yaml up -d --build
```

Open **http://localhost:18080** — log in with `admin` / `healthops-demo-admin`

The demo spins up a full stack: HealthOps + MongoDB + MySQL + Redis + nginx + 2 Linux SSH targets + a controllable API + log emitter + **local AI provider** (no API key needed).

```bash
# Trigger failure scenarios
scripts/demo-scenario.sh api-down     # API outage → incident + AI analysis
scripts/demo-scenario.sh mysql-load   # MySQL stress → alert + AI diagnosis
scripts/demo-scenario.sh recover      # Everything recovers, resolution alerts fire

# Stop and clean up
docker compose -f compose.demo.yaml down -v
```

### Production Setup

```bash
# 1. Configure
cat > .env << 'EOF'
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD=choose-a-strong-password
# HEALTHOPS_PUBLIC_URL=https://healthops.yourcompany.com
EOF

# 2. Start
docker compose up -d --build

# 3. Verify
curl http://localhost:8080/healthz
# → {"success":true,"data":{"status":"ok"}}
```

Open **http://localhost:8080**, log in with `admin` / your password.

See the [Deployment Guide](docs/deployment-guide.md) for TLS, reverse proxy, and production hardening.

---

## Check Types

| Type | What it monitors | Example |
|------|-----------------|---------|
| `api` | HTTP/HTTPS endpoints — status code, body, latency | `https://api.myapp.com/health` |
| `tcp` | Port connectivity and response time | `db.internal:5432` |
| `process` | Process existence by name | `nginx` |
| `command` | Shell command exit code and output | `df -h \| awk '$5>90'` |
| `log` | Log file modification recency | `/var/log/app.log` |
| `mysql` | 30+ MySQL status variables, delta rates, 9 alert rules | DSN via env var |
| `ssh` | Remote server CPU, memory, disk, load over SSH | Host + SSH key |

---

## AI Root-Cause Analysis

Every incident automatically triggers AI analysis — no manual action needed.

```
Check fails → Incident created → Evidence captured → AI enqueued
    → AI reads check type, failure message, metrics, evidence snapshot
    → AI produces: what failed, probable root cause, what to fix
    → Analysis ready in the UI before you open the dashboard
```

### Bring any AI provider

| Provider | Cost | Privacy |
|----------|------|---------|
| **Ollama** | Free | 100% local — no data leaves your server |
| OpenAI | ~$0.01-0.05/analysis | OpenAI's policy |
| Anthropic | ~$0.01-0.05/analysis | Anthropic's policy |
| Google Gemini | ~$0.01/analysis | Google's policy |
| Custom | Varies | Any OpenAI-compatible API |

API keys are encrypted with AES-256-GCM at rest.

**Set up Ollama (free, local):**
```bash
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3
# In HealthOps: Settings → AI → Add Provider → Ollama → http://localhost:11434
```

### What the AI sees

When an incident fires, the AI receives full context: check type and target, the exact error message, response latency vs baseline, a snapshot of system state at failure time (MySQL variables, process list, disk usage), and historical patterns. It produces targeted analysis — not generic "check your logs" advice.

**MySQL example:**
```
Connections: 245/256 (95.7%)     ← near max_connections
Slow queries: 847/sec            ← 400x normal
Lock wait time: 12,400ms avg     ← writes blocking reads
Buffer pool hit: 62%             ← cache thrashing (normally 98%)
```

AI output: *"Connection pool near exhaustion. Long-running transaction holding locks causing downstream queries to pile up. Run SHOW PROCESSLIST to identify the blocking query."*

---

## SSH Server Monitoring

**Agentless** — nothing installed on the target. If you can SSH in, you can monitor it.

Collected every interval:
- CPU usage (from `/proc/stat`)
- Memory (total, used, %)
- Disk (total, used, %)
- Load averages (1m, 5m, 15m)
- Top 15 processes by memory
- Disk I/O (read/write IOPS)
- Uptime

Built-in thresholds: CPU >80% = warning, >95% = critical. Memory >85% = warning. Disk >85% = warning. All configurable.

---

## Alerting & Notifications

**6 channels:** Slack, Email, Discord, Telegram, PagerDuty, Webhooks

- Smart deduplication — one alert per incident, not one per check run
- Severity-based routing and tag filters
- Cooldown periods to prevent alert fatigue
- Resolution alerts when incidents clear
- Test button to verify before going live

**Slack setup (3 steps):**
1. Create an Incoming Webhook at [api.slack.com/apps](https://api.slack.com/apps)
2. In HealthOps: Notifications → Add Channel → Slack → paste URL
3. Click **Test** → **Save**

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    React Frontend                        │
│         (Vite + TypeScript + Tailwind + Recharts)        │
└───────────────────────┬─────────────────────────────────┘
                        │ REST API + SSE
┌───────────────────────┴─────────────────────────────────┐
│                    Go Backend                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐  │
│  │Scheduler │  │ Runner   │  │ Incident │  │  AI    │  │
│  │(per-check│  │(7 types) │  │ Manager  │  │Service │  │
│  │ timers)  │  │          │  │          │  │(BYOK)  │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───┬────┘  │
│       └──────────────┴─────────────┴─────────────┘       │
│  ┌───────────────────────────────────────────────────┐   │
│  │                  MongoDB Store                    │   │
│  └───────────────────────────────────────────────────┘   │
│  JWT Auth · Users · Notifications · Alert Rules           │
│  Audit · Prometheus · AI Queue · Deduplication            │
└─────────────────────────────────────────────────────────┘
```

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23, `net/http` stdlib |
| Frontend | React 19, TypeScript, Vite 6, Tailwind CSS, Recharts |
| State | TanStack React Query |
| Storage | MongoDB |
| Observability | Prometheus metrics (`/metrics`), SSE real-time updates |
| Container | Docker multi-stage build |

---

## Resource Usage

| Setup | RAM | CPU (idle) | Disk/month |
|-------|-----|-----------|------------|
| 20 checks | ~60 MB | <1% | <100 MB |
| 50 checks + MongoDB | ~120 MB | <2% | ~1 GB |
| 100 checks + MongoDB | ~180 MB | <3% | ~2 GB |

A $6/month VPS (Hetzner CX11, DigitalOcean Droplet, Linode Nanode) runs 100+ checks with room to spare.

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | — | Admin password for first run |
| `HEALTHOPS_PUBLIC_URL` | — | Public URL (required for email links) |
| `MONGODB_URI` | — | MongoDB connection string |
| `MONGODB_DATABASE` | `healthops` | Database name |
| `DATA_DIR` | `data/` | Encryption keys and JWT secrets |
| `CORS_ORIGIN` | — | CORS origin for custom domains |

See the [Deployment Guide](docs/deployment-guide.md) for all options.

---

## Project Layout

```
healthops/
├── backend/               # Go service (cmd/healthops = entrypoint)
│   ├── cmd/healthops/     # main.go
│   ├── internal/          # Core packages
│   ├── config/            # Seed configs
│   └── docs/              # API reference
├── frontend/              # React + TypeScript + Vite
├── docker/                # Demo containers, init scripts
├── docs/                  # Guides, ADRs, screenshots, OpenAPI spec
├── compose.yaml           # Production
├── compose.demo.yaml      # Demo with full stack
└── Dockerfile             # Multi-stage build
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Deployment Guide](docs/deployment-guide.md) | Docker, bare metal, TLS, reverse proxy, SSH, MySQL |
| [API Reference](backend/docs/api-reference.md) | 62+ REST endpoints with examples |
| [OpenAPI Spec](docs/openapi.yaml) | Machine-readable OpenAPI 3.0 |
| [Runbook](docs/runbook.md) | Operations, backup/restore, performance tuning |
| [Architecture Decisions](docs/decisions/) | ADRs for persistence, auth, incidents, AI |

---

## Security

- JWT auth with role-based access (admin / ops)
- Bcrypt passwords, login rate-limited (10 attempts/min per IP)
- Security headers (CSP, X-Frame-Options, HSTS, Referrer-Policy)
- AES-256-GCM encryption for AI API keys at rest
- SSH host key fingerprint verification
- Command checks disabled by default

Found a vulnerability? See [SECURITY.md](SECURITY.md).

---

## Development

```bash
# Backend
cd backend && go test ./... -race
cd backend && go fmt ./...

# Frontend
cd frontend && npm run typecheck
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — we welcome new check types, notification channels, AI prompt improvements, and bug fixes.

---

## License

[MIT](LICENSE)
