# HealthOps

> **AI-native infrastructure monitoring that runs on a $6/month server and replaces tools costing $300–500/month.**

[![License](https://img.shields.io/github/license/varaprasadreddy9676/healthops)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/varaprasadreddy9676/healthops/ci.yml?label=CI)](https://github.com/varaprasadreddy9676/healthops/actions)
[![Docker](https://img.shields.io/badge/Docker-ready-blue)](https://github.com/varaprasadreddy9676/healthops/pkgs/container/healthops)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8)](https://golang.org)

HealthOps monitors your servers, APIs, databases, and services — alerts you the moment something breaks, **automatically runs AI root-cause analysis**, and tells you exactly what to fix. All self-hosted. All open source.

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

## What makes HealthOps different

Most monitoring tools tell you **that** something broke. HealthOps tells you **why** — automatically, before you even open the dashboard.

| Capability | Datadog | New Relic | PagerDuty | Nagios/Zabbix | **HealthOps** |
|-----------|---------|-----------|-----------|---------------|---------------|
| HTTP/API checks | ✅ | ✅ | ❌ | ✅ | ✅ |
| MySQL deep monitoring | ✅ (agent) | ✅ (agent) | ❌ | ⚠️ limited | ✅ agentless |
| SSH agentless server monitoring | ❌ | ❌ | ❌ | ✅ complex config | ✅ modern UI |
| Auto AI incident analysis | 💰 enterprise only | 💰 enterprise only | 💰 enterprise only | ❌ | ✅ free (BYOK) |
| Auto RCA on every incident | ❌ | ❌ | ❌ | ❌ | ✅ |
| Self-hosted / data sovereignty | ❌ | ❌ | ❌ | ✅ | ✅ |
| Runs on $6/month VPS | ❌ | ❌ | ❌ | ⚠️ complex | ✅ |
| Monthly cost for 5-person team | $300–500 | $250 | $100 (alerts only) | free (but your time) | **$6–10** |

---

## AI-native operations — not AI bolted on

Most tools treat AI as an upsell. HealthOps is built AI-first: every incident automatically triggers analysis, so by the time you open the dashboard, the answer is already there.

### How it works — zero clicks required

```
Check fails
    ↓
Alert rule triggers → Incident created + Evidence snapshot captured
    ↓
Incident auto-enqueued for AI analysis (no manual action needed)
    ↓
AI worker picks it up within 5 seconds
    ↓
AI reads: check type, target, failure message, response metrics,
          historical pattern, evidence snapshot at failure time
    ↓
AI produces: what failed, probable root cause, what to check next
    ↓
Analysis saved to incident detail view — ready when you open it
```

You wake up, open the incident, and the RCA is already written. Not a vague "something failed" — a specific, context-aware analysis of your actual infrastructure.

### Bring any AI provider — including free local models

| Provider | Cost | Privacy | Models |
|----------|------|---------|--------|
| **Ollama** | Free | 100% local, no data leaves your server | Llama 3, Mistral, Phi, any GGUF |
| OpenAI | ~$0.01–0.05 per analysis | OpenAI's policy | GPT-4o, GPT-4, GPT-3.5 |
| Anthropic | ~$0.01–0.05 per analysis | Anthropic's policy | Claude 3.5 Sonnet, Haiku |
| Google | ~$0.01 per analysis | Google's policy | Gemini 1.5 Flash/Pro |
| Custom | Varies | Your control | Any OpenAI-compatible API |

**Using Ollama means completely free, completely private AI analysis.** Run a local model on the same server as HealthOps — no API keys, no internet calls, no per-analysis cost.

API keys are stored encrypted with AES-256-GCM at rest.

### What the AI actually analyzes

When an incident fires, the AI receives full context:

- **Check details** — what type of check (API, MySQL, SSH, TCP), what target, what the expected behavior was
- **Failure message** — the exact error: HTTP 503, connection refused, process not found, replication lag 45s
- **Response metrics** — latency at failure time, how it compares to the check's normal baseline
- **Evidence snapshot** — a freeze-frame of your system state at the moment of failure: MySQL status variables, process list, disk usage — whatever was collected
- **Incident history** — has this check failed before? What pattern? First time, or recurring?

The AI uses all of this to produce a targeted analysis — not a generic "check your logs" response.

### Configurable AI prompts

You control exactly what the AI looks for. Edit the prompt template per check type:

- API checks: focus on upstream dependencies, DNS, certificate expiry
- MySQL checks: focus on connection exhaustion, slow query patterns, replication
- Process checks: focus on OOM kills, crash loops, dependency failures
- SSH server checks: focus on resource saturation, disk full, load spike causes

No need to re-explain your infrastructure to the AI every time — the prompt template carries that context automatically.

### AI for MySQL deep analysis

MySQL issues are notoriously hard to diagnose from a "database is slow" alert alone. HealthOps captures 30+ MySQL status variables at incident time and feeds them directly to the AI:

```
Connections: 245/256 (95.7% utilized)        ← near max_connections
Slow queries: 847/sec (normally ~2/sec)       ← 400x spike
Lock wait time: 12,400ms avg                  ← writes blocking reads
InnoDB buffer pool hit: 62% (normally 98%)    ← cache thrashing
```

The AI sees this snapshot and can say: *"Connection pool is near exhaustion — likely caused by a query holding locks which is causing downstream queries to pile up waiting. Check for long-running transactions with SHOW PROCESSLIST. Consider increasing innodb_lock_wait_timeout or finding the blocking query."*

That's a specific, actionable RCA — not "database might be slow."

---

## SSH server monitoring — agentless, modern UI

Most modern monitoring tools require installing an agent (a daemon process) on every server you want to monitor. This creates real problems:

- You need SSH access and sudo permissions just to set up monitoring
- Agents don't work on servers you don't control (client servers, managed hosting)
- Agents add processes, RAM usage, and maintenance burden to every server
- If the agent crashes, you're blind

**HealthOps monitors remote servers over SSH — nothing installed on the target.** If you can SSH in, you can monitor it.

What HealthOps collects over SSH every interval:
- **CPU usage** — computed from `/proc/stat` (two samples, accurate %)
- **Memory** — total, used, usage % from `free -b`
- **Disk** — total, used, usage % from `df -B1`
- **Load averages** — 1m, 5m, 15m from `/proc/loadavg`
- **Top processes** — top 15 by memory from `ps aux`
- **Disk I/O** — read/write IOPS from `/proc/diskstats`
- **Uptime** — seconds since boot

Built-in thresholds fire alerts automatically: CPU >80% = warning, >95% = critical. Memory >85% = warning. Disk >85% = warning. All configurable.

### Who has SSH monitoring?

| Tool | SSH monitoring | Setup experience |
|------|---------------|-----------------|
| Datadog | ❌ agent only | Install agent on every host |
| New Relic | ❌ agent only | Install agent on every host |
| UptimeRobot | ❌ | HTTP, port, ping only |
| Better Uptime | ❌ | HTTP, keyword, port only |
| Prometheus | ❌ | Install Node Exporter on every host |
| Nagios | ✅ | `check_by_ssh` plugin — config files, no UI |
| Zabbix | ✅ | SSH mode buried in complex interface |
| **HealthOps** | ✅ | Add SSH credentials in UI → monitoring starts |

Nagios and Zabbix have SSH checks but require writing config files by hand, navigating decade-old interfaces, and typically a dedicated engineer to manage. HealthOps gives you the same capability in a modern UI, deployed in one command.

---

## The problem with existing tools

| Tool | Monthly cost (5-person team) | What you get |
|------|------------------------------|--------------|
| Datadog | $300–500 | Monitoring + alerts. AI only on Enterprise. |
| New Relic | $250 | Monitoring + alerts. AI is extra. |
| PagerDuty | $100 | On-call routing only. No monitoring. |
| Better Uptime | $30–80 | Basic uptime + status page. No deep monitoring. |
| Nagios/Zabbix | Free software, $200+/month your time | Full monitoring, decade-old UX, no AI. |
| **HealthOps** | **$6–10 (your VPS)** | **Monitoring + SSH + MySQL + AI RCA + alerting** |

A $6/month Hetzner or DigitalOcean server runs HealthOps for 100+ checks. Unlimited users, unlimited checks, unlimited channels.

---

## Who uses HealthOps

### Startups and small teams who can't justify enterprise pricing

5–15 servers, a few APIs, a MySQL database. You need to know when things break — and why — without a $400/month contract. HealthOps runs on a small VPS next to your stack. Configure 30 checks in 20 minutes. When something breaks, the AI tells you why before you've finished reading the Slack alert.

### Freelancers and agencies managing multiple clients

Responsible for 10–30 client websites and servers. One dashboard shows everything. You know about the outage before the client calls. The AI analysis gives you something specific to say when they do. No per-site pricing — add as many clients as you want.

### DevOps engineers who hate noisy alerts and manual RCA

Alert fatigue is real. HealthOps groups failures into single incidents, deduplicates across channels, and respects cooldowns. You get one Slack message, not thirty. And when it fires, you open the incident and the root-cause analysis is already written. Less 2am debugging. Less "why did this happen again?"

### Self-hosted operators who care about privacy and compliance

Your monitoring data — and your AI analysis — stays on your servers. Use Ollama for completely local, private AI with no external API calls. GDPR, SOC 2, HIPAA-adjacent compliance is simpler when nothing leaves your network.

---

## Real-world scenarios

### "My checkout API went down at 2am. I didn't know for 45 minutes."

With HealthOps:
- API check detects failure within 60 seconds
- Incident created, evidence captured, Slack alert fires
- AI analyzes: response code, latency pattern, whether it's a total outage or degradation
- You wake up to: "Checkout API returning 503. Likely cause: upstream payment service timeout based on response pattern. Check payment-service logs and connection pool."

You fix it in minutes. Customers never notice.

---

### "Our MySQL is slow. We spent 3 hours on a call trying to figure out why."

HealthOps MySQL monitoring captures 30+ status variables every 30 seconds. When a threshold is breached:

- Alert fires with current connection count, slow query rate, lock wait time
- Incident created with a full MySQL snapshot as evidence
- AI analyzes the snapshot: "Connection utilization at 94%. Slow query rate spiked 400x at 03:42 UTC. InnoDB buffer pool hit rate dropped to 62%. A long-running transaction is likely holding locks and causing a pile-up. Run: `SHOW PROCESSLIST` to identify blocking queries."

3 hours of debugging → 10 minutes of reading.

---

### "We're manually SSHing into 6 servers every morning to check if things are running."

Add SSH-based server checks and process checks:

| What to check | Type | Config |
|--------------|------|--------|
| CPU / memory / disk on prod server | `ssh` | Host + SSH key |
| nginx process running | `process` | `nginx` |
| Background worker running | `process` | `worker.py` |
| Database port open | `tcp` | `db.internal:5432` |
| Application API healthy | `api` | `https://api.myapp.com/health` |

Dashboard shows everything green or red. If anything goes wrong overnight, you get a Slack alert — and an AI analysis — while it's still dark outside.

---

### "A client called to say their site is down. We had no idea."

Add an API check per client domain, 60-second interval. Optional: separate Slack channel per client.

When their site goes down, you know first. When it recovers, you get a resolution alert — no need to manually verify. The AI gives you a summary of what happened and how long it was down, ready to forward to the client.

---

### "We deployed a change. Response times degraded. We caught it 4 hours later."

Set `warningThresholdMs: 500` on your API checks. If your endpoint normally responds in 150ms and spikes to 800ms after a deploy, HealthOps fires a warning within 60 seconds.

The analytics page shows response time trends over 7 days with a timeline. You see the exact minute the degradation started, correlated with your deploy.

---

### "We're running Datadog, PagerDuty, and a custom Grafana dashboard. None of them talk to each other."

HealthOps replaces the whole stack:

| What you have | What HealthOps replaces it with |
|-------------|-------------------------------|
| UptimeRobot / Better Uptime | API, TCP, process checks with retry + cooldown |
| SSH + htop for server stats | SSH-based CPU, memory, disk, load monitoring |
| Manual MySQL inspection | MySQL deep monitoring, 9 alert rules, AI analysis |
| PagerDuty | Multi-channel notifications with smart dedup |
| Postmortem doc writing | Auto-generated AI incident analysis and RCA |
| Grafana dashboards | Built-in analytics with uptime, response time, failure rate |

One tool. One dashboard. One place to look at 3am.

---

## Why it runs on minimal infrastructure

Most monitoring tools run agents on every server. HealthOps is agentless:

- **No agent installation** — HealthOps reaches out over HTTP, TCP, SSH, or MySQL. Nothing runs on monitored servers except your existing SSH key.
- **Go binary** — single compiled binary, ~50–80MB RAM at rest. No JVM, no runtime, no interpreter.
- **Efficient scheduler** — per-check timers, not a global polling loop. 100 checks at 60-second intervals = 100 requests/minute. Trivial.
- **File-first storage** — works without MongoDB. Add it when you need team access and persistence across restarts.

**Measured resource usage:**

| Setup | RAM | CPU (idle) | Disk/month |
|-------|-----|-----------|------------|
| 20 checks, file storage | ~60 MB | <1% | <100 MB |
| 50 checks + MongoDB | ~120 MB | <2% | ~1 GB |
| 100 checks + MongoDB | ~180 MB | <3% | ~2 GB |

A $6/month Hetzner CX11 (2 vCPU, 2GB RAM) runs 100+ checks, MongoDB, and AI analysis with room to spare.

---

## Quick Start

### Option A — Try the Demo (2 minutes, no config)

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops
docker compose -f compose.demo.yaml up -d --build
```

Open **http://localhost:18080** — log in with `admin` / `healthops-demo-admin`.

The demo includes a full stack: HealthOps, MongoDB, MySQL, Redis, nginx, two Linux SSH targets, a controllable API, a log emitter, and a **built-in local AI provider**. AI analysis works immediately — no API key, no internet call, no cost.

Trigger realistic failure scenarios and watch AI RCA in action:

```bash
scripts/demo-scenario.sh api-down    # API goes offline → incident + AI analysis fires
scripts/demo-scenario.sh api-slow    # API slows → warning alert
scripts/demo-scenario.sh mysql-load  # MySQL under load → alert + AI MySQL analysis
scripts/demo-scenario.sh rca         # Trigger full AI root-cause analysis
scripts/demo-scenario.sh recover     # Everything goes green, resolution alerts fire
```

Stop and wipe:

```bash
docker compose -f compose.demo.yaml down -v
```

---

### Option B — Production Setup

**Step 1 — Create `.env`:**

```bash
cat > .env << 'EOF'
# Required: admin password for first login
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD=choose-a-strong-password-here

# Recommended: your public URL (email notification links won't work without this)
# HEALTHOPS_PUBLIC_URL=https://healthops.yourcompany.com
EOF
```

**Step 2 — Start:**

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops
docker compose up -d --build
```

**Step 3 — Open** http://localhost:8080, log in with `admin` / your password.

**Startup timeline:**
```
Building image...       (2–5 min first run, ~15 sec after)
Starting MongoDB...     (~10 seconds)
Starting HealthOps...   (~5 seconds)
Ready at http://localhost:8080
```

```bash
curl http://localhost:8080/healthz   # → {"success":true,"data":{"status":"ok"}}
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

### All check types

| Type | Use for | Example |
|------|---------|---------|
| `api` | HTTP endpoints, REST APIs, websites | `https://api.myapp.com/health` |
| `tcp` | Ports — databases, Redis, any service | `db.internal:5432` |
| `process` | Running processes | `nginx` |
| `command` | Custom scripts, disk checks | `df -h \| awk '$5>90'` |
| `log` | Log file freshness | `/var/log/app.log` |
| `mysql` | MySQL/MariaDB health + performance | DSN via env var |
| `ssh` | Remote server CPU, memory, disk, load | Host + SSH key |

---

## Set Up AI Analysis (5 minutes)

### Option 1: Ollama — free, local, private

```bash
# Install Ollama on your server
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3

# In HealthOps: Settings → AI Configuration → Add Provider
# Type: Ollama, URL: http://localhost:11434, Model: llama3
```

No API key. No cost. No data leaves your server.

### Option 2: OpenAI / Anthropic / Google

In HealthOps: **Settings** → **AI Configuration** → **Add Provider** → paste your API key → **Save**.

AI analysis starts automatically on the next incident. Cost: typically $0.01–0.05 per incident analysis.

---

## Set Up Slack Alerts in 3 Steps

1. [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **Incoming Webhooks** → create a webhook → copy the URL
2. HealthOps → **Settings** → **Notification Channels** → **Add Channel** → **Slack** → paste URL
3. Click **Test** → **Save**

Same flow for **Email**, **Discord**, **Telegram**, **PagerDuty**, and custom **Webhooks**.

---

## System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| RAM | 512 MB | 1–2 GB |
| Disk | 5 GB | 20 GB |
| Docker | 24.0+ | Latest |
| Docker Compose | v2.20+ | Latest |
| OS | Linux, macOS, Windows (WSL2) | Linux |

**Works on:** Hetzner CX11 ($6/mo) · DigitalOcean Droplet · Linode Nanode · AWS t3.micro · Any VPS with Docker

---

## Prerequisites

```bash
docker --version        # 24.x or later
docker compose version  # v2.x or later
```

Install Docker: [docs.docker.com/get-docker](https://docs.docker.com/get-docker/)

---

## Features

### Health Check Types

| Type | What it monitors |
|------|-----------------|
| `api` | HTTP/HTTPS — status code, response body, latency threshold |
| `tcp` | Port connectivity and response time |
| `process` | Process existence by name or command |
| `command` | Shell command exit code and output |
| `log` | Log file modification recency |
| `mysql` | 30+ MySQL status variables, delta rates, 9 alert rules |
| `ssh` | Remote CPU, memory, disk, load, top processes over SSH |

### AI-Powered Incident Analysis

- **Zero-click** — every incident auto-enqueued, no manual trigger
- **Context-aware** — AI sees check details, failure message, metrics, evidence snapshot
- **Root-cause analysis** — specific, actionable, not generic
- **MySQL AI analysis** — reads actual MySQL status variables at failure time
- **5 providers** — Ollama (free/local), OpenAI, Anthropic, Google, Custom
- **Configurable prompts** — customize per check type
- **Background processing** — AI queue runs async, never slows alerting
- **Persistent results** — analysis saved and browsable in the UI

### Notification Channels

- **6 channel types** — Email, Slack, Discord, Telegram, Webhooks, PagerDuty
- **Smart filters** — route by severity, check, type, server, tag
- **Deduplication** — one alert per incident, not one per check run
- **Digest mode** — multiple failures batched into one message
- **Test button** — verify before going live
- **Resolution alerts** — know when incidents clear automatically

### Incident Management

- Auto-created by configurable alert rules
- Full lifecycle: open → acknowledge → resolve
- Evidence snapshots captured at creation
- MTTA/MTTR analytics
- AI analysis attached to every incident

### Alert Rules

- 5 built-in rules out of the box
- Configurable thresholds, cooldowns, consecutive-breach logic
- Per-check or global scope
- Custom rules per check type

### MySQL Deep Monitoring

- 30+ status variables tracked every check interval
- Delta rate computation (queries/sec, connections/sec, etc.)
- 9 built-in alert rules: connection utilization, slow queries, lock waits, InnoDB buffer pool, replication lag, and more
- Dedicated dashboard: connections, queries, threads, server stats
- AI analysis on MySQL incidents with full status snapshot

### SSH Agentless Server Monitoring

- CPU, memory, disk, load, top processes — over existing SSH connection
- Nothing installed on the target server
- Works on any Linux server you can SSH into
- Built-in alert thresholds (CPU >80%, disk >85%, etc.)
- SSH host key fingerprint verification to prevent MitM attacks

### Analytics & Observability

- 7-day uptime per check
- Response time charts (p50/p95/p99)
- Failure rate trends
- Prometheus metrics at `/metrics`
- Audit log for all mutations
- Real-time SSE dashboard updates
- CSV/JSON export for incidents and results

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
│  │             (Primary persistence)                 │   │
│  └───────────────────────────────────────────────────┘   │
│  JWT Auth · Users · Notifications · Alert Rules           │
│  Audit · Prometheus · AI Queue · Deduplication            │
└─────────────────────────────────────────────────────────┘
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [Deployment Guide](docs/deployment-guide.md) | Docker, bare metal, TLS, reverse proxy, notifications, SSH, MySQL |
| [API Reference](backend/docs/api-reference.md) | All 62+ REST endpoints |
| [OpenAPI Spec](docs/openapi.yaml) | Machine-readable OpenAPI 3.0 |
| [Runbook](docs/runbook.md) | Day-to-day ops, backup/restore, performance tuning |
| [Architecture Decisions](docs/) | ADRs for persistence, auth, incidents, AI |

---

## Project Layout

```
healthops/
├── backend/                  # Go backend (cmd/healthops = entrypoint)
├── frontend/                 # React + TypeScript + Vite
├── docker/                   # Init scripts, workload generators
├── docs/                     # Guides, ADRs, screenshots
├── .github/                  # CI/CD workflows, issue templates
├── compose.yaml              # Production: HealthOps + MongoDB
├── compose.demo.yaml         # Demo with full realistic stack
└── Dockerfile                # Multi-stage build
```

---

## Configuration

### Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | — | Admin password for first run |
| `HEALTHOPS_PUBLIC_URL` | — | Public URL — required for email links |
| `MONGODB_URI` | — | MongoDB connection string (required) |
| `MONGODB_DATABASE` | `healthops` | MongoDB database name |
| `DATA_DIR` | `data/` | Data directory (encryption keys, JWT secrets) |
| `CORS_ORIGIN` | — | CORS origin for custom domains |

See the [Deployment Guide](docs/deployment-guide.md) for all options.

---

## Testing

```bash
cd backend && go test ./... -race   # backend
cd frontend && npm run typecheck     # frontend
```

---

## Security

- JWT auth with role-based access (admin/viewer)
- Bcrypt passwords; login rate-limited (10 attempts/min per IP)
- Security headers (CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy)
- AES-256-GCM encryption for AI API keys at rest
- SSH host key fingerprint verification
- Command checks disabled by default (`allowCommandChecks=false`)

Found a vulnerability? See [SECURITY.md](SECURITY.md).

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23, `net/http` stdlib |
| Frontend | React 19, TypeScript, Vite 6, Tailwind CSS |
| Charts | Recharts |
| State | TanStack React Query |
| Storage | MongoDB |
| Observability | Prometheus client, SSE |
| Container | Docker multi-stage build |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — we welcome new check types, notification channels, AI prompt improvements, and bug fixes.

Security issues: [SECURITY.md](SECURITY.md).

---

## License

MIT — see [LICENSE](LICENSE).
