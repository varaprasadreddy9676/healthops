# HealthOps Backup and Disaster Recovery

**Audience:** the operator on the hook for restoring HealthOps after data
loss.

For volume layout details see [deployment.md](deployment.md). For the
operational playbooks that follow a restore see [runbook.md](runbook.md).

---

## 1. What to back up

Back up the **entire** data directory (`/var/lib/healthops/data/` for the
binary install, `/srv/healthops/data/` for the recommended Compose layout).
The whole directory is self-consistent; do not cherry-pick files.

| File / dir | What it holds | Loss impact |
|---|---|---|
| `state.json` | Checks, results, incidents, retention pointers | Authoritative service state. Loss = full reset. |
| `users.json` | Local user store (file-backed deployments) | Loss = locked out, must re-bootstrap admin. |
| `.ai_enc_key` | AES-256 key encrypting `ai_config.json` | **Catastrophic.** Without it, AI provider creds are unrecoverable. |
| `.jwt_secret` | JWT signing key | Loss invalidates outstanding sessions; users must re-log-in. |
| `ai_config.json` | Encrypted AI provider keys, prompt templates | Loss removes BYOK config; must re-enter API keys. |
| `alert_rules.json` | MySQL and check alert thresholds | Loss reverts to defaults; tuning is lost. |
| `audit.json` | Audit trail | Compliance / forensics value only. |
| `incident_snapshots.jsonl` | Evidence captured at incident open | Loss removes post-mortem context. |
| `mysql_samples.jsonl`, `mysql_deltas.jsonl`, `mysql_rule_states.json` | MySQL telemetry and rule state | Loss = short-term gap in MySQL trends. |
| `notification_outbox.jsonl` | Queued + delivered notifications | Loss may re-fire pending alerts on restart. |
| `ai_queue.jsonl`, `ai_results.jsonl` | AI work queue + history | Loss drops pending analyses and history. |
| `server_metrics/*.jsonl` | Per-server time-series | Loss = short-term gap in trends. |

### `.ai_enc_key` requires its own treatment

`.ai_enc_key` decrypts `ai_config.json`. If the snapshot containing
`ai_config.json` is restored without the matching `.ai_enc_key`, every stored
AI provider credential is lost. Two practical options:

1. **Back up `.ai_enc_key` alongside the rest of `data/`** (simplest), and
   accept that anyone with access to the backup can decrypt the AI config.
   Encrypt the backup at rest with `age` or `gpg` to compensate.
2. **Back up `.ai_enc_key` separately to a secrets store** (Vault, AWS
   Secrets Manager, 1Password, etc.) and back up the rest of `data/` to
   bulk storage. Document the restore order: pull the key first, restore
   `data/` second.

The cron snippet below uses option 1 with `age` encryption.

---

## 2. Backup frequency

| Workload pattern | Frequency | Rationale |
|---|---|---|
| Idle / dev | Daily | RPO 24h is fine. |
| Active monitoring with incidents | Hourly snapshot of `state.json` + JSONL + daily full | Limits incident/audit data loss to ~1h. |
| MySQL monitoring at 1m intervals | Hourly | `mysql_samples.jsonl` and `mysql_deltas.jsonl` grow continuously; hourly keeps the gap small. |

Always keep at least:
- 7 daily snapshots,
- 4 weekly snapshots,
- 12 monthly snapshots,

before pruning. This survives both fast (single-day) corruption and slow
(weeks-old) corruption.

---

## 3. Backup script

Run as the same user that owns `data/` (`healthops` for the binary install,
UID 1000 for the container). Tested on Linux with `tar`, `age`, and either
`rclone` or `aws-cli`.

