# Health Monitoring Service Runbook

**Last Updated:** 2026-04-17

## 1. Startup

### Prerequisites

- **Go 1.25+** installed for source builds
- **MongoDB** - required for the production Docker stack and primary persistence
- **Basic tools:** `curl`, `ps`, `netstat`

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CONFIG_PATH` | No | `backend/config/default.json` | Path to configuration file |
| `MONGODB_URI` | Yes for production | - | MongoDB connection string for primary persistence |
| `MONGODB_DATABASE` | No | `healthops` | MongoDB database name |
| `MONGODB_COLLECTION_PREFIX` | No | `healthops` | MongoDB collection prefix |
| `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` | Yes on first Mongo-backed deployment | - | Strong temporary password used to create/reset the Mongo-backed `admin` user |
| `HEALTHOPS_BOOTSTRAP_ADMIN_EMAIL` | No | `admin@healthops.local` | Admin email used during bootstrap |
| `HEALTHOPS_BOOTSTRAP_ADMIN_RESET` | No | `false` | Set to `true` only when intentionally resetting the admin password |

For Docker/Mongo deployments, set `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` before the first start. HealthOps does not create default admin credentials automatically.

### Start the Service

```bash
# Navigate to backend directory
cd backend

# Start the monitoring service
MONGODB_URI=mongodb://localhost:27017 \
HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD='change-this-strong-password' \
go run ./cmd/healthops

# Alternative: Build and run
go build -o healthops ./cmd/healthops
MONGODB_URI=mongodb://localhost:27017 ./healthops
```

### Verify Startup

**Check health endpoint:**
```bash
curl http://localhost:8080/healthz
# Expected: {"status":"ok","success":true}
```

**Check readiness:**
```bash
curl http://localhost:8080/readyz
# Expected: {"success":true,"data":{"status":"ready","checks":<count>,"lastRunAt":null}}
```

**Check service status:**
```bash
# Check if process is running
ps aux | grep healthops

# Check port binding
netstat -an | grep 8080
```

## 2. Configuration

### Config File Structure

```json
{
  "server": {
    "addr": ":8080",
    "readTimeoutSeconds": 10,
    "writeTimeoutSeconds": 10,
    "idleTimeoutSeconds": 60
  },
  "auth": {
    "enabled": false,
    "username": "admin",
    "password": "securepassword"
  },
  "retentionDays": 7,
  "checkIntervalSeconds": 60,
  "workers": 8,
  "allowCommandChecks": false,
  "checks": [...]
}
```

### Check Types

#### API Checks
```json
{
  "id": "prod-api",
  "name": "Production API",
  "type": "api",
  "server": "prod-1",
  "application": "medics",
  "target": "https://example.com/health",
  "expectedStatus": 200,
  "timeoutSeconds": 5,
  "warningThresholdMs": 1000,
  "intervalSeconds": 120,
  "retryCount": 3,
  "retryDelaySeconds": 5,
  "cooldownSeconds": 30,
  "enabled": true,
  "tags": ["api", "prod"]
}
```

#### TCP Checks
```json
{
  "id": "database-port",
  "name": "Database Port Check",
  "type": "tcp",
  "server": "prod-1",
  "application": "database",
  "host": "localhost",
  "port": 3306,
  "timeoutSeconds": 5,
  "warningThresholdMs": 500,
  "intervalSeconds": 60,
  "enabled": true
}
```

#### Process Checks
```json
{
  "id": "nginx-process",
  "name": "Nginx Process Check",
  "type": "process",
  "server": "prod-1",
  "application": "webserver",
  "target": "nginx",
  "timeoutSeconds": 5,
  "intervalSeconds": 60,
  "enabled": true
}
```

#### Command Checks
```json
{
  "id": "disk-space",
  "name": "Disk Space Check",
  "type": "command",
  "server": "prod-1",
  "application": "system",
  "command": "df -h / | awk 'NR==2 {print $5}' | sed 's/%//'",
  "expectedStatus": 0,
  "expectedContains": "Use this",
  "timeoutSeconds": 10,
  "intervalSeconds": 300,
  "enabled": true
}
```

**NOTE:** Command checks require `allowCommandChecks: true` in config.

#### Log Checks
```json
{
  "id": "log-rotation",
  "name": "Log Rotation Check",
  "type": "log",
  "server": "prod-1",
  "application": "medics",
  "path": "/var/log/medics/access.log",
  "freshnessSeconds": 3600,
  "timeoutSeconds": 5,
  "intervalSeconds": 300,
  "enabled": true
}
```

### Per-Check Scheduling

Each check can have individual scheduling parameters:

- **`intervalSeconds`** - How often to run the check (default: 60)
- **`retryCount`** - Number of retry attempts on failure (default: 3)
- **`retryDelaySeconds`** - Delay between retries (default: 5)
- **`cooldownSeconds`** - Minimum time between consecutive failures (default: 30)

## 3. Troubleshooting - Failed Checks

### Check Logs for Error Messages

```bash
# Check service logs
docker compose logs -f healthops

