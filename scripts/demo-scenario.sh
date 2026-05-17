#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

HEALTHOPS_URL="${HEALTHOPS_URL:-http://localhost:${HEALTHOPS_PORT:-18080}}"
DEMO_API_URL="${DEMO_API_URL:-http://localhost:${DEMO_API_PORT:-19100}}"
DEMO_USERNAME="${DEMO_USERNAME:-admin}"
DEMO_PASSWORD="${HEALTHOPS_DEMO_ADMIN_PASSWORD:-healthops-demo-admin}"
MYSQL_USER="${MYSQL_USER:-healthops}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-healthops123}"
MONGODB_DATABASE="${MONGODB_DATABASE:-healthops}"
MONGODB_COLLECTION_PREFIX="${MONGODB_COLLECTION_PREFIX:-healthops}"
token=""

usage() {
  cat <<EOF
Usage: scripts/demo-scenario.sh <scenario>

Scenarios:
  smoke        Verify Docker demo health, login, API summary, and Mongo collections.
  persistence  Verify Mongo-backed checks/results survive a HealthOps container restart.
  mongo-outage Stop MongoDB, verify degraded read-only mode, then restore MongoDB.
  configure-real-integrations
               Configure Slack and OpenRouter from SLACK_WEBHOOK_URL / OPENROUTER_API_KEY.
  ai-health    Verify the configured OpenRouter provider is still healthy after restart.
  stale-log    Create an old log file check and verify it opens an incident.
  crash-logs   Ingest crash-like, OOM, panic, and suspicious security logs.
  real-incident
               Trigger a checkout outage and verify incident + Slack notification + AI analysis.
  api-slow     Make the checkout API respond slowly so latency checks warn.
  api-down     Make the checkout API return 503 so incidents open.
  api-flaky    Make every few checkout requests return 500.
  log-spike    Push a burst of realistic application/database errors.
  log-storm    Ingest one of every real-world log family (15+ entries: payments, gc,
               oom-killed, crashloop, cert-expiry, redis, dns, jwt, hikari, etc.).
  sshd-bruteforce
               Simulate an SSH brute-force burst (10 failed-login events from rotating IPs).
  disk-pressure
               Emit a "disk full" cascade (kernel + app write failures + service restarts).
  cert-expiry  Emit TLS certificate expiry warnings at 30d/7d/1d windows.
  mysql-load   Run a short MySQL workload spike.
  rca          Create a checkout incident, generate AI brief, and run RCA.
  recover      Return the demo API to healthy baseline and refresh checks.
  status       Show current demo API scenario state.
EOF
}

compose_demo() {
  docker compose -f "$ROOT_DIR/compose.demo.yaml" "$@"
}

wait_for_http() {
  local url="$1"
  local label="$2"
  local attempts="${3:-45}"

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "Timed out waiting for ${label}: ${url}" >&2
  return 1
}

wait_for_status() {
  local url="$1"
  local want="$2"
  local label="$3"
  local attempts="${4:-45}"
  local code

  for ((i = 1; i <= attempts; i++)); do
    code="$(curl -sS -o /tmp/healthops-demo-status.out -w '%{http_code}' "$url" || true)"
    if [[ "$code" == "$want" ]]; then
      return 0
    fi
    sleep 2
  done

  echo "Timed out waiting for ${label}: expected HTTP ${want}, got ${code}" >&2
  cat /tmp/healthops-demo-status.out >&2 || true
  return 1
}

login_token() {
  local response
  response="$(curl -fsS \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"${DEMO_USERNAME}\",\"password\":\"${DEMO_PASSWORD}\"}" \
    "$HEALTHOPS_URL/api/v1/auth/login")"

  sed -n 's/.*"token":"\([^"]*\)".*/\1/p' <<<"$response"
}

auth_get() {
  local path="$1"
  curl -fsS -H "Authorization: Bearer $token" "$HEALTHOPS_URL$path"
}

auth_post() {
  local path="$1"
  local payload="${2:-{}}"
  curl -fsS -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$HEALTHOPS_URL$path"
}

auth_put_status() {
  local path="$1"
  local payload="${2:-{}}"
  curl -sS -o /tmp/healthops-demo-put.out -w '%{http_code}' -X PUT \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$HEALTHOPS_URL$path"
}

auth_delete_status() {
  local path="$1"
  curl -sS -o /tmp/healthops-demo-delete.out -w '%{http_code}' -X DELETE \
    -H "Authorization: Bearer $token" \
    "$HEALTHOPS_URL$path"
}

auth_post_status() {
  local path="$1"
  local payload="${2:-{}}"
  curl -sS -o /tmp/healthops-demo-write.out -w '%{http_code}' -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$HEALTHOPS_URL$path"
}

require_token() {
  wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps"
  token="$(login_token)"
  if [[ -z "$token" ]]; then
    echo "Could not log in to HealthOps demo as ${DEMO_USERNAME}" >&2
    exit 1
  fi
}

