# Production Release Checklist

Use this checklist before tagging or announcing a public production release.

## 1. Required Automated Verification

- [ ] Backend tests pass:
  ```bash
  cd backend && go test ./...
  ```

- [ ] Frontend typecheck passes:
  ```bash
  cd frontend && npm run typecheck
  ```

- [ ] Frontend production build passes:
  ```bash
  cd frontend && npm run build
  ```

- [ ] OpenAPI YAML parses:
  ```bash
  ruby -e 'require "yaml"; YAML.load_file("docs/openapi.yaml"); puts "openapi yaml ok"'
  ```

- [ ] Public docs do not contain stale persistence or auth guidance:
  ```bash
  rg -n "Authorization: Basic|state\\.json|data/ai_config\\.json|users\\.json|STATE_PATH|file store is the source" README.md docs backend/docs backend/internal/monitoring/helpcontent --glob '!backend/docs/release-checklist.md'
  ```

- [ ] No obvious committed API keys:
  ```bash
  rg -n "sk-or-|sk-[A-Za-z0-9_-]{20,}|xox[baprs]-|ghp_[A-Za-z0-9_]{20,}" .
  ```

## 2. Docker Verification

- [ ] Production compose boots:
  ```bash
  HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD='change-this-strong-password' docker compose up -d --build
  curl http://localhost:8080/healthz
  docker compose down
  ```

- [ ] Demo compose boots:
  ```bash
  docker compose -f compose.demo.yaml up -d --build
  scripts/demo-scenario.sh smoke
  scripts/demo-scenario.sh rca
  docker compose -f compose.demo.yaml down -v
  ```

## 3. Security Verification

- [ ] `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` is set only through deployment secrets or environment.
- [ ] `HEALTHOPS_BOOTSTRAP_ADMIN_RESET=false` for normal production operation.
- [ ] MongoDB is private to the host/network and backed up.
- [ ] `data/.jwt_secret` and `data/.ai_enc_key` are backed up securely.
- [ ] TLS is terminated by a reverse proxy or load balancer.
- [ ] At least one notification channel is configured and tested.
- [ ] AI provider keys are configured through Settings/API and never committed.
- [ ] Any key pasted into chat, issues, docs, screenshots, or demos is rotated before release.

## 4. Release Sign-Off

- [ ] Engineering sign-off:
- [ ] QA sign-off:
- [ ] Security sign-off:
- [ ] Version tag created:
- [ ] Changelog/release notes published:

## Release Gate

All items in sections 1, 2, and 3 must pass for a production release.