# Docker deployment logs
docker compose logs -f healthops

# Check audit log in MongoDB
mongosh "$MONGODB_URI/healthops" \
  --eval 'db.healthops_audit.find().sort({timestamp:-1}).limit(20).pretty()'

# Run verbose mode (if available)
go run ./cmd/healthops -v
```

### Common Issues and Solutions

#### **Connection Refused**
```bash
# Test connectivity manually
curl -v https://example.com/health

# Check if target is reachable
ping example.com

# Check firewall rules
sudo iptables -L
```

#### **Timeout Issues**
```bash
# Increase timeout in config
{
  "timeoutSeconds": 30,
  "warningThresholdMs": 5000
}
```

#### **Wrong Status Code**
```bash
# Check actual response
curl -I https://example.com/health

# Adjust expected status in config
{
  "expectedStatus": 200
}
```

#### **Wrong Response Body**
```bash
# Check response content
curl -s https://example.com/health

# Update expectedContains
{
  "expectedContains": "healthy"
}
```

### Test Individual Check Manually

```bash
# Trigger a specific check
curl -X POST http://localhost:8080/api/v1/runs \
  -H "Content-Type: application/json" \
  -d '{"checkId": "prod-api"}'

# Check results
curl http://localhost:8080/api/v1/results?checkId=prod-api&days=1
```

### Debug Single Check

```bash
# Create a simple test check
curl -X POST http://localhost:8080/api/v1/checks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "debug-check",
    "name": "Debug Check",
    "type": "api",
    "target": "https://httpbin.org/status/200",
    "expectedStatus": 200,
    "intervalSeconds": 10,
    "enabled": true
  }'

# Watch results in real-time
watch -n 5 curl http://localhost:8080/api/v1/results?checkId=debug-check
```

## 4. Troubleshooting - Failed Alerts

### Check Alert Rules Configuration

```bash
# Get current alert rules (if configured via API)
curl http://localhost:8080/api/v1/alert-rules

# Check if alerts are enabled in config
grep -A 10 "alertRules" config/default.json
```

### Check Cooldown Period

```bash
# Check if cooldown is blocking alerts
{
  "cooldownMinutes": 15,
  "severity": "critical"
}
```

### Test Alert Delivery

```bash
# Check audit log for alert attempts
mongosh "$MONGODB_URI/healthops" \
  --eval 'db.healthops_audit.find({action:/alert|notification/}).sort({timestamp:-1}).limit(20).pretty()'

# Manual alert test
curl -X POST http://localhost:8080/api/v1/runs \
  -H "Content-Type: application/json" \
  -d '{"checkId": "prod-api"}'
```

### Verify Channel Configuration

#### Email Alerts
```json
{
  "type": "email",
  "config": {
    "smtp": {
      "host": "smtp.gmail.com",
      "port": 587,
      "username": "alerts@company.com",
      "password": "app-password"
    },
    "from": "health-monitor@company.com",
    "to": ["admin@company.com", "ops@company.com"]
  }
}
```

#### Webhook Alerts
```json
{
  "type": "webhook",
  "config": {
    "url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json",
      "Authorization": "Bearer token"
    }
  }
}
```

## 5. Troubleshooting - Incidents

### Check Incident Status

```bash
# List all incidents
curl http://localhost:8080/api/v1/incidents

# Get specific incident
curl http://localhost:8080/api/v1/incidents/<incident-id>
```

### Manual Incident Management

**Acknowledge an incident:**
```bash
curl -X PATCH http://localhost:8080/api/v1/incidents/<incident-id> \
  -H "Content-Type: application/json" \
  -d '{
    "status": "acknowledged",
    "acknowledgedBy": "admin@company.com"
  }'
```

**Resolve an incident:**
```bash
curl -X PATCH http://localhost:8080/api/v1/incidents/<incident-id> \
  -H "Content-Type: application/json" \
  -d '{
    "status": "resolved",
    "resolvedBy": "admin@company.com",
    "message": "Issue resolved"
  }'
```

### Auto-Resolve on Recovery

The service automatically resolves incidents when the underlying check recovers:

```json
{
  "autoResolve": true,
  "resolutionThreshold": 3
}
```

## 6. Backup and Restore

### Persistence Location

- **MongoDB:** required primary persistence for production deployments
- **Audit log:** MongoDB collection named `<prefix>_audit`
- **AI provider config:** MongoDB collection named `<prefix>_ai_config`; provider keys are encrypted
- **Data directory:** contains only local secrets such as `.jwt_secret` and `.ai_enc_key`
- **Config file:** `backend/config/default.json` is a first-run seed; after seeding, use the API/UI

### Backup Procedure

```bash
# Create backup directory
mkdir -p /backup/health-monitor/$(date +%Y%m%d)