mongo_eval() {
  compose_demo exec -T mongo mongosh --quiet "$MONGODB_DATABASE" --eval "$1"
}

mongo_count() {
  local collection="$1"
  mongo_eval "db.${collection}.countDocuments()"
}

json_escape() {
  python3 -c 'import json,sys; print(json.dumps(sys.stdin.read())[1:-1])'
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "Assertion failed for ${label}: expected to find ${needle}" >&2
    echo "$haystack" >&2
    exit 1
  fi
}

configure_real_integrations() {
  require_token

  local slack_url="${SLACK_WEBHOOK_URL:-}"
  local openrouter_key="${OPENROUTER_API_KEY:-}"
  local openrouter_model="${OPENROUTER_MODEL:-openai/gpt-4o-mini}"

  if [[ -z "$slack_url" ]]; then
    echo "SLACK_WEBHOOK_URL is required for configure-real-integrations" >&2
    exit 1
  fi
  if [[ -z "$openrouter_key" ]]; then
    echo "OPENROUTER_API_KEY is required for configure-real-integrations" >&2
    exit 1
  fi

  local channel_payload
  channel_payload="$(cat <<JSON
{
  "id": "slack-ops-live",
  "name": "Slack Ops Live",
  "type": "slack",
  "enabled": true,
  "webhookUrl": "${slack_url}",
  "severities": ["critical", "warning"],
  "cooldownMinutes": 0,
  "notifyOnResolve": true
}
JSON
)"

  local channel_status
  channel_status="$(auth_put_status "/api/v1/notification-channels/slack-ops-live" "$channel_payload")"
  if [[ "$channel_status" == "404" || "$channel_status" == "400" ]]; then
    curl -fsS -X POST \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json" \
      -d "$channel_payload" \
      "$HEALTHOPS_URL/api/v1/notification-channels" >/dev/null
  elif [[ "$channel_status" != "200" ]]; then
    echo "Failed to upsert Slack channel, HTTP ${channel_status}" >&2
    exit 1
  fi

  auth_post "/api/v1/notification-channels/test" '{"channelId":"slack-ops-live"}' >/dev/null

  local provider_payload
  provider_payload="$(cat <<JSON
{
  "id": "openrouter-live",
  "provider": "custom",
  "name": "OpenRouter Live",
  "apiKey": "${openrouter_key}",
  "baseURL": "https://openrouter.ai/api/v1",
  "model": "${openrouter_model}",
  "maxTokens": 1200,
  "temperature": 0.2,
  "enabled": true,
  "isDefault": true
}
JSON
)"

  local provider_status
  provider_status="$(auth_put_status "/api/v1/ai/providers/openrouter-live" "$provider_payload")"
  if [[ "$provider_status" == "404" || "$provider_status" == "400" ]]; then
    curl -fsS -X POST \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json" \
      -d "$provider_payload" \
      "$HEALTHOPS_URL/api/v1/ai/providers" >/dev/null
  elif [[ "$provider_status" != "200" ]]; then
    echo "Failed to upsert OpenRouter provider, HTTP ${provider_status}" >&2
    exit 1
  fi

  auth_put_status "/api/v1/ai/config" '{"enabled":true,"autoAnalyze":true,"maxConcurrent":2,"timeoutSeconds":60,"retryCount":1,"retryDelayMs":1000}' >/dev/null

  local health
  health="$(auth_get "/api/v1/ai/health")"
  assert_contains "$health" '"id":"openrouter-live"' "OpenRouter provider health"
  assert_contains "$health" '"healthy":true' "OpenRouter provider health"

  echo "Real integrations OK: Slack test notification sent and OpenRouter provider is healthy."
}

smoke() {
	wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps"
	require_token

	local summary status servers checks results users enabled healthy total
	summary="$(auth_get "/api/v1/summary")"
	status="$(auth_get "/api/v1/system/status")"
	servers="$(auth_get "/api/v1/servers")"
	checks="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_checks")"
	results="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_results")"
	users="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_users")"
	enabled="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["enabledChecks"])' <<<"$summary")"
	healthy="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["healthy"])' <<<"$summary")"
	total="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["totalChecks"])' <<<"$summary")"

	assert_contains "$status" '"healthy":true' "system status"
	assert_contains "$servers" '"linux-server-1"' "server repository"

	if (( enabled < 24 )); then
		echo "Expected at least 24 enabled checks, got ${enabled}" >&2
		exit 1
	fi
	if (( healthy < 1 )); then
		echo "Expected at least one healthy check, got ${healthy}" >&2
		exit 1
	fi
	if (( total != checks )); then
		echo "Summary totalChecks (${total}) does not match Mongo checks (${checks})" >&2
		exit 1
	fi
	if (( checks < 24 )); then
		echo "Expected at least 24 Mongo checks, got ${checks}" >&2
		exit 1
	fi
  if (( results < 1 )); then
    echo "Expected Mongo results to be populated, got ${results}" >&2
    exit 1
  fi
  if (( users < 1 )); then
    echo "Expected Mongo users to be populated, got ${users}" >&2
    exit 1
  fi

	printf 'Smoke OK: checks=%s results=%s users=%s\n' "$checks" "$results" "$users"
}

