# Security Policy

## Supported Versions

Only the latest release and the `main` branch receive security fixes.

| Version | Supported |
|---------|-----------|
| latest  | Yes       |
| older   | No        |

## Reporting a Vulnerability

**Please do not file a public GitHub issue for security vulnerabilities.**

Report security issues privately using GitHub's built-in vulnerability reporting:

**[Report a vulnerability](https://github.com/varaprasadreddy9676/healthops/security/advisories/new)**

### What to include

- HealthOps version (or commit SHA)
- Deployment method (Docker, bare metal)
- Steps to reproduce the vulnerability
- Potential impact assessment
- Any proof-of-concept code (responsible disclosure only)

### Response timeline

- **Acknowledge** within 72 hours
- **Assessment** within 7 days
- **Fix + release** for critical issues within 30 days

We will credit researchers in the release notes unless you prefer to remain anonymous.

## Security Best Practices for Operators

- Set a strong `HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD` on first run
- Run behind a reverse proxy (nginx, Caddy) with TLS — see the [deployment guide](docs/deployment-guide.md)
- Set `HEALTHOPS_PUBLIC_URL` to your actual domain so notification links work correctly
- Rotate the admin password after initial setup via the Users page
- Do not expose the MongoDB port (`27017`) to the internet — it is internal to the Docker network by default
- Use SSH key auth (not passwords) for remote server checks; set `hostKeyFingerprint` to prevent MitM attacks
- Restrict `command` check type: `allowCommandChecks` is `false` by default — only enable if needed
