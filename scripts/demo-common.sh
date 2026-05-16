#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.demo"
RUNTIME_ENV_FILE="$ROOT_DIR/.demo-runtime.env"
COMPOSE_FILES=(-f "$ROOT_DIR/docker-compose.yml" -f "$ROOT_DIR/docker-compose.demo.yml")
RESERVED_PORTS=()

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

if [[ -f "$RUNTIME_ENV_FILE" && "${HEALTHOPS_DEMO_PICK_PORTS:-false}" != "true" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$RUNTIME_ENV_FILE"
  set +a
fi

port_in_use() {
  (echo >"/dev/tcp/127.0.0.1/$1") >/dev/null 2>&1
}

port_reserved() {
  local port="$1"
  local reserved
  for reserved in ${RESERVED_PORTS[@]+"${RESERVED_PORTS[@]}"}; do
    [[ "$reserved" == "$port" ]] && return 0
  done
  return 1
}

pick_port() {
  local candidate="$1"
  while port_in_use "$candidate" || port_reserved "$candidate"; do
    candidate=$((candidate + 1))
  done
  echo "$candidate"
}

ensure_port() {
  local var_name="$1"
  local fallback="$2"
  local requested="${!var_name:-$fallback}"
  local selected
  selected="$(pick_port "$requested")"
  if [[ "$selected" != "$requested" ]]; then
    echo "${var_name}=${requested} is busy; using ${selected} for this demo run." >&2
  fi
  export "$var_name=$selected"
  RESERVED_PORTS+=("$selected")
}

if [[ "${HEALTHOPS_DEMO_PICK_PORTS:-false}" == "true" ]]; then
  ensure_port HEALTHOPS_PORT 18080
else
  export HEALTHOPS_PORT="${HEALTHOPS_PORT:-18080}"
fi
export MONGODB_PORT="${MONGODB_PORT:-27018}"
if [[ "${HEALTHOPS_DEMO_PICK_PORTS:-false}" == "true" ]]; then
  ensure_port MYSQL_PORT 13306
  ensure_port DEMO_API_PORT 19100
  ensure_port DEMO_LINUX_SSH_PORT_1 12222
  ensure_port DEMO_LINUX_SSH_PORT_2 12223
else
  export MYSQL_PORT="${MYSQL_PORT:-13306}"
  export DEMO_API_PORT="${DEMO_API_PORT:-19100}"
  export DEMO_LINUX_SSH_PORT_1="${DEMO_LINUX_SSH_PORT_1:-12222}"
  export DEMO_LINUX_SSH_PORT_2="${DEMO_LINUX_SSH_PORT_2:-12223}"
fi
export HEALTHOPS_DEMO_ADMIN_PASSWORD="${HEALTHOPS_DEMO_ADMIN_PASSWORD:-healthops-demo-admin}"
export HEALTHOPS_BOOTSTRAP_ADMIN_RESET="${HEALTHOPS_BOOTSTRAP_ADMIN_RESET:-true}"
export MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-rootpass123}"
export MYSQL_USER="${MYSQL_USER:-healthops}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-healthops123}"

PROJECT_NAME="${COMPOSE_PROJECT_NAME:-healthops-demo}"
HEALTHOPS_URL="${HEALTHOPS_URL:-http://localhost:${HEALTHOPS_PORT}}"
DEMO_API_URL="${DEMO_API_URL:-http://localhost:${DEMO_API_PORT}}"
DEMO_USERNAME="${DEMO_USERNAME:-admin}"
DEMO_PASSWORD="${HEALTHOPS_DEMO_ADMIN_PASSWORD:-healthops-demo-admin}"

compose() {
  if [[ -f "$ENV_FILE" ]]; then
    docker compose -p "$PROJECT_NAME" --env-file "$ENV_FILE" "${COMPOSE_FILES[@]}" "$@"
  else
    docker compose -p "$PROJECT_NAME" "${COMPOSE_FILES[@]}" "$@"
  fi
}

wait_for_http() {
  local url="$1"
  local label="$2"
  local attempts="${3:-90}"

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "Timed out waiting for ${label}: ${url}" >&2
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