ai_health() {
	require_token

	local health
	health="$(auth_get "/api/v1/ai/health")"
	assert_contains "$health" '"id":"openrouter-live"' "OpenRouter provider health"
	assert_contains "$health" '"healthy":true' "OpenRouter provider health"

	echo "AI health OK: OpenRouter provider is configured and healthy."
}

wait_for_incident() {
  local check_id="$1"
  local attempts="${2:-30}"
  local incident_id=""

  for ((i = 1; i <= attempts; i++)); do
    incident_id="$(latest_incident_for_check "$check_id" || true)"
    if [[ -n "$incident_id" ]]; then
      echo "$incident_id"
      return 0
    fi
    sleep 2
  done

  return 1
}

persistence() {
  require_token
  auth_post "/api/v1/runs" '{"checkId":"demo-api-health"}' >/dev/null || true
  sleep 2

  local checks_before results_before checks_after results_after
  checks_before="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_checks")"
  results_before="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_results")"

  compose_demo restart healthops >/dev/null
  wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps after restart" 60
  require_token

  checks_after="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_checks")"
  results_after="$(mongo_count "${MONGODB_COLLECTION_PREFIX}_results")"

  if (( checks_after < checks_before )); then
    echo "Mongo check count regressed across restart: before=${checks_before}, after=${checks_after}" >&2
    exit 1
  fi
  if (( results_after < results_before )); then
    echo "Mongo result count regressed across restart: before=${results_before}, after=${results_after}" >&2
    exit 1
  fi

  printf 'Persistence OK: checks %s -> %s, results %s -> %s\n' "$checks_before" "$checks_after" "$results_before" "$results_after"
}

stale_log() {
  require_token

  local stale_stamp
  stale_stamp="$(python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc) - timedelta(minutes=15)).strftime("%Y%m%d%H%M.%S"))
PY
)"
  compose_demo exec -T healthops sh -lc "printf 'last message before logger wedged\n' > /app/data/stale-demo.log && touch -t '$stale_stamp' /app/data/stale-demo.log"

  local payload
  payload="$(cat <<JSON
{
  "id": "demo-stale-log",
  "name": "Demo Stale Log",
  "type": "log",
  "server": "healthops",
  "application": "healthops",
  "path": "/app/data/stale-demo.log",
  "freshnessSeconds": 30,
  "timeoutSeconds": 5,
  "intervalSeconds": 30,
  "enabled": true,
  "tags": ["logs", "stale", "demo"]
}
JSON
)"
  local check_status
  check_status="$(auth_put_status "/api/v1/checks/demo-stale-log" "$payload")"
  if [[ "$check_status" == "404" || "$check_status" == "400" ]]; then
    curl -fsS -X POST \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "$HEALTHOPS_URL/api/v1/checks" >/dev/null
  elif [[ "$check_status" != "200" ]]; then
    echo "Failed to upsert stale log check, HTTP ${check_status}" >&2
    cat /tmp/healthops-demo-put.out >&2 || true
    exit 1
  fi
  auth_post "/api/v1/runs" '{"checkId":"demo-stale-log"}' >/dev/null || true

  local incident_id
  if ! incident_id="$(wait_for_incident "demo-stale-log" 20)"; then
    echo "Expected stale log incident was not created" >&2
    exit 1
  fi

  echo "Stale log OK: incident=${incident_id}"
}

crash_logs() {
  require_token

  local now payload
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  payload="$(cat <<JSON
{
  "entries": [
    {
      "timestamp": "${now}",
      "level": "fatal",
      "source": "checkout-api",
      "server": "linux-server-1",
      "message": "panic: runtime error: invalid memory address or nil pointer dereference while finalizing checkout",
      "stackTrace": "panic: runtime error: invalid memory address\\n  at checkout.finalizeOrder(/srv/app/checkout.go:221)\\n  at http.handler(/srv/app/server.go:88)",
      "tags": ["demo", "crash", "panic", "checkout"],
      "meta": {"scenario": "crash-logs", "impact": "checkout_api_crash"}
    },
    {
      "timestamp": "${now}",
      "level": "error",
      "source": "kernel",
      "server": "linux-server-2",
      "message": "Out of memory: Killed process 4242 (java) total-vm:4096000kB anon-rss:2048000kB",
      "tags": ["demo", "oom", "kernel"],
      "meta": {"scenario": "crash-logs", "component": "event-consumer"}
    },
    {
      "timestamp": "${now}",
      "level": "error",
      "source": "auth-service",
      "server": "linux-server-1",
      "message": "suspicious login burst: 147 failed admin logins from 203.0.113.55 in 60s",
      "tags": ["demo", "security", "bruteforce"],
      "meta": {"scenario": "crash-logs", "remoteIp": "203.0.113.55"}
    },
    {
      "timestamp": "${now}",
      "level": "error",
      "source": "mysql",
      "server": "mysql",
      "message": "InnoDB: Deadlock found when trying to get lock; transaction rolled back for checkout payment update",
      "tags": ["demo", "mysql", "deadlock"],
      "meta": {"scenario": "crash-logs", "table": "payments"}
    }
  ]
}
JSON
)"

  curl -fsS \
    -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$HEALTHOPS_URL/api/v1/logs/ingest" >/dev/null

  echo "Crash logs OK: ingested panic, OOM kill, suspicious login burst, and MySQL deadlock evidence."
}

