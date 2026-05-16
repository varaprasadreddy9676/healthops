# Changelog

All notable changes to HealthOps will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Check edit UI — edit existing checks directly from the dashboard
- Notification logs pagination — load more logs on demand
- SSH host key fingerprint verification — prevent MitM attacks on remote checks
- Login rate limiting — 10 attempts per minute per IP
- Security headers — X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy
- GitHub Actions CI/CD pipeline
- OpenAPI 3.0 specification

### Fixed
- Notification logs endpoint now requires authentication
- Scheduler goroutine timer leak on shutdown

## [0.1.0] - 2024-01-01

### Added
- **Health check engine** — HTTP/API, TCP, process, command, log, MySQL, and SSH check types
- **AI-powered incident analysis** — Bring Your Own Key (OpenAI, Anthropic, Google Gemini, Ollama, custom)
- **Notification channels** — Email, Slack, Discord, Telegram, Webhook, PagerDuty with smart filters
- **Incident management** — Full lifecycle (open → acknowledge → resolve) with MTTA/MTTR analytics
- **MySQL deep monitoring** — Live SHOW GLOBAL STATUS/VARIABLES, delta metrics, 9 built-in alert rules
- **SSH remote server monitoring** — Process, command, and connectivity checks over SSH
- **React dashboard** — Real-time SSE updates, dark mode, charts (Recharts), 8 pages
- **MongoDB persistence** — Optional mirror with automatic file-based fallback
- **Prometheus metrics** — Check runs, failures, durations, HTTP requests at `/metrics`
- **Audit logging** — All mutations tracked with actor and timestamp
- **JWT authentication** — Role-based access (admin/viewer)
- **User management** — Create, update, delete users via API and UI
- **Alert rules engine** — 5 default rules, configurable thresholds, cooldowns, consecutive-breach logic
- **Docker Compose** — Production stack (HealthOps + MongoDB) and demo stack with realistic targets
- **62+ REST API endpoints** — Full CRUD for all resources

[Unreleased]: https://github.com/varaprasadreddy9676/healthops/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/varaprasadreddy9676/healthops/releases/tag/v0.1.0
