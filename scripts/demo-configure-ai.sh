#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/demo-common.sh"

wait_for_http "$HEALTHOPS_URL/healthz" "HealthOps"
token="$(login_token)"
if [[ -z "$token" ]]; then
  echo "Could not log in to HealthOps demo as ${DEMO_USERNAME}" >&2
  exit 1
fi

provider_id="local-demo-ai"
provider_name="Local Demo AI"
provider_key="demo-local-key"
provider_base_url="http://demo-ai-provider:9200/v1"
model="healthops-demo-ops-model"

if [[ -n "${OPENROUTER_API_KEY:-}" ]]; then
  provider_id="openrouter-demo"
  provider_name="OpenRouter Demo"
  provider_key="${OPENROUTER_API_KEY}"
  provider_base_url="https://openrouter.ai/api/v1"
  model="${OPENROUTER_MODEL:-openai/gpt-4o-mini}"
fi

provider_payload="$(cat <<JSON
{
  "id": "${provider_id}",
  "provider": "custom",
  "name": "${provider_name}",
  "apiKey": "${provider_key}",
  "baseURL": "${provider_base_url}",
  "model": "${model}",
  "maxTokens": 1200,
  "temperature": 0.2,
  "enabled": true,
  "isDefault": true
}
JSON
)"

status="$(curl -sS -o /tmp/healthops-ai-provider.json -w '%{http_code}' \
  -X POST \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d "$provider_payload" \
  "$HEALTHOPS_URL/api/v1/ai/providers")"

if [[ "$status" != "201" && "$status" != "200" ]]; then
  curl -fsS \
    -X PUT \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$provider_payload" \
    "$HEALTHOPS_URL/api/v1/ai/providers/${provider_id}" >/dev/null
fi

curl -fsS \
  -X PUT \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true,"autoAnalyze":true,"maxConcurrent":2,"timeoutSeconds":30,"retryCount":1,"retryDelayMs":1000}' \
  "$HEALTHOPS_URL/api/v1/ai/config" >/dev/null

curl -fsS -H "Authorization: Bearer $token" "$HEALTHOPS_URL/api/v1/ai/health" >/dev/null || true

echo "${provider_name} provider configured with model ${model}."
if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  echo "Using the local deterministic demo provider. For a real model, run:"
  echo "  OPENROUTER_API_KEY=sk-or-v1-... scripts/demo-configure-ai.sh"
fi
