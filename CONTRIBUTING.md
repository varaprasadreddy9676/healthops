# Contributing to HealthOps

Thank you for your interest in contributing! This document explains how to get involved — from reporting bugs to submitting code.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

---

## Reporting Bugs

Found something broken? Please [open a GitHub Issue](https://github.com/varaprasadreddy9676/healthops/issues/new) and include:

- **What you expected to happen**
- **What actually happened** (paste error messages or logs)
- **Steps to reproduce** — the more specific, the better
- **Your environment**: OS, Docker version, Docker Compose version
- **HealthOps version** (check the `/healthz` endpoint for the version)

> For security vulnerabilities, please do **not** open a public issue. See [SECURITY.md](SECURITY.md) instead.

---

## Suggesting Features

Have an idea? [Open a GitHub Issue](https://github.com/varaprasadreddy9676/healthops/issues/new) with the label `enhancement`. Describe:

- **The problem you're trying to solve** — what workflow is painful today?
- **Your proposed solution** — how would it work from the user's perspective?
- **Alternatives you considered**

We prioritize features that help ops teams with real-world monitoring challenges.

---

## Development Setup

### Option A: Full stack with Docker (recommended for UI work)

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops

# Start the demo stack — everything pre-configured
docker compose -f compose.demo.yaml up -d --build

# Open http://localhost:18080
```

### Option B: Backend only (fastest iteration)

```bash
git clone https://github.com/varaprasadreddy9676/healthops.git
cd healthops/backend

# Run the backend server (serves on :8080)
go run ./cmd/healthops
```

### Option C: Full local development

**Backend:**
```bash
cd backend
go run ./cmd/healthops
```

**Frontend (in a separate terminal):**
```bash
cd frontend
npm install
npm run dev      # Vite dev server at http://localhost:5173
```

The frontend dev server proxies API calls to the backend at `:8080` automatically.

---

## Running Tests

### Backend

```bash
cd backend

# Run all tests
go test ./...

# Run with race detector (recommended before submitting a PR)
go test ./... -race

# Run a specific test
go test ./internal/monitoring -run TestDashboard

# Format code
go fmt ./...

# Vet code
go vet ./...
```

### Frontend

```bash
cd frontend

npm install

# Type check (must pass before submitting a PR)
npm run typecheck

# Lint
npm run lint
```

---

## Pull Request Process

### Branch Naming

Use descriptive branch names that follow this pattern:

```
feat/add-slack-alert-filters
fix/mysql-delta-overflow
chore/update-go-deps
docs/improve-quickstart
```

### Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short description>

<optional longer body>
```

**Types:**

| Type | When to use |
|------|-------------|
| `feat` | New feature or behavior |
| `fix` | Bug fix |
| `refactor` | Code change that isn't a feature or fix |
| `docs` | Documentation only |
| `test` | Adding or fixing tests |
| `chore` | Build, CI, dependency updates |
| `perf` | Performance improvement |

**Examples:**
```
feat: add PagerDuty notification channel
fix: prevent duplicate incidents on rapid check failure
docs: add Slack alert setup guide to README
```

### PR Description Requirements

Every PR should clearly explain:

1. **What** changed — a summary of the changes
2. **Why** — the motivation or problem being solved
3. **How to test** — steps a reviewer can follow to verify the fix or feature
4. **Screenshots** — for UI changes, include before/after screenshots

### What Makes a Good PR

- **Small and focused** — one feature or fix per PR. Easier to review, easier to revert.
- **Tests included** — new behavior should have tests; bug fixes should include a regression test.
- **Passes CI** — all tests green, no type errors, no lint warnings.
- **No unrelated changes** — don't sneak in refactors or style fixes unrelated to the PR's purpose.
- **Clear description** — a reviewer shouldn't need to guess what the PR does or why.

---

## Project Structure

```
healthops/
├── backend/internal/monitoring/   # Core Go packages — most backend work goes here
├── frontend/src/
│   ├── pages/                     # One file per page
│   ├── components/                # Reusable UI components
│   └── api/                       # API client modules
└── docs/                          # Architecture decisions, runbook, deployment guide
```

See [CLAUDE.md](CLAUDE.md) for a detailed map of every file in the backend.

---

## Questions?

Open a [GitHub Discussion](https://github.com/varaprasadreddy9676/healthops/discussions) or comment on an existing issue. We're happy to help.
