# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Medics Health Check is a monitoring system for app servers, APIs, processes, logs, and short-term health trends. The project is built with a Go backend and a planned frontend UI.

**Current architecture:** Go service (`backend/`) handles health checks, alerting, and JSON APIs. Frontend (`frontend/`) is reserved for the future monitoring UI.

## Common Commands

All Go commands must be run from the `backend/` directory:

```bash
# Run the monitoring service (API + scheduler)
cd backend && go run ./cmd/healthmon

# Run tests
cd backend && go test ./...

# Run specific test
cd backend && go test ./internal/monitoring -run TestDashboard

# Format code before committing
cd backend && go fmt ./...
```

## Architecture

### Backend Structure (`backend/`)

- **`cmd/healthmon/main.go`** - Service entrypoint. Loads config, initializes stores (FileStore + optional MongoDB mirror), and starts the HTTP server and check scheduler.
- **`internal/monitoring/`** - Core monitoring package:
  - `config.go` - Config loading, validation, defaults. Config file defines checks and runtime settings.
  - `types.go` - Core types: `Config`, `CheckConfig`, `CheckResult`, `State`, `Summary`, `Store` interface, `Mirror` interface.
  - `store.go` - `FileStore` implementation (local JSON state file with atomic writes).
  - `hybrid_store.go` - `HybridStore` that wraps `FileStore` with an optional MongoDB mirror. Reads prefer Mongo if fresh and non-stale, writes always go to local file first then best-effort sync to Mongo. Service stays up if Mongo is unreachable.
  - `mongo.go` - `MongoMirror` implementation syncing state to MongoDB collections.
  - `runner.go` - `Runner` executes checks in parallel workers (configurable count). Supports check types: `api`, `tcp`, `process`, `command`, `log`. Returns `RunSummary` with results and aggregated counts.
  - `service.go` - `Service` HTTP handlers and scheduler. Exposes health, readiness, check CRUD, run trigger, summary, results, and dashboard endpoints.

### Storage Architecture

The service uses a hybrid storage approach:
1. **Local file store** (`backend/data/state.json`) - Always works, primary source of truth
2. **MongoDB mirror** - Optional best-effort sync via `MONGODB_URI` env var

Read operations (especially dashboard endpoints) prefer Mongo if the data is fresh and non-stale. Writes always update the local file first, then attempt to sync to Mongo asynchronously with a 5-second timeout. If Mongo is down, the service continues operating normally.

### Check Types

Defined in `backend/config/default.json`:

- **`api`** - HTTP endpoint checks. Validates status code, optional response body substring, latency threshold
- **`tcp`** - Port connectivity checks. Can check if port is open and measure latency
- **`process`** - Process existence checks via `ps` command. Matches by keyword in command line
- **`command`** - Custom shell command execution. Validates exit code and optional output contains
- **`log`** - Log file freshness checks. Validates file modification time is within threshold

Each check can be disabled via `"enabled": false`, grouped by `server`/`application` tags, and has configurable timeout and warning thresholds.

### API Endpoints

**Standard endpoints:**
- `GET /healthz` - Liveness probe
- `GET /readyz` - Readiness probe with check count and last run time
- `GET /api/v1/checks` - List all checks
- `POST /api/v1/checks` - Create new check
- `PUT/PATCH /api/v1/checks/{id}` - Update check
- `DELETE /api/v1/checks/{id}` - Delete check
- `POST /api/v1/runs` - Trigger an immediate check run
- `GET /api/v1/summary` - Get health summary with latest results per check
- `GET /api/v1/results?checkId={id}&days={n}` - Get historical results with filtering

**Dashboard endpoints** (return read-optimized snapshots from Mongo when available):
- `GET /api/v1/dashboard/checks` - Dashboard view of all checks
- `GET /api/v1/dashboard/summary` - Pre-aggregated summary for dashboard
- `GET /api/v1/dashboard/results?checkId={id}&days={n}` - Dashboard historical results

### Configuration

**Config file** (`backend/config/default.json`):
```json
{
  "server": {
    "addr": ":8080",
    "readTimeoutSeconds": 10,
    "writeTimeoutSeconds": 10,
    "idleTimeoutSeconds": 60
  },
  "retentionDays": 7,
  "checkIntervalSeconds": 60,
  "workers": 8,
  "checks": [...]
}
```

**Environment variables:**
- `CONFIG_PATH` - Override config file location
- `STATE_PATH` - Override local state file location
- `MONGODB_URI` - Enable MongoDB mirroring (optional)
- `MONGODB_DATABASE` - Mongo database name (default: `healthmon`)
- `MONGODB_COLLECTION_PREFIX` - Mongo collection prefix (default: `healthmon`)

The `backend/config/default.json` contains the check definitions.

## Development Guidelines

### Code Organization
- Place new backend code in `backend/internal/monitoring/` or new `internal/` packages as appropriate
- Keep frontend code in `frontend/` (reserved for future UI)

### Testing
- Tests are located next to the code they cover (e.g., `service_test.go`)
- The `fakeStore` mock in `service_test.go` shows the pattern for Store interface mocking
- Run `go test ./...` before committing changes
- For API verification, start the service and hit `/healthz`, `/api/v1/summary`, and `GET /api/v1/checks`

### Adding New Check Types
To add a new check type:
1. Add the type string to the validate switch in `config.go`
2. Add a `run{Type}` method in `runner.go` that executes the check and populates `result.Metrics`
3. Add a case for the new type in `executeCheck` in `runner.go`
4. Update `backend/config/default.json` with example checks

### Important Patterns

**Immutability:** The store uses a clone-on-write pattern. `Update()` accepts a mutator function that receives a copy of the state, and the change is applied atomically via a temp file + rename operation.

**State pruning:** Old results are automatically pruned based on `retentionDays` config. The `pruneResults` function in `store.go` handles this.

**Dashboard read model:** Dashboard endpoints return a `DashboardSnapshot` which includes both raw state and a pre-computed summary. This is optimized for UI rendering.
