# autopsy

Autopsy is a Go-based incident response management platform inspired by Grafana IRM, packaged as a **single binary** that serves both API and web UI.

## Capabilities

- Alert ingestion endpoint (`/api/alerts`) for Grafana webhooks today, extensible to any webhook source.
- AI-assisted alert triage that adds context and likely root-cause hints.
- Automatic incident creation and status-page URL generation for critical alerts, with public status updates whenever incidents are declared.
- Postmortem and playbook management aligned with Google SRE handbook concepts:
  - SLO/error-budget aware triage guidance
  - standard incident lifecycle
  - learning-focused follow-up records
- On-call scheduling with escalation paths.
- Local user authentication with seeded admin account on startup.
- Role-based access control (RBAC) with extensible roles and permission lists.
- Admin panel for user/role/invite management.
- Persistent storage with configurable database backend:
  - SQLite for local development
  - PostgreSQL for multi-instance/runtime deployments

## Run locally

```bash
go run .
```

Default credentials: `admin/admin`.

Environment variables:
- `AUTOPSY_ADMIN_USER` (default: `admin`)
- `AUTOPSY_ADMIN_PASSWORD` (default: `admin`)
- `AUTOPSY_AUTH_SECRET` (default: `autopsy-dev-secret`)
- `AUTOPSY_DB_DRIVER` (default: `sqlite`)
- `AUTOPSY_DB_DSN` (default: `file:autopsy.db?_pragma=busy_timeout(5000)`)

Use PostgreSQL:

```bash
AUTOPSY_DB_DRIVER=postgres \
AUTOPSY_DB_DSN='postgres://postgres:postgres@localhost:5432/autopsy?sslmode=disable' \
go run .
```

Open: <http://localhost:8080>
Public status page: <http://localhost:8080/status>


Quick local quality gate:

```bash
make check
```

PR guard (recommended before committing feature work):

```bash
bash .codex/skills/autopsy-pr-guard/scripts/pre_pr_check.sh
```

## API overview

- `POST /api/login`
- `POST /api/logout`
- `GET /api/me`
- `GET|POST /api/alerts`
- `GET /api/incidents`
- `GET /api/statuspage` (public, no auth)
- `GET|POST /api/postmortems`
- `GET|POST /api/playbooks`
- `GET|POST /api/oncall`
- `GET|POST /api/admin/users`
- `GET|POST /api/admin/roles`
- `GET|POST /api/admin/invites`

## CI/CD

GitHub Actions is split into dedicated workflows:
- `lint`: runs formatting checks and `golangci-lint`.
- `test`: runs Go unit/integration tests with coverage.
- `docker-release`: builds and pushes a GHCR image only when a git tag like `v1.2.3` is created; the Docker image tag matches `github.ref_name`.
