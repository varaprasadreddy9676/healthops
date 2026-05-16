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
  compose_demo exec -T mongo mongosh --quiet healthops --eval "$1"
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
	checks="$(mongo_count "healthops_checks")"
	results="$(mongo_count "healthops_results")"
	users="$(mongo_count "healthops_users")"
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
  checks_before="$(mongo_count "healthops_checks")"
  results_before="$(mongo_count "healthops_results")"

  compose_demo restart healthops >/dev/null
  wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps after restart" 60
  require_token

  checks_after="$(mongo_count "healthops_checks")"
  results_after="$(mongo_count "healthops_results")"

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