# Copy configuration files
cp backend/config/default.json /backup/health-monitor/$(date +%Y%m%d)/config.json

# Copy local cryptographic keys
cp backend/data/.jwt_secret /backup/health-monitor/$(date +%Y%m%d)/ 2>/dev/null || true
cp backend/data/.ai_enc_key /backup/health-monitor/$(date +%Y%m%d)/ 2>/dev/null || true

# Backup MongoDB primary persistence
mongodump --uri="$MONGODB_URI" --db=healthops --archive=/backup/health-monitor/$(date +%Y%m%d)/mongodb.archive

# Verify backup
ls -la /backup/health-monitor/$(date +%Y%m%d)/
```

### Restore Procedure

```bash
# Stop the service
pkill healthops

# Restore config
cp /backup/health-monitor/YYYYMMDD/config.json backend/config/default.json

# Restore local cryptographic keys
cp /backup/health-monitor/YYYYMMDD/.jwt_secret backend/data/ 2>/dev/null || true
cp /backup/health-monitor/YYYYMMDD/.ai_enc_key backend/data/ 2>/dev/null || true

# Restore MongoDB primary persistence
mongorestore --uri="$MONGODB_URI" --archive=/backup/health-monitor/YYYYMMDD/mongodb.archive --drop

# Start the service
cd backend && MONGODB_URI="$MONGODB_URI" go run ./cmd/healthops
```

### Automated Backup Script

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/health-monitor"
DATE=$(date +%Y%m%d)

mkdir -p "$BACKUP_DIR/$DATE"

# Backup files
cp -r config/ "$BACKUP_DIR/$DATE/"
cp data/.jwt_secret "$BACKUP_DIR/$DATE/" 2>/dev/null || true
cp data/.ai_enc_key "$BACKUP_DIR/$DATE/" 2>/dev/null || true
mongodump --uri="$MONGODB_URI" --db=healthops --archive="$BACKUP_DIR/$DATE/mongodb.archive"

# Keep only last 7 days
cd "$BACKUP_DIR"
ls -t | tail -n +8 | xargs rm -rf

echo "Backup completed: $BACKUP_DIR/$DATE"
```

## 7. Performance Tuning

### Worker Count Tuning

```json
{
  "workers": 16,
  "checkIntervalSeconds": 60
}
```

**Recommendations:**
- **CPU cores:** Set workers to 2x CPU cores
- **Network checks:** More workers for frequent checks
- **Disk I/O:** Fewer workers for heavy disk checks

### Interval Tuning

```json
{
  "checkIntervalSeconds": 30,
  "retentionDays": 14
}
```

**Best practices:**
- **Critical checks:** 30-60 seconds
- **Warning checks:** 300-600 seconds
- **System checks:** 60-120 seconds

### Retention Tuning

```json
{
  "retentionDays": 30,
  "cleanupIntervalHours": 24
}
```

**Storage recommendations:**
- **Development:** 1-3 days
- **Production:** 7-30 days
- **Compliance:** 90+ days

### MongoDB Mirroring Considerations

```json
{
  "MONGODB_URI": "mongodb://localhost:27017/healthops",
  "MONGODB_DATABASE": "healthops",
  "MONGODB_COLLECTION_PREFIX": "healthops"
}
```

**Performance tips:**
- Use connection pooling
- Index CheckID and timestamps
- Consider sharding for large deployments
- Set appropriate read preferences

## 8. Security

### Authentication Setup

HealthOps uses JWT bearer authentication. On the first Mongo-backed start, set `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` and log in as the fixed `admin` user. Keep `HEALTHOPS_BOOTSTRAP_ADMIN_RESET=false` unless you are intentionally resetting the admin password.

### Command Checks Security

```json
{
  "allowCommandChecks": false
}
```

**Security warnings:**
- Command checks execute arbitrary shell commands
- Only enable for trusted configurations
- Review all command checks regularly
- Use sudo restrictions if possible

### TLS Configuration

```json
{
  "server": {
    "addr": ":8443",
    "tls": {
      "certFile": "cert/server.crt",
      "keyFile": "cert/server.key"
    }
  }
}
```

### Firewall Recommendations

```bash
# Allow HTTP access
sudo ufw allow 8080/tcp

# Allow HTTPS access
sudo ufw allow 8443/tcp

# Restrict access to specific IPs (optional)
sudo ufw allow from 192.168.1.0/24 to any port 8080
```

## 9. Monitoring

### Metrics Endpoint

```bash
# Access Prometheus metrics
curl http://localhost:8080/debug/vars

# Access service metrics (if configured)
curl http://localhost:8080/api/v1/metrics
```

### Key Metrics to Watch

