# Development Guide

## Prerequisites

| Tool | Minimum version | Install |
|---|---|---|
| Go | 1.25 | <https://go.dev/dl/> |
| Docker | 24 | <https://docs.docker.com/get-docker/> |
| Node.js | 22 | [nvm](https://github.com/nvm-sh/nvm) recommended |
| make | any | `apt install make` / `brew install make` |

> **Dev container:** open the repo in VS Code with the Remote – Containers extension and all tools are pre-installed.

---

## First-time setup

```bash
git clone https://github.com/d9705996/autopsy.git
cd autopsy
go mod download
npm install --prefix web
cp .env.example .env
```

---

## Running the stack

```bash
make dev
```

This starts:
- **Postgres 17** via Docker Compose (`compose.yaml`) on `:5432`
- **Go backend** via `air` hot-reload on `:8080`
- **Vite dev server** on `:5173` (proxies `/api` to `:8080`)

Visit <http://localhost:5173>.

---

## Building a production binary

```bash
make build        # compiles ui/dist into the binary via go:embed
./bin/autopsy
```

---

## Running tests

```bash
make test                   # unit tests (no Postgres needed)
make test-integration       # integration tests (spawns testcontainers)
make test-cover             # opens HTML coverage report
```

---

## Database migrations

Migrations live in `internal/db/migrations/` as embedded SQL files.  They run
automatically on startup.  To apply manually:

```bash
make migrate-up
```

To roll back the last migration:

```bash
make migrate-down
```

---

## Code structure

```
cmd/autopsy/          — main package; wires everything
internal/
  api/
    handler/          — HTTP handler implementations
    jsonapi/          — JSON:API 1.1 envelope types
    middleware/       — request middleware (auth, logging)
    router.go         — route registration
  auth/               — JWT issuing, parsing, refresh tokens
  config/             — env-based config
  db/                 — pgx pool + migrate runner
    migrations/       — embedded SQL migration files
  health/             — /health and /ready handlers
  observability/      — slog + OTel bootstrap
  seed/               — seed admin user on first boot
  version/            — build-time version variables
  worker/             — River queue bootstrap
ui/                   — go:embed wrapper for ui/dist
web/                  — React 19 + PatternFly 6 source
  src/
    pages/            — route-level page components
charts/autopsy/       — Helm chart
docs/
  adr/               — Architecture Decision Records
```

---

## Environment variables

See [.env.example](../.env.example) for the full list with descriptions.

---

## Architecture Decision Records

| ADR | Title |
|---|---|
| [0001](adr/0001-go-stdlib-http.md) | Use Go stdlib ServeMux |
| [0002](adr/0002-jwt-stateless.md) | Stateless JWT with refresh token blocklist |
| [0003](adr/0003-postgres-only.md) | Postgres as the only data store |
| [0004](adr/0004-river-queue.md) | River for background jobs |
| [0005](adr/0005-patternfly.md) | PatternFly 6 for the React UI |
| [0006](adr/0006-goreleaser.md) | GoReleaser + Milestone trigger |
| [0007](adr/0007-distroless.md) | Distroless runtime image |
| [0008](adr/0008-otel.md) | OpenTelemetry for traces and metrics |
| [0009](adr/0009-slog.md) | log/slog for structured logging |
| [0010](adr/0010-embed.md) | SPA served via go:embed |
| [0011](adr/0011-jsonapi.md) | JSON:API 1.1 response envelope |
