# Repository Guidelines

## Project Structure & Module Organization
- `backend/` is the new Go service. `backend/cmd/healthmon/main.go` is the entrypoint and `backend/internal/monitoring/` holds config, store, runner, and HTTP handlers.
- `backend/config/default.json` defines the initial monitored checks and runtime defaults.
- `backend/data/` stores the file-backed state used by the API and scheduler.
- `frontend/` is reserved for the monitoring UI and should stay separate from backend code.

## Build, Test, and Development Commands
- `cd backend && go run ./cmd/healthmon` starts the monitoring API and scheduler.
- `cd backend && go test ./...` runs backend tests once they are added.
- `go fmt ./...` should be run from `backend/` before committing Go changes.
- `go test ./...` and `go run ./cmd/healthmon` assume you are inside the backend module directory.

## Coding Style & Naming Conventions
- Use standard Go formatting and keep package names short, lowercase, and domain-oriented.
- Prefer small packages with clear boundaries such as `internal/monitoring`.
- Name JSON fields and config keys in lower camel case to match the API payloads.
- Keep new UI files under `frontend/` and backend code under `backend/` only.

## Testing Guidelines
- Add Go tests next to the code they cover, for example `backend/internal/monitoring/config_test.go`.
- Focus tests on config validation, result retention, check execution, and API handlers.
- For manual verification, hit `/healthz`, `/api/v1/summary`, and `GET /api/v1/checks` after starting the service.

## Commit & Pull Request Guidelines
- Keep commits short, imperative, and scoped to a single backend or frontend change.
- Pull requests should explain the feature, the impact on checks/configuration, and how it was validated.
- Include screenshots for UI work and sample JSON for API changes when useful.

## Security & Configuration Tips
- Do not hardcode credentials, webhook URLs, or passwords in source files.
- Keep secrets in environment variables or deployment config, never in `backend/config/default.json`.
- Review config changes carefully because they directly control alerting and monitoring scope.
