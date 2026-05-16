#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

: "${HEALTHOPS_SMTP_PASS:?HEALTHOPS_SMTP_PASS must be set before starting HealthOps}"

cd "$BACKEND_DIR"

CONFIG_PATH="$BACKEND_DIR/config/medics.json" \
STATE_PATH="$BACKEND_DIR/data/medics/state.json" \
DATA_DIR="$BACKEND_DIR/data/medics" \
FRONTEND_DIR="$ROOT_DIR/frontend/dist" \
go run ./cmd/healthops
