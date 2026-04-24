# Production Deployment Guide

**Audience:** Operators deploying HealthOps to a production environment.
**Related docs:** [SLO definitions](slo.md) · [Backups & DR](backups.md) · [Runbook](runbook.md) · [Security audit](../backend/docs/security-audit.md)

This guide covers two supported topologies: a single static binary behind a
reverse proxy, and the bundled `docker-compose.yml`. Both assume the service
is exposed only through a TLS-terminating reverse proxy and that the `data/`
directory lives on persistent, backed-up storage.

## 1. Prerequisites

- Linux host (kernel 4.x+) or any Docker-compatible container platform.
- Persistent volume mounted at the path you choose for `DATA_DIR` (default in
  containers: `/app/data`). The volume must:
  - survive container/host restarts,
  - be on the daily backup tier (see [backups.md](backups.md)),
  - have file mode `0700` (owner-only) — it stores credentials, JWT secret,
    and the AI encryption key.
- Optional: MongoDB 6+ if you want a mirrored read replica for dashboards.
  HealthOps treats Mongo as a best-effort mirror, not the source of truth.
- Reverse proxy with TLS (nginx, Caddy, AWS ALB, Cloudflare). The Go process
  speaks plain HTTP and must never be exposed directly to the internet.
- Outbound network access from the host to: any monitored targets, your
  notification destinations (SMTP/webhooks), and your AI provider if BYOK is
  enabled.

## 2. Required Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `HEALTHOPS_REQUIRE_PROD_AUTH` | Yes (prod) | Set to `true`. Refuses to start with default admin credentials or `allowCommandChecks=true`. |
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | Yes (first run) | Strong unique password for the bootstrap admin user. Required when using the Mongo user store. |
| `HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL` | No | Email for the bootstrap admin. Default `admin@healthops.local`. |
| `CONFIG_PATH` | No | Path to the seed config (first run only). Default `backend/config/default.json`. |
| `STATE_PATH` | No | Path to `state.json`. Default `backend/data/state.json`. |
| `DATA_DIR` | No | Directory for JSONL stores, users.json, .ai_enc_key. Default `backend/data`. |
| `MONGODB_URI` | No | Enables Mongo mirroring. Omit for file-only storage. |
| `MONGODB_DATABASE` | No | Mongo DB name. Default `healthops`. |
| `MONGODB_COLLECTION_PREFIX` | No | Collection prefix. Default `healthops`. |
| `FRONTEND_DIR` | No | Path to built frontend assets. Default `frontend/dist`. |

After the first successful start, `data/state.json` becomes the source of
truth. Edits to `default.json` are ignored on subsequent runs — manage checks
through the API or UI.

## 3. Volume Layout

The persistent volume mounted as `DATA_DIR` must contain (or be allowed to
create):

```
/data/
  state.json                 # Checks + recent results — primary source of truth
  users.json                 # File-backed user store (when no Mongo URI)
  .ai_enc_key                # AES-256-GCM key for AI provider credentials
  .jwt_secret                # JWT signing secret
  ai_config.json             # Encrypted AI provider config
  ai_queue.jsonl             # Pending AI analysis jobs
  ai_results.jsonl           # AI analysis history
  alert_rules.json           # Alert rule definitions
  audit.json                 # Mutating-action audit trail
  incident_snapshots.jsonl   # Incident evidence captures
  notification_outbox.jsonl  # Alert delivery attempts
  mysql_samples.jsonl        # MySQL collector samples
  mysql_deltas.jsonl
  mysql_rule_states.json
  server_metrics/            # Per-server JSONL series
```

Hard requirements:

- The directory must be on persistent storage (not container ephemeral FS,
  not `/tmp`).
- It must be backed up daily — see [backups.md](backups.md).
- Permissions must be `0700` and owned by the service user. World-readable
  permissions leak the JWT secret, AI encryption key, and password hashes.
- `.ai_enc_key` must be backed up **separately and securely**. Losing it
  permanently destroys the ability to decrypt `ai_config.json`.

## 4. Topology A — Single Binary + systemd + nginx

### 4.1 Build

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops/backend
go build -o /usr/local/bin/healthops ./cmd/healthops

# Frontend (optional — only if serving the UI from the same host)
cd ../frontend
npm ci && npm run build
sudo mkdir -p /opt/healthops
sudo cp -r dist /opt/healthops/frontend-dist
```

### 4.2 Filesystem layout

```bash
sudo useradd --system --home /var/lib/healthops --shell /usr/sbin/nologin healthops
sudo mkdir -p /var/lib/healthops/data /etc/healthops
sudo cp backend/config/default.json /etc/healthops/config.json
sudo chown -R healthops:healthops /var/lib/healthops /etc/healthops
sudo chmod 700 /var/lib/healthops/data
```

### 4.3 systemd unit `/etc/systemd/system/healthops.service`

```ini
[Unit]
Description=HealthOps Monitoring Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=healthops
Group=healthops
WorkingDirectory=/var/lib/healthops
ExecStart=/usr/local/bin/healthops
Restart=on-failure
RestartSec=5s

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/healthops
CapabilityBoundingSet=
AmbientCapabilities=

# Required env
Environment=CONFIG_PATH=/etc/healthops/config.json
Environment=STATE_PATH=/var/lib/healthops/data/state.json
Environment=DATA_DIR=/var/lib/healthops/data
Environment=FRONTEND_DIR=/opt/healthops/frontend-dist
Environment=HEALTHOPS_REQUIRE_PROD_AUTH=true
EnvironmentFile=/etc/healthops/healthops.env