mongo_outage() {
  require_token

  restore_mongo() {
    compose_demo start mongo >/dev/null || true
  }
  trap restore_mongo EXIT

  compose_demo stop mongo >/dev/null
  wait_for_status "$HEALTHOPS_URL/healthz" "503" "HealthOps unhealthy when MongoDB is stopped" 45

  local status write_code
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    status="$(auth_get "/api/v1/system/status" || true)"
    if [[ "$status" == *'"healthy":false'* ]]; then
      break
    fi
    sleep 2
  done
  assert_contains "$status" '"healthy":false' "degraded system status"

  write_code="$(auth_post_status "/api/v1/checks" '{"id":"should-not-write","name":"Should Not Write","type":"api","target":"https://example.com"}')"
  if [[ "$write_code" != "503" ]]; then
    echo "Expected protected write to be blocked with 503 during Mongo outage, got HTTP ${write_code}" >&2
    cat /tmp/healthops-demo-write.out >&2 || true
    exit 1
  fi

  compose_demo start mongo >/dev/null
  trap - EXIT
  wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps after MongoDB restore" 60

  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    status="$(auth_get "/api/v1/system/status" || true)"
    if [[ "$status" == *'"healthy":true'* ]]; then
      break
    fi
    sleep 2
  done
  assert_contains "$status" '"healthy":true' "recovered system status"

  echo "Mongo outage OK: healthz failed, system entered degraded mode, writes were blocked, and MongoDB recovery cleared degraded mode."
}

post_demo_api() {
  local path="$1"
  curl -fsS -X POST "$DEMO_API_URL$path"
  echo
}

log_spike() {
  require_token
  local now payload
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  payload="$(cat <<JSON
{
  "entries": [
    {
      "timestamp": "${now}",
      "level": "error",
      "source": "checkout-api",
      "server": "linux-server-1",
      "message": "payment authorization timeout after 2900ms for order 880001",
      "stackTrace": "TimeoutError: payment authorization timeout\\n    at authorize (/srv/app/payments.js:88:13)\\n    at checkout (/srv/app/checkout.js:42:9)",
      "tags": ["demo", "scenario", "payments", "timeout"],
      "meta": {"scenario": "log-spike", "dependency": "payment-gateway"}
    },
    {
      "timestamp": "${now}",
      "level": "error",
      "source": "worker",
      "server": "linux-server-2",
      "message": "job reconciliation failed for tenant demo-alpha: deadlock found when trying to get lock",
      "stackTrace": "OperationalError: deadlock found\\n  File \\"/srv/workers/reconcile.py\\", line 144, in run\\n  File \\"/srv/workers/reconcile.py\\", line 61, in update_batch",
      "tags": ["demo", "scenario", "worker", "deadlock"],
      "meta": {"scenario": "log-spike", "queue": "reconciliation"}
    },
    {
      "timestamp": "${now}",
      "level": "warn",
      "source": "mysql",
      "server": "mysql",
      "message": "slow query detected: SELECT COUNT(*) FROM demo_orders WHERE description LIKE '%card%' took 2440ms",
      "tags": ["demo", "scenario", "mysql", "slow-query"],
      "meta": {"scenario": "log-spike", "table": "demo_orders"}
    }
  ]
}
JSON
)"

  curl -fsS \
    -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$HEALTHOPS_URL/api/v1/logs/ingest"
  echo
}

mysql_load() {
  compose_demo exec -T -e MYSQL_PWD="${MYSQL_PASSWORD}" mysql mysql \
    -u"${MYSQL_USER}" \
    healthops \
    -e "INSERT INTO demo_audit_events (event_type, actor, payload) VALUES ('scenario.mysql_load', 'demo-user', JSON_OBJECT('startedAt', NOW())); SELECT COUNT(*) FROM demo_orders a JOIN demo_orders b ON a.status = b.status WHERE a.description LIKE '%card checkout authorization%'; SELECT BENCHMARK(1500000, SHA2(UUID(), 256));"
}

