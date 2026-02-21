# autopsy

Autopsy is a Go-based incident response management platform inspired by Grafana IRM, packaged as a **single binary** that serves both API and web UI.

## Capabilities

- Alert ingestion endpoint (`/api/alerts`) for Grafana webhooks today, extensible to any webhook source.
- AI-assisted alert triage that adds context and likely root-cause hints.
- Automatic incident creation and status-page URL generation for critical alerts.
- Postmortem and playbook management aligned with Google SRE handbook concepts:
  - SLO/error-budget aware triage guidance
  - standard incident lifecycle
  - learning-focused follow-up records
- On-call scheduling with escalation paths.
- Session-based login for authenticated API usage.

## Run locally

```bash
go run .
```

Default credentials: `admin/admin` (override with `AUTOPSY_ADMIN_USER` and `AUTOPSY_ADMIN_PASSWORD`).

Open: <http://localhost:8080>

## API overview

- `POST /api/login`
- `GET|POST /api/alerts`
- `GET /api/incidents`
- `GET|POST /api/postmortems`
- `GET|POST /api/playbooks`
- `GET|POST /api/oncall`

## CI/CD

GitHub Actions workflow runs formatting checks, linting, tests, and `gosec`, then builds Docker images and publishes to GHCR on non-PR events.