`/usr/local/bin/healthops-backup.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${DATA_DIR:-/var/lib/healthops/data}"
BACKUP_TMP="$(mktemp -d)"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
HOST="$(hostname -s)"
ARCHIVE="${BACKUP_TMP}/healthops-${HOST}-${TIMESTAMP}.tar.zst"
ENCRYPTED="${ARCHIVE}.age"

# AGE_RECIPIENT is a public key (age1...). Keep the matching identity OFFLINE.
: "${AGE_RECIPIENT:?AGE_RECIPIENT must be set to an age public key}"

# Snapshot the data dir. Use --acls --xattrs to preserve mode 0750 etc.
tar --zstd \
    --acls --xattrs \
    -C "$(dirname "$DATA_DIR")" \
    -cf "$ARCHIVE" \
    "$(basename "$DATA_DIR")"

# Encrypt at rest
age -r "$AGE_RECIPIENT" -o "$ENCRYPTED" "$ARCHIVE"
shred -u "$ARCHIVE"

# Ship to remote storage. Pick one.
# rclone copy "$ENCRYPTED" remote:healthops-backups/
aws s3 cp "$ENCRYPTED" "s3://your-bucket/healthops/${HOST}/" \
    --storage-class STANDARD_IA \
    --only-show-errors

rm -rf "$BACKUP_TMP"
echo "ok ${ENCRYPTED##*/}"
```

`/etc/cron.d/healthops-backup`:

```cron
# m h dom mon dow user command
0  *  *   *   *   healthops AGE_RECIPIENT=age1... /usr/local/bin/healthops-backup.sh >> /var/log/healthops-backup.log 2>&1
```

For the daily-only cadence drop the hourly entry and use `0 2 * * *`.

Lifecycle on the bucket: transition to S3 IA at 7 days, Glacier at 30 days,
expire at 400 days. Adjust to your retention policy.

---

## 4. Restore procedure

Restore is a stop / replace / start. The whole `data/` directory is restored
atomically; partial restores produce inconsistent state.

```bash
# 1. Stop the service
sudo systemctl stop healthops
# or: docker compose -f docker-compose.yml -f docker-compose.prod.yml stop healthops

# 2. Pull the encrypted archive
aws s3 cp s3://your-bucket/healthops/<host>/healthops-<host>-<timestamp>.tar.zst.age /tmp/

# 3. Decrypt with the offline age identity
age -d -i /path/to/age-identity.txt \
    -o /tmp/healthops-restore.tar.zst \
    /tmp/healthops-<host>-<timestamp>.tar.zst.age

# 4. Move the current data dir aside (do NOT delete until restore verified)
DATA_DIR=/var/lib/healthops/data
sudo mv "$DATA_DIR" "${DATA_DIR}.broken-$(date -u +%Y%m%dT%H%M%SZ)"

# 5. Extract the snapshot
sudo tar --zstd --acls --xattrs \
    -C "$(dirname "$DATA_DIR")" \
    -xf /tmp/healthops-restore.tar.zst
sudo chown -R healthops:healthops "$DATA_DIR"
sudo chmod 0750 "$DATA_DIR"

# 6. Start the service
sudo systemctl start healthops
# or: docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d healthops

# 7. Verify
curl -fsS http://127.0.0.1:8080/healthz
TOKEN=$(curl -fsS -X POST http://127.0.0.1:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"<known-good-admin-password>"}' \
  | jq -r '.data.token')
curl -fsS http://127.0.0.1:8080/api/v1/summary -H "Authorization: Bearer $TOKEN" | jq

# 8. Once verified for at least 24h, remove the .broken-* directory
```

If the restored `ai_config.json` cannot be decrypted, the matching
`.ai_enc_key` was not in the snapshot. Restore the key separately from the
secrets store and restart.

---

## 5. MongoDB mirror is not a backup

When `MONGODB_URI` is set HealthOps mirrors writes to MongoDB on a
best-effort basis with a short timeout. Reads may prefer Mongo when the data
is fresh. This is **not** a backup:

- Writes are async. If the process dies between the local file write and the
  Mongo write, the Mongo copy is missing the last record.
- If the local file is corrupted and replicated to Mongo, the corruption is
  in Mongo too.
- Mongo collections are pruned in lockstep with local retention.

Always run a real backup of `data/` even when Mongo is in use. Back Mongo up
separately (mongodump on its own schedule) if you rely on it for analytics.

---

## 6. Targets

For an internal infrastructure tool:

- **RPO (Recovery Point Objective):** 24 hours. Hourly snapshots tighten this
  to 1 hour during active monitoring.
- **RTO (Recovery Time Objective):** 1 hour. Restore is a single archive
  download plus a service restart; this fits comfortably.

Tighten these only if HealthOps itself is reclassified above
"internal-tool" tier — at which point you also need redundancy at the
service layer (see [slo.md](slo.md), section 4).