extract_first_id() {
  sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -n 1
}

run_check() {
  local check_id="$1"
  curl -fsS \
    -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"checkId\":\"${check_id}\"}" \
    "$HEALTHOPS_URL/api/v1/runs" >/dev/null
}

latest_incident_for_check() {
  local check_id="$1"
  curl -fsS \
    -H "Authorization: Bearer $token" \
    "$HEALTHOPS_URL/api/v1/incidents?checkId=${check_id}&limit=1" | extract_first_id
}

rca_workflow() {
  require_token
  post_demo_api "/toggle/slow?enabled=true" >/dev/null
  log_spike >/dev/null

  local incident_id=""
  for _ in 1 2 3 4 5 6; do
    run_check "demo-api-latency" || true
    incident_id="$(latest_incident_for_check "demo-api-latency" || true)"
    if [[ -n "$incident_id" ]]; then
      break
    fi
    sleep 3
  done

  if [[ -z "$incident_id" ]]; then
    echo "No demo-api-latency incident found yet. Wait 20 seconds and rerun: scripts/demo-scenario.sh rca" >&2
    exit 1
  fi

  echo "Incident selected for RCA: ${incident_id}"

  curl -fsS \
    -X POST \
    -H "Authorization: Bearer $token" \
    "$HEALTHOPS_URL/api/v1/evidence/brief/${incident_id}" >/tmp/healthops-demo-brief.json
  echo "AI incident brief generated."

  curl -fsS \
    -X POST \
    -H "Authorization: Bearer $token" \
    "$HEALTHOPS_URL/api/v1/rca/analyze/${incident_id}" >/tmp/healthops-demo-rca.json

  local report_id
  report_id="$(extract_first_id </tmp/healthops-demo-rca.json)"
  if [[ -n "$report_id" ]]; then
    echo "RCA report generated: ${report_id}"
  else
    echo "RCA request completed. Inspect /tmp/healthops-demo-rca.json for details."
  fi
}

real_incident() {
  configure_real_integrations
  post_demo_api "/toggle/fail?enabled=true" >/dev/null

  run_check "demo-api-health" || true
  run_check "demo-api-checkout" || true

  local incident_id
  if ! incident_id="$(wait_for_incident "demo-api-health" 30)"; then
    incident_id="$(wait_for_incident "demo-api-checkout" 10 || true)"
  fi
  if [[ -z "$incident_id" ]]; then
    echo "Expected checkout outage incident was not created" >&2
    exit 1
  fi

  sleep 5
  local notifications
  notifications="$(auth_get "/api/v1/notification-logs?limit=20")"
  assert_contains "$notifications" "Slack Ops Live" "Slack notification log"

  local analysis_status
  analysis_status="$(auth_post_status "/api/v1/ai/analyze/${incident_id}" '{"providerId":"openrouter-live"}')"
  if [[ "$analysis_status" != "200" ]]; then
    echo "AI analysis failed for incident ${incident_id}, HTTP ${analysis_status}" >&2
    cat /tmp/healthops-demo-write.out >&2 || true
    exit 1
  fi

  post_demo_api "/recover" >/dev/null
  run_check "demo-api-health" || true
  run_check "demo-api-checkout" || true

  echo "Real incident OK: incident=${incident_id}, Slack notification recorded, OpenRouter analysis completed, demo API recovered."
}

# Shared helper: POST a JSON log batch to /api/v1/logs/ingest using the active token.
post_log_batch() {
  local payload="$1"
  curl -fsS \
    -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$HEALTHOPS_URL/api/v1/logs/ingest" >/dev/null
}

