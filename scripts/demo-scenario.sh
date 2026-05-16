#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/demo-common.sh"

usage() {
  cat <<EOF
Usage: scripts/demo-scenario.sh <scenario>

Scenarios:
  api-slow     Make the checkout API respond slowly so latency checks warn.
  api-down     Make the checkout API return 503 so incidents open.
  api-flaky    Make every few checkout requests return 500.
  log-spike    Push a burst of realistic application/database errors.
  mysql-load   Run a short MySQL workload spike.
  rca          Create a checkout incident, generate AI brief, and run RCA.
  recover      Return the demo API to healthy baseline.
  status       Show current demo API scenario state.
EOF
}

post_demo_api() {
  local path="$1"
  curl -fsS -X POST "$DEMO_API_URL$path"
  echo
}

require_token() {
  wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps"
  token="$(login_token)"
  if [[ -z "$token" ]]; then
    echo "Could not log in to HealthOps demo as ${DEMO_USERNAME}" >&2
    exit 1
  fi
}

log_spike() {
  require_token
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
  compose exec -T -e MYSQL_PWD="${MYSQL_PASSWORD:-healthops123}" mysql mysql \
    -u"${MYSQL_USER:-healthops}" \
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

  report_id="$(extract_first_id </tmp/healthops-demo-rca.json)"
  if [[ -n "$report_id" ]]; then
    echo "RCA report generated: ${report_id}"
  else
    echo "RCA request completed. Inspect /tmp/healthops-demo-rca.json for details."
  fi
}

scenario="${1:-}"
case "$scenario" in
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
