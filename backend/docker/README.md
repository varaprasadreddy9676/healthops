# Docker Deployment

This guide covers deploying the Medics Health Check service using Docker and Docker Compose.

## Prerequisites

- Docker (20.10+)
- Docker Compose (2.0+)

## Quick Start

### 1. Build the Docker image

```bash
docker build -t healthops .
```

### 2. Start the full stack

```bash
docker compose up -d
```

This starts:
- **healthops** service on port 8080
- **MongoDB** on port 27017

### 3. View logs

```bash
# Follow all logs
docker compose logs -f

# Follow specific service logs
docker compose logs -f healthops
docker compose logs -f mongo
```

### 4. Verify deployment

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

1. **Bootstrap admin credentials** - Set `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` before first start, then rotate it after login
2. **Enable authentication** - Set `AUTH_ENABLED=true`
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