# Ingest one entry per real-world log family — payments timeout, GC pause, OOMKilled,
# CrashLoopBackOff, redis/dns failures, JWT signature failure, HikariCP exhaustion,
# slow query, deadlock, rate limit, audit trail, distributed trace, cert expiry,
# disk full, feature-flag timeout, circuit breaker.
log_storm() {
  require_token

  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  local payload
  payload="$(cat <<JSON
{
  "entries": [
    {"timestamp":"${now}","level":"error","source":"checkout-api","server":"linux-server-1",
     "message":"payment authorization timeout after 3100ms for order 880777",
     "stackTrace":"TimeoutError: payment authorization timeout\\n    at authorize (/srv/app/payments.js:88:13)\\n    at checkout (/srv/app/checkout.js:42:9)",
     "tags":["demo","scenario","payments","timeout"],
     "meta":{"scenario":"log-storm","dependency":"payment-gateway"}},

    {"timestamp":"${now}","level":"error","source":"checkout-api","server":"linux-server-1",
     "message":"circuit breaker OPEN for payment-gateway after 5 consecutive failures",
     "tags":["demo","scenario","resilience","circuit-breaker"],
     "meta":{"scenario":"log-storm","breaker":"payment-gateway","state":"open"}},

    {"timestamp":"${now}","level":"warn","source":"jvm","server":"linux-server-2",
     "message":"GC pause exceeded threshold: G1 Young Generation took 1820ms (heap before=3.8G after=1.2G)",
     "tags":["demo","scenario","jvm","gc"],
     "meta":{"scenario":"log-storm","collector":"G1","pauseMs":1820}},

    {"timestamp":"${now}","level":"error","source":"kubelet","server":"linux-server-2",
     "message":"Container event-consumer-7b9f4 in pod events-prod/event-consumer-deploy-xyz was OOMKilled (limit=2Gi rss=2.1Gi)",
     "tags":["demo","scenario","k8s","oom-killed"],
     "meta":{"scenario":"log-storm","pod":"event-consumer-deploy-xyz","namespace":"events-prod"}},

    {"timestamp":"${now}","level":"error","source":"kubelet","server":"linux-server-2",
     "message":"Pod payments-api-5d8c restarted 6 times in 10m (CrashLoopBackOff, last exit code 137)",
     "tags":["demo","scenario","k8s","crashloop"],
     "meta":{"scenario":"log-storm","pod":"payments-api-5d8c","restartCount":6}},

    {"timestamp":"${now}","level":"error","source":"checkout-api","server":"linux-server-1",
     "message":"redis ECONNREFUSED at cache-primary:6379 — falling back to direct DB read",
     "stackTrace":"Error: connect ECONNREFUSED 10.0.5.12:6379\\n    at TCPConnectWrap.afterConnect (node:net:1494:16)",
     "tags":["demo","scenario","cache","redis"],
     "meta":{"scenario":"log-storm","dependency":"redis","endpoint":"cache-primary:6379"}},

    {"timestamp":"${now}","level":"error","source":"worker","server":"linux-server-2",
     "message":"DNS resolution failed for billing.internal.svc.cluster.local after 5 retries (NXDOMAIN)",
     "tags":["demo","scenario","dns","network"],
     "meta":{"scenario":"log-storm","host":"billing.internal.svc.cluster.local"}},

    {"timestamp":"${now}","level":"error","source":"auth-service","server":"linux-server-1",
     "message":"JWT signature verification failed for request_id=req-storm-001 (alg=HS256, kid=rotated-2024-q4)",
     "stackTrace":"JsonWebTokenError: invalid signature\\n    at /srv/app/auth/jwt.js:55:22",
     "tags":["demo","scenario","security","auth","jwt"],
     "meta":{"scenario":"log-storm","reason":"signature_invalid"}},

    {"timestamp":"${now}","level":"error","source":"checkout-api","server":"linux-server-1",
     "message":"HikariCP connection pool exhausted: 30/30 active, 12 pending threads waiting >5000ms (db=checkout_prod)",
     "stackTrace":"SQLTransientConnectionException: HikariPool-1 - Connection is not available, request timed out after 5001ms\\n    at com.zaxxer.hikari.pool.HikariPool.createTimeoutException(HikariPool.java:696)",
     "tags":["demo","scenario","db","pool-exhausted"],
     "meta":{"scenario":"log-storm","pool":"checkout_prod","active":30,"max":30}},

    {"timestamp":"${now}","level":"warn","source":"mysql","server":"mysql",
     "message":"slow query detected: SELECT COUNT(*) FROM demo_orders WHERE description LIKE '%card%' took 2440ms",
     "tags":["demo","scenario","mysql","slow-query"],
     "meta":{"scenario":"log-storm","table":"demo_orders","durationMs":2440}},

    {"timestamp":"${now}","level":"error","source":"mysql","server":"mysql",
     "message":"InnoDB: Deadlock found when trying to get lock; transaction rolled back for checkout payment update",
     "tags":["demo","scenario","mysql","deadlock"],
     "meta":{"scenario":"log-storm","table":"payments"}},

    {"timestamp":"${now}","level":"warn","source":"api-gateway","server":"linux-server-1",
     "message":"rate limit triggered: 429 Too Many Requests from 198.51.100.42 (limit=1000/min)",
     "tags":["demo","scenario","api","rate-limit"],
     "meta":{"scenario":"log-storm","remoteIp":"198.51.100.42"}},

    {"timestamp":"${now}","level":"info","source":"audit","server":"linux-server-1",
     "message":"user.role.changed actor=admin@example.com target=user-delta from=member to=admin",
     "tags":["demo","scenario","audit","compliance"],
     "meta":{"scenario":"log-storm","event":"user.role.changed","actor":"admin@example.com"}},

    {"timestamp":"${now}","level":"info","source":"api-gateway","server":"linux-server-1",
     "message":"GET /api/v1/orders 200 142ms request_id=req-storm-007 trace_id=4bf92f3577b34da6a3ce929d0e0e4736 span_id=00f067aa0ba902b7",
     "tags":["demo","scenario","access-log","trace"],
     "meta":{"scenario":"log-storm","method":"GET","path":"/api/v1/orders","status":200}},

    {"timestamp":"${now}","level":"warn","source":"nginx","server":"linux-server-1",
     "message":"SSL certificate for api.demo.example.com will expire in 7 days (issued by Let's Encrypt R3)",
     "tags":["demo","scenario","tls","cert-expiry"],
     "meta":{"scenario":"log-storm","issuer":"Let's Encrypt","daysRemaining":7}},

    {"timestamp":"${now}","level":"error","source":"kernel","server":"linux-server-1",
     "message":"EXT4-fs warning: no space left on device /var/lib/docker (95% used, 250MB free of 50GB)",
     "tags":["demo","scenario","disk","out-of-space"],
     "meta":{"scenario":"log-storm","mount":"/var/lib/docker","usagePercent":95}},

    {"timestamp":"${now}","level":"warn","source":"feature-flags","server":"linux-server-1",
     "message":"feature flag evaluation timeout: defaulting to OFF for flag=checkout_v2_new_pricing (provider=launchdarkly took >250ms)",
     "tags":["demo","scenario","feature-flag","timeout"],
     "meta":{"scenario":"log-storm","flag":"checkout_v2_new_pricing"}}
  ]
}
JSON
)"

  post_log_batch "$payload"
  echo "Log storm OK: ingested 17 entries across application, db, security, jvm, k8s, network, tls, and audit families."
}

