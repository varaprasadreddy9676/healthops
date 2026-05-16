#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/demo-common.sh"

wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps"
wait_for_http "$DEMO_API_URL/health" "demo API" 30

token="$(login_token)"
if [[ -z "$token" ]]; then
  echo "Could not log in to HealthOps demo as ${DEMO_USERNAME}" >&2
  exit 1
fi

curl -fsS -H "Authorization: Bearer $token" "$HEALTHOPS_URL/api/v1/summary" >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$HEALTHOPS_URL/api/v1/checks" >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$HEALTHOPS_URL/api/v1/logs/stats" >/dev/null
curl -fsS -H "Authorization: Bearer $token" "$HEALTHOPS_URL/api/v1/mysql/health" >/dev/null

echo "Demo smoke checks passed."
