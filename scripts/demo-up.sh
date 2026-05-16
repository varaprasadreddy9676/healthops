#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HEALTHOPS_DEMO_PICK_PORTS=true
# shellcheck disable=SC1091
source "$SCRIPT_DIR/demo-common.sh"

cat >"$RUNTIME_ENV_FILE" <<EOF
HEALTHOPS_PORT=$HEALTHOPS_PORT
MYSQL_PORT=$MYSQL_PORT
DEMO_API_PORT=$DEMO_API_PORT
DEMO_LINUX_SSH_PORT_1=$DEMO_LINUX_SSH_PORT_1
DEMO_LINUX_SSH_PORT_2=$DEMO_LINUX_SSH_PORT_2
EOF

compose up -d --build
unset HEALTHOPS_DEMO_PICK_PORTS
"$SCRIPT_DIR/demo-smoke.sh"

"$SCRIPT_DIR/demo-configure-ai.sh"

cat <<EOF

HealthOps demo is ready.
URL:      ${HEALTHOPS_URL}
Login:    ${DEMO_USERNAME}
Password: ${DEMO_PASSWORD}

Try scenarios:
  scripts/demo-scenario.sh api-slow
  scripts/demo-scenario.sh api-down
  scripts/demo-scenario.sh api-flaky
  scripts/demo-scenario.sh log-spike
  scripts/demo-scenario.sh mysql-load
  scripts/demo-scenario.sh rca
  scripts/demo-scenario.sh recover
EOF