# Simulate an SSH brute-force burst: 10 failed-password attempts from rotating IPs
# against multiple usernames within the same minute. Mirrors what /var/log/auth.log
# sees during a real attack.
sshd_bruteforce() {
  require_token

  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

  local entries=""
  local sep=""
  local users=(root admin postgres jenkins deploy ubuntu git mysql backup support)
  for i in 1 2 3 4 5 6 7 8 9 10; do
    local user="${users[$((i - 1))]}"
    local octet=$((100 + RANDOM % 150))
    local port=$((40000 + RANDOM % 20000))
    entries+="${sep}{\"timestamp\":\"${now}\",\"level\":\"warn\",\"source\":\"sshd\",\"server\":\"linux-server-2\",\"message\":\"Failed password for invalid user ${user} from 203.0.113.${octet} port ${port} ssh2\",\"tags\":[\"demo\",\"scenario\",\"security\",\"sshd\",\"auth-failure\"],\"meta\":{\"scenario\":\"sshd-bruteforce\",\"remoteIp\":\"203.0.113.${octet}\",\"user\":\"${user}\"}}"
    sep=","
  done
  entries+=",{\"timestamp\":\"${now}\",\"level\":\"error\",\"source\":\"sshd\",\"server\":\"linux-server-2\",\"message\":\"PAM: Possible break-in attempt — 10 failed root/admin logins in 60s from subnet 203.0.113.0/24\",\"tags\":[\"demo\",\"scenario\",\"security\",\"sshd\",\"bruteforce\"],\"meta\":{\"scenario\":\"sshd-bruteforce\",\"subnet\":\"203.0.113.0/24\",\"failureCount\":10}}"

  post_log_batch "{\"entries\":[${entries}]}"
  echo "SSH brute-force OK: ingested 10 rotating failed-login events plus 1 PAM break-in summary."
}

# Simulate a "disk full" cascade: kernel warning, app write failure, db checkpoint
# failure, container exit, follow-up after cleanup.
disk_pressure() {
  require_token

  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  local payload
  payload="$(cat <<JSON
{
  "entries": [
    {"timestamp":"${now}","level":"warn","source":"kernel","server":"linux-server-1",
     "message":"EXT4-fs warning: /var/lib/docker is at 87% capacity (6.5GB free of 50GB)",
     "tags":["demo","scenario","disk","warning"],
     "meta":{"scenario":"disk-pressure","mount":"/var/lib/docker","usagePercent":87}},

    {"timestamp":"${now}","level":"error","source":"kernel","server":"linux-server-1",
     "message":"EXT4-fs error: no space left on device /var/lib/docker (98% used, 100MB free of 50GB)",
     "tags":["demo","scenario","disk","out-of-space"],
     "meta":{"scenario":"disk-pressure","mount":"/var/lib/docker","usagePercent":98}},

    {"timestamp":"${now}","level":"error","source":"checkout-api","server":"linux-server-1",
     "message":"failed to write checkout receipt to /var/log/app/receipts.log: ENOSPC (No space left on device)",
     "stackTrace":"Error: ENOSPC: no space left on device, write\\n    at Object.writeSync (node:fs:1067:3)\\n    at /srv/app/log.js:22:10",
     "tags":["demo","scenario","disk","app-error"],
     "meta":{"scenario":"disk-pressure","service":"checkout","errno":"ENOSPC"}},

    {"timestamp":"${now}","level":"error","source":"mysql","server":"mysql",
     "message":"InnoDB: Error: Write to file ./ibdata1 failed at offset 4194304: OS errno 28 - No space left on device. Aborting checkpoint.",
     "tags":["demo","scenario","disk","mysql"],
     "meta":{"scenario":"disk-pressure","service":"mysql","errno":28}},

    {"timestamp":"${now}","level":"fatal","source":"docker","server":"linux-server-1",
     "message":"container checkout-api exited with code 1 after disk write failures",
     "tags":["demo","scenario","disk","container-exit"],
     "meta":{"scenario":"disk-pressure","container":"checkout-api","exitCode":1}}
  ]
}
JSON
)"

  post_log_batch "$payload"
  echo "Disk pressure OK: ingested kernel warning -> ENOSPC -> app write failure -> MySQL checkpoint failure -> container exit cascade."
}

