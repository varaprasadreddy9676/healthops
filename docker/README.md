# Docker Deployment

This guide covers deploying HealthOps using Docker and Docker Compose.

## Prerequisites

- Docker (20.10+)
- Docker Compose (2.0+)

## Quick Start

### 1. Build the Docker image

```bash
docker build -t healthops .
```

### 2. Configure first-run admin and ports

```bash
cp .env.example .env
```

Edit `.env` before first start:

- Set `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` to a strong temporary admin password.
- Leave `HEALTHOPS_BOOTSTRAP_ADMIN_RESET=false` for normal starts. Set it to `true` only when intentionally resetting the admin password.
- Change `HEALTHOPS_PORT` or `MONGODB_PORT` if those host ports are already in use.

Mongo-backed deployments do not create an insecure `admin/admin` user automatically.

### 3. Start the stack

```bash
docker compose up -d
```

This starts:
- **healthops** service on port 8080 (Go backend + React frontend)
- **MongoDB** on port 27017

### 3b. Start the full demo stack

For open-source evaluation, use the scripted demo. It starts HealthOps plus realistic monitoring targets, emits logs, creates MySQL activity, and gives you scenario commands to trigger real incidents.

```bash
cp .env.demo.example .env.demo
scripts/demo-up.sh
```

Open the URL printed by the script and log in with `admin` / `healthops-demo-admin`. The default URL is `http://localhost:18080`; the script will pick another free port if needed.

This adds:
- **HealthOps** on port 18080 by default (override `HEALTHOPS_PORT`)
- **nginx** internally (web server with health endpoint)
- **MySQL** on port 13306 by default (for MySQL deep monitoring; override `MYSQL_PORT`)
- **Redis** internally (cache, TCP check)
- **echo-server** internally (simple API endpoint)
- **demo-api** on port 19100 by default (controllable checkout API; override `DEMO_API_PORT`)
- **demo-ai-provider** internally (local OpenAI-compatible provider for AI brief and RCA demos)
- **demo-log-emitter** (posts realistic application, worker, gateway, and MySQL errors into HealthOps)
- **mysql-workload** (keeps MySQL samples, queries, and thread views populated)
- **linux-server-1** on SSH port 12222 by default (nginx + python HTTP server + cron)
- **linux-server-2** on SSH port 12223 by default (nginx + python HTTP server + cron)

The Linux servers run real processes (nginx, python3 http.server, cron) that HealthOps monitors remotely via SSH process checks.

All demo services come pre-configured with health checks in `backend/config/demo.json`.

Trigger scenarios:

```bash
scripts/demo-scenario.sh api-slow
scripts/demo-scenario.sh api-down
scripts/demo-scenario.sh api-flaky
scripts/demo-scenario.sh log-spike
scripts/demo-scenario.sh mysql-load
scripts/demo-scenario.sh rca
scripts/demo-scenario.sh recover
```

AI/RCA runs out of the box against the local demo AI provider. Optional BYOK setup with OpenRouter:

```bash
# The script sends the key once to HealthOps, where it is stored encrypted in MongoDB.
OPENROUTER_API_KEY=sk-or-v1-... scripts/demo-configure-ai.sh
```

### 4. View logs

```bash
# Follow all logs
docker compose logs -f

# Follow specific service logs
docker compose logs -f healthops
docker compose logs -f mongo
```

### 5. Verify deployment

```bash
# Check service health
curl http://localhost:8080/healthz

# View readiness status
curl http://localhost:8080/readyz

# Get health summary
curl http://localhost:8080/api/v1/summary
```

## Configuration

### Environment Variables

Create a `.env` file from the example:

```bash
cp .env.example .env
```

Edit `.env` to customize:

- **HEALTHOPS_PORT** - Host port for the HealthOps service (default: 8080)
- **MONGODB_PORT** - Host port for MongoDB loopback publishing (default: 27017)
- **HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD** - Strong temporary password used to create/reset the Mongo-backed `admin` user
- **HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL** - Admin email used during bootstrap
- **HEALTHOPS_BOOTSTRAP_ADMIN_RESET** - Set to `true` only when intentionally resetting the admin user
- **AUTH_ENABLED / AUTH_USERNAME / AUTH_PASSWORD** - Legacy Basic auth settings. UI login uses the Mongo-backed user store.
- **MONGODB_URI** - MongoDB connection string
- **MONGODB_DATABASE** - Database name (default: healthops)
- **MONGODB_COLLECTION_PREFIX** - Collection prefix (default: healthops)
- **STATE_PATH** - Path to local state file
- **CONFIG_PATH** - Path to config file

### Volume Mounts

The base `docker-compose.yml` uses named volumes:
- `mongo_data:/data/db` - MongoDB primary persistence
- `healthops_data:/app/data` - local fallback state, logs, generated secrets, queues, and other runtime files

Back up both volumes for a complete restore.

## Management Commands

### Stop services

```bash
docker compose down
```

### Stop and remove volumes

```bash
docker compose down -v
```

### Restart services

```bash
docker compose restart
```

### Rebuild after code changes

```bash
docker compose up -d --build
```

### Execute commands in container

```bash
# Open shell in healthops container
docker compose exec healthops sh

# View config
docker compose exec healthops cat /app/config/default.json

# Check state
docker compose exec healthops cat /app/data/state.json
```

## Production Considerations

1. **Change default credentials** - Update AUTH_PASSWORD in production
2. **Enable authentication** - Set AUTH_ENABLED=true
3. **Secure MongoDB** - Configure MongoDB authentication for production and update `MONGODB_URI`
4. **Resource limits** - Review the memory/CPU limits in `docker-compose.yml` for your workload
5. **Log aggregation** - Configure centralized logging
6. **Health checks** - Use /healthz and /readyz for orchestration
7. **Backup strategy** - Regularly backup both data volume and MongoDB

## Troubleshooting

### Service fails to start

```bash
# Check logs
docker compose logs healthops

# Verify config
docker compose exec healthops cat /app/config/default.json
```

### MongoDB connection issues

```bash
# Check MongoDB is running
docker compose ps

# Test MongoDB connection
docker compose exec mongo mongosh --eval "db.adminCommand('ping')"
```

### Data persistence issues

```bash
# Check volume mounts
docker compose exec healthops ls -la /app/data

# Verify data directory permissions
docker compose exec healthops id
```

## Network Access

By default, services are exposed on:
- **Health Monitor API**: http://localhost:8080
- **MongoDB**: mongodb://localhost:27017

For production deployment, consider:
- Using reverse proxy (nginx/traefik)
- Enabling HTTPS/TLS
- Restricting MongoDB to internal network only
- Configuring proper firewall rules

## Upgrading

1. Pull latest code
2. Rebuild image: `docker compose build`
3. Restart with new image: `docker compose up -d`
4. Monitor logs: `docker compose logs -f`

The data volumes persist across upgrades.