[Install]
WantedBy=multi-user.target
```

Put secrets in `/etc/healthops/healthops.env` (mode `0600`, owned by
`healthops`):

```bash
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD=<generate-strong-password>
HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL=ops@example.com
# Optional Mongo mirror
# MONGODB_URI=mongodb://healthops:<pw>@mongo.internal:27017
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now healthops
sudo systemctl status healthops
```

### 4.4 nginx server block

```nginx
server {
    listen 443 ssl http2;
    server_name healthops.example.com;

    ssl_certificate     /etc/ssl/certs/healthops.crt;
    ssl_certificate_key /etc/ssl/private/healthops.key;
    ssl_protocols TLSv1.2 TLSv1.3;

    # Hardening headers
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin" always;

    client_max_body_size 1m;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }

    # SSE endpoint needs streaming
    location /api/v1/events {
        proxy_pass         http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_buffering    off;
        proxy_cache        off;
        proxy_read_timeout 1h;
    }
}

server {
    listen 80;
    server_name healthops.example.com;
    return 301 https://$host$request_uri;
}
```

## 5. Topology B — Docker Compose

The repo ships [`docker-compose.yml`](../docker-compose.yml) building from the
root [`Dockerfile`](../Dockerfile). For production you should override:

- `image:` — pin to a built tag from your registry, do not build on the host.
- `environment:` — inject prod env vars from a secrets manager.
- `ports:` — bind to `127.0.0.1:8080` only and front with a reverse proxy.
- `volumes:` — mount a host path or named volume that is on durable storage
  and included in your backup job.

Sample `docker-compose.prod.yml` overlay:

```yaml
services:
  healthops:
    image: registry.example.com/healthops:1.0.0
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      - HEALTHOPS_REQUIRE_PROD_AUTH=true
      - HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD=${HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD}
      - HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL=ops@example.com
      - MONGODB_URI=mongodb://mongo:27017
    volumes:
      - /srv/healthops/data:/app/data
    restart: unless-stopped

  mongo:
    image: mongo:7
    ports: []   # do not publish in prod
    volumes:
      - /srv/healthops/mongo:/data/db
    restart: unless-stopped
```

Deploy:

```bash
export HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD="$(openssl rand -base64 24)"
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

Front the published port with the same nginx server block from §4.4.

## 6. First-Run Bootstrap

1. Generate a strong admin password and put it in
   `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD`.
2. Set `HEALTHOPS_REQUIRE_PROD_AUTH=true`. The service refuses to start
   if it detects default credentials or `allowCommandChecks=true`.
3. Confirm `allowCommandChecks` is `false` in `config/default.json` (it is by
   default). Do not enable it via the API in production — shell command checks
   are an RCE risk.
4. Start the service. Watch logs for
   `HEALTHOPS_REQUIRE_PROD_AUTH=true: production hardening checks passed`.
5. Open the UI through the reverse proxy, log in with the bootstrap admin.
6. Rotate the bootstrap password through the UI/API, then unset
   `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` from the environment file (or set
   `HEALTHOPS_BOOTSTRAP_ADMIN_RESET=false`, the default) so it is not
   re-applied on the next start.
7. Restart the service to confirm it still starts with
   `HEALTHOPS_REQUIRE_PROD_AUTH=true` after the bootstrap variable is gone.

## 7. Smoke Tests

Run from a host that can reach the reverse proxy:

```bash
BASE=https://healthops.example.com

# Liveness — must succeed without auth
curl -fsS "$BASE/healthz"

# Readiness — must succeed without auth
curl -fsS "$BASE/readyz"

# Login — capture session cookie / JWT
curl -fsS -c cookies.txt -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops@example.com","password":"'"$ADMIN_PW"'"}'

# List checks — must succeed when authenticated
curl -fsS -b cookies.txt "$BASE/api/v1/checks" | jq '.data | length'

# Prometheus metrics endpoint
curl -fsS "$BASE/metrics" | head -20
```

A successful deployment shows:
- `/healthz` returns `{"status":"ok","success":true}`.
- `/readyz` returns `success: true` with the configured check count.
- `/api/v1/checks` returns a JSON array (possibly empty for a clean install).
- `/metrics` exposes `healthops_*` counters and histograms.

## 8. Rollback

Rollback is a four-step operation. The persistent `data/` directory is the
unit of state; the binary/image is stateless.

```bash
# 1. Stop the running service
sudo systemctl stop healthops      # or: docker compose stop healthops

# 2. Restore data/ from the most recent good backup (see backups.md)
sudo rm -rf /var/lib/healthops/data
sudo tar -xzf /var/backups/healthops/data-2026-04-23.tar.gz \
  -C /var/lib/healthops
sudo chown -R healthops:healthops /var/lib/healthops/data
sudo chmod 700 /var/lib/healthops/data

# 3. Pin the previous binary or image
sudo cp /usr/local/bin/healthops.previous /usr/local/bin/healthops
# or: docker compose pull healthops:<previous-tag>

# 4. Start and re-run smoke tests from §7
sudo systemctl start healthops
curl -fsS https://healthops.example.com/healthz
```

If the rollback target is older than the last admin-password rotation, you
will need to log in with whichever password was current at the time of the
backup. After verifying the rollback, rotate the password again immediately.

See [runbook.md](runbook.md) for incident playbooks once the service is
running.