# Simulate TLS cert expiry across windows: 30 days, 7 days, 1 day, expired.
cert_expiry() {
  require_token

  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  local payload
  payload="$(cat <<JSON
{
  "entries": [
    {"timestamp":"${now}","level":"info","source":"cert-monitor","server":"linux-server-1",
     "message":"TLS certificate for api.demo.example.com will expire in 30 days (issued by Let's Encrypt R3)",
     "tags":["demo","scenario","tls","cert-expiry"],
     "meta":{"scenario":"cert-expiry","host":"api.demo.example.com","daysRemaining":30}},

    {"timestamp":"${now}","level":"warn","source":"cert-monitor","server":"linux-server-1",
     "message":"TLS certificate for api.demo.example.com will expire in 7 days — renewal recommended",
     "tags":["demo","scenario","tls","cert-expiry"],
     "meta":{"scenario":"cert-expiry","host":"api.demo.example.com","daysRemaining":7}},

    {"timestamp":"${now}","level":"error","source":"cert-monitor","server":"linux-server-1",
     "message":"TLS certificate for api.demo.example.com expires in 1 day — automatic renewal failed (acme challenge timeout)",
     "tags":["demo","scenario","tls","cert-expiry","renewal-failed"],
     "meta":{"scenario":"cert-expiry","host":"api.demo.example.com","daysRemaining":1}},

    {"timestamp":"${now}","level":"fatal","source":"nginx","server":"linux-server-1",
     "message":"TLS handshake failures spiking: certificate for api.demo.example.com has EXPIRED (notAfter=2026-05-16T23:59:59Z)",
     "tags":["demo","scenario","tls","cert-expired"],
     "meta":{"scenario":"cert-expiry","host":"api.demo.example.com","status":"expired"}}
  ]
}
JSON
)"

  post_log_batch "$payload"
  echo "Cert expiry OK: ingested 30d -> 7d -> 1d -> expired progression with renewal failure context."
}

scenario="${1:-}"
case "$scenario" in
  smoke)
    smoke
    ;;
  persistence)
    persistence
    ;;
  mongo-outage)
    mongo_outage
    ;;
  configure-real-integrations)
    configure_real_integrations
    ;;
  ai-health)
    ai_health
    ;;
  stale-log)
    stale_log
    ;;
  crash-logs)
    crash_logs
    ;;
  real-incident)
    real_incident
    ;;
  api-slow)
    post_demo_api "/toggle/slow?enabled=true"
    echo "Scenario enabled: checkout API latency. Watch Dashboard, Checks, Incidents, and RCA."
    ;;
  api-down)
    post_demo_api "/toggle/fail?enabled=true"
    echo "Scenario enabled: checkout API outage. Incidents should open after retry/cooldown timing."
    ;;
  api-flaky)
    post_demo_api "/toggle/flaky?enabled=true"
    echo "Scenario enabled: intermittent checkout failures."
    ;;
  log-spike)
    log_spike
    echo "Scenario injected: log spike. Open Logs to inspect grouped families."
    ;;
  log-storm)
    log_storm
    ;;
  sshd-bruteforce)
    sshd_bruteforce
    ;;
  disk-pressure)
    disk_pressure
    ;;
  cert-expiry)
    cert_expiry
    ;;
  mysql-load)
    mysql_load
    echo "Scenario injected: MySQL workload spike. Open MySQL pages for queries/threads/samples."
    ;;
  rca)
    rca_workflow
    echo "Scenario completed: open Incidents, AI Analysis, and RCA Reports to inspect the generated outputs."
    ;;
  recover)
    post_demo_api "/recover"
    require_token
    run_check "demo-api-health" || true
    run_check "demo-api-checkout" || true
    run_check "demo-api-latency" || true
    echo "Scenario recovered: checkout API back to healthy baseline; checkout checks were refreshed."
    ;;
  status)
    curl -fsS "$DEMO_API_URL/status"
    echo
    ;;
  ""|-h|--help)
    usage
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
