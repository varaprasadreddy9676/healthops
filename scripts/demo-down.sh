#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/demo-common.sh"

if [[ "${1:-}" == "--volumes" || "${1:-}" == "-v" ]]; then
  compose down -v --remove-orphans
else
  compose down --remove-orphans
fi
