# MongoDB Repository Integration

## Status

MongoDB is the required runtime persistence backend for HealthOps.

This document is a concise reference for the current repository wiring. Older integration notes that described fallback storage, best-effort MongoDB mirroring, or state-file migration are obsolete. See `docs/decisions/ADR-005-mongodb-primary-persistence.md` for the final persistence decision.

## Runtime Model

At startup, `backend/cmd/healthops/main.go`:

1. Requires `MONGODB_URI`.
2. Connects to MongoDB and fails fast if the ping fails.
3. Initializes local cryptographic material in `DATA_DIR`.
4. Seeds checks and servers from config only when MongoDB collections are empty.
5. Wires all runtime repositories to MongoDB-backed implementations.
6. Exposes `/healthz` and system status as degraded/unhealthy when MongoDB is unavailable.

There is no production JSON or JSONL persistence path.

## Core Collections

Collection names use the configured `MONGODB_COLLECTION_PREFIX`, defaulting to `healthops`.

| Domain | Collection |
|--------|------------|
| Main state, checks, latest status | `healthops_state` |
| Users | `healthops_users` |
| Servers | `healthops_servers` |
| Incidents | `healthops_incidents` |
| Incident snapshots | `healthops_incident_snapshots` |
| Audit events | `healthops_audit` |
| Notification channels | `healthops_notification_channels` |
| Notification outbox/logs | `healthops_notification_outbox` |
| Alert rules | `healthops_alert_rules` |
| AI provider config | `healthops_ai_config` |
| AI queue | `healthops_ai_queue` |
| AI results | `healthops_ai_results` |
| RCA reports | `healthops_rca_reports` |
| Log entries/families | MongoDB log repositories |
| Recommendations/remediation | MongoDB recommendation/remediation repositories |

## Secrets

MongoDB stores encrypted AI provider records, but it does not store the local encryption key.

Required local files in `DATA_DIR`:

| File | Purpose |
|------|---------|
| `.jwt_secret` | JWT signing secret |
| `.ai_enc_key` | AES-256-GCM key used to encrypt/decrypt AI provider keys |

Back up MongoDB plus these two local files. Losing `.ai_enc_key` prevents existing encrypted AI provider keys from being decrypted.

## Operational Checks

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/system/status
```

MongoDB backup:

```bash
mongodump --uri="$MONGODB_URI" --db="${MONGODB_DATABASE:-healthops}" --archive=healthops-mongodb.archive
```

MongoDB restore:

```bash
mongorestore --uri="$MONGODB_URI" --archive=healthops-mongodb.archive --drop
```
