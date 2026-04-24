#!/usr/bin/env bash
#
# run-chaos.sh — REAL process-kill durability evidence for HealthOps.
#
# Builds the healthops binary, then in a loop:
#   1. spawns it pointed at an isolated mktemp data dir,
#   2. waits a random 0.5–2 s for the scheduler to perform writes,
#   3. kill -9's it,
#   4. asserts the resulting state.json is parseable JSON,
#   5. relaunches briefly to confirm the binary cold-starts from the
#      crashed-on disk state without panicking.
#
# Exit code: 0 if every iteration passes, 1 on first failure.
#
# Flags:
#   --quick     5 iterations (CI-friendly, default for `go test`-adjacent runs)
#   (default)   50 iterations
#
# IMPORTANT: this script never touches the developer's real backend/data/.
# Each iteration uses its own mktemp dir, removed on success.

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

ITERATIONS=50
if [[ "${1:-}" == "--quick" ]]; then
  ITERATIONS=5
fi

BIN="$(mktemp -t healthops-chaos.XXXXXX)"
trap 'rm -f "${BIN}"' EXIT

echo "==> Building healthops binary"
( cd "${REPO_ROOT}/backend" && go build -o "${BIN}" ./cmd/healthops ) || {
  echo "FAIL: build failed"
  exit 1
}
echo "    binary: ${BIN}"
echo

PASS=0
FAIL=0
START_TS=$(date +%s)
PORT=18080  # avoid colliding with anything on :8080

for ((i=1; i<=ITERATIONS; i++)); do
  DATA_DIR="$(mktemp -d -t healthops-chaos-data.XXXXXX)"
  CONFIG_PATH="${DATA_DIR}/config.json"
  STATE_PATH="${DATA_DIR}/state.json"
  PORT=$((PORT + 1))

  # Minimal config: bind a unique port, no checks (faster startup, fewer side
  # effects), retentionDays>0 so the scheduler still flushes UpdatedAt writes.
  # Config validation requires >=1 check. We use a disabled tcp check to a
  # closed loopback port — the scheduler skips it, so no real I/O happens, but
  # state validation passes and the runtime FileStore writes proceed.
  cat > "${CONFIG_PATH}" <<EOF
{
  "server": {
    "addr": "127.0.0.1:${PORT}",
    "readTimeoutSeconds": 5,
    "writeTimeoutSeconds": 5,
    "idleTimeoutSeconds": 30
  },
  "auth": { "enabled": false },
  "retentionDays": 1,
  "checkIntervalSeconds": 1,
  "workers": 2,
  "allowCommandChecks": false,
  "checks": [
    {
      "id": "chaos-noop",
      "name": "Chaos NoOp",
      "type": "tcp",
      "host": "127.0.0.1",
      "port": 1,
      "timeoutSeconds": 1,
      "intervalSeconds": 60,
      "enabled": false
    }
  ]
}
EOF

  # Launch in background with isolated paths.
  CONFIG_PATH="${CONFIG_PATH}" \
  STATE_PATH="${STATE_PATH}" \
  DATA_DIR="${DATA_DIR}" \
  "${BIN}" >"${DATA_DIR}/stdout.log" 2>"${DATA_DIR}/stderr.log" &
  PID=$!

  # Random sleep 0.5–2 s (bash + awk for portability across mac/linux).
  SLEEP=$(awk -v s=$RANDOM 'BEGIN { srand(s); printf "%.2f", 0.5 + rand()*1.5 }')
  sleep "${SLEEP}"

  # SIGKILL — simulates power-loss / OOM-kill / kill -9.
  kill -9 "${PID}" 2>/dev/null
  wait "${PID}" 2>/dev/null

  # Assertion 1: state.json exists and is valid JSON.
  if [[ ! -f "${STATE_PATH}" ]]; then
    echo "FAIL[${i}/${ITERATIONS}]: state.json was never written (sleep=${SLEEP}s)"
    echo "--- stderr ---"
    cat "${DATA_DIR}/stderr.log" | head -40
    FAIL=$((FAIL + 1))
    rm -rf "${DATA_DIR}"
    continue
  fi
  if ! python3 -m json.tool < "${STATE_PATH}" > /dev/null 2>&1; then
    echo "FAIL[${i}/${ITERATIONS}]: state.json is not valid JSON after kill -9 (sleep=${SLEEP}s)"
    echo "--- state.json (first 400 bytes) ---"
    head -c 400 "${STATE_PATH}"
    echo
    echo "--- stderr ---"
    cat "${DATA_DIR}/stderr.log" | tail -20
    FAIL=$((FAIL + 1))
    rm -rf "${DATA_DIR}"
    continue
  fi

  # Assertion 2: no leftover .tmp scratch files anywhere in the data dir.
  if find "${DATA_DIR}" -name '*.tmp' -print -quit | grep -q .; then
    echo "FAIL[${i}/${ITERATIONS}]: .tmp scratch file left behind by aborted writeLocked"
    find "${DATA_DIR}" -name '*.tmp' -print
    FAIL=$((FAIL + 1))
    rm -rf "${DATA_DIR}"
    continue
  fi

  # Assertion 3: cold-restart the binary against the post-crash state — it
  # must successfully open the file (LoadConfig + NewFileStore both succeed)
  # AND respond on /healthz within 3 s.
  CONFIG_PATH="${CONFIG_PATH}" \
  STATE_PATH="${STATE_PATH}" \
  DATA_DIR="${DATA_DIR}" \
  "${BIN}" >"${DATA_DIR}/stdout2.log" 2>"${DATA_DIR}/stderr2.log" &
  PID2=$!

  # Poll /healthz up to 3 seconds.
  HEALTH_OK=0
  for _ in 1 2 3 4 5 6; do
    if curl -fsS --max-time 0.5 "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
      HEALTH_OK=1
      break
    fi
    sleep 0.5
  done
  # SIGKILL the cold-restart instance too — graceful shutdown isn't under test
  # here, and SIGTERM has been observed to block on slow background workers.
  kill -9 "${PID2}" 2>/dev/null
  wait "${PID2}" 2>/dev/null

  if [[ "${HEALTH_OK}" -ne 1 ]]; then
    echo "FAIL[${i}/${ITERATIONS}]: cold restart could not serve /healthz (sleep=${SLEEP}s)"
    echo "--- stderr2 ---"
    cat "${DATA_DIR}/stderr2.log" | tail -20
    FAIL=$((FAIL + 1))
    rm -rf "${DATA_DIR}"
    continue
  fi

  PASS=$((PASS + 1))
  printf "PASS[%2d/%d]  sleep=%ss  state_size=%dB\n" \
    "${i}" "${ITERATIONS}" "${SLEEP}" "$(wc -c < "${STATE_PATH}" | tr -d ' ')"
  rm -rf "${DATA_DIR}"
done

END_TS=$(date +%s)
ELAPSED=$((END_TS - START_TS))

echo
echo "================================================================"
echo "CHAOS SUMMARY"
echo "  iterations : ${ITERATIONS}"
echo "  pass       : ${PASS}"
echo "  fail       : ${FAIL}"
echo "  wall time  : ${ELAPSED}s"
echo "================================================================"

if [[ "${FAIL}" -gt 0 ]]; then
  exit 1
fi
exit 0