- **`healthops_checks_total`** - Total checks executed
- **`healthops_checks_failed_total`** - Failed checks
- **`healthops_incidents_total`** - Total incidents
- **`healthops_alerts_triggered_total`** - Alerts triggered
- **`healthops_last_run_timestamp_seconds`** - Last run timestamp

### Alerting Recommendations

```json
{
  "alerting": {
    "cpu_usage_percent": {
      "threshold": 80,
      "duration": "5m"
    },
    "memory_usage_percent": {
      "threshold": 85,
      "duration": "10m"
    },
    "check_failure_rate": {
      "threshold": 10,
      "duration": "5m"
    }
  }
}
```

### Log Monitoring

```bash
# Monitor service logs
tail -f /var/log/health-monitor/service.log

# Monitor access logs
tail -f /var/log/health-monitor/access.log

# Monitor error logs
grep ERROR /var/log/health-monitor/service.log
```

## 10. Common Errors

### "Check Not Found"

```bash
# Error: Check ID not found
# Solution: Verify correct check ID
curl http://localhost:8080/api/v1/checks
```

**Fix:**
```bash
# List available checks
curl http://localhost:8080/api/v1/checks

# Use correct check ID from the list
curl http://localhost:8080/api/v1/runs \
  -d '{"checkId": "correct-check-id"}'
```

### "Unauthorized"

```bash
# Error: 401 Unauthorized
# Solution: Check authentication
TOKEN="$(curl -fsS -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<admin-password>"}' | jq -r '.data.token')"

curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/checks
```

**Fix:**
```bash
# If the admin password was lost, reset it once through bootstrap envs.
export HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD='<new-strong-password>'
export HEALTHOPS_BOOTSTRAP_ADMIN_RESET=true
systemctl restart healthops

# After login works, disable reset and restart again.
export HEALTHOPS_BOOTSTRAP_ADMIN_RESET=false
systemctl restart healthops
```

### "Command Checks Disabled"

```bash
# Error: Command checks disabled for security
# Solution: Enable command checks in config
```

**Fix:**
```json
{
  "allowCommandChecks": true,
  "commandChecks": [
    {
      "id": "safe-command",
      "name": "Safe Command",
      "type": "command",
      "command": "df -h",
      "expectedStatus": 0,
      "enabled": true
    }
  ]
}
```

### "Timeout"

```bash
# Error: Check timeout after X seconds
# Solution: Increase timeout or fix connectivity
```

**Fix:**
```json
{
  "timeoutSeconds": 30,
  "warningThresholdMs": 5000
}
```

### "Connection Refused"

```bash
# Error: connection refused
# Solution: Check target service and network
```

**Fix:**
```bash
# Test connectivity
ping target-host
nc -zv target-host port

# Check service status
systemctl status target-service
```

### "Database Connection Failed"

```bash
# Error: MongoDB connection failed
# Solution: Check MongoDB configuration
```

**Fix:**
```bash
# Test MongoDB connection
mongosh --uri="$MONGODB_URI"

# Check MongoDB status
sudo systemctl status mongod
```

### "File Not Found"

```bash
# Error: Log file not found
# Solution: Verify file path and permissions
```

**Fix:**
```bash
# Check file existence
ls -la /path/to/log/file.log

# Check permissions
ls -ld /path/to/log/

# Fix permissions
sudo chmod 644 /path/to/log/file.log
```

### "Invalid Configuration"

```bash
# Error: Invalid configuration file
# Solution: Validate JSON syntax
```

**Fix:**
```bash
# Validate JSON syntax
jq . config/default.json

# Check configuration
go run ./cmd/healthops --validate-config
```

## Emergency Procedures

### Service Won't Start

```bash
# Check for existing process
pkill healthops

# Check port availability
lsof -i :8080

# Check file permissions
ls -la backend/data/
```

### Data Corruption

```bash
# Restore MongoDB from last good backup
mongorestore --uri="$MONGODB_URI" --archive=backup/last-good/mongodb.archive --drop

# Preserve or restore backend/data/.jwt_secret and backend/data/.ai_enc_key
# Losing .ai_enc_key prevents existing encrypted AI provider keys from decrypting.
```

### High CPU Usage

```bash
# Reduce worker count
# Lower check intervals
# Enable rate limiting
```

### Memory Issues

```bash
# Reduce retention period
# Implement result pruning
# Enable MongoDB mirroring to reduce memory usage
```

## Support

For emergency issues, contact the operations team:
- **Email:** ops@company.com
- **Slack:** #health-monitor-alerts
- **PagerDuty:** Health Monitoring Service

## Related Documentation

- [Architecture Overview](../docs/ARCHITECTURE.md)
- [API Reference](../docs/API.md)
- [Configuration Guide](../docs/CONFIGURATION.md)
- [Troubleshooting Guide](../docs/TROUBLESHOOTING.md)
