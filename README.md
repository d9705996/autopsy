# Autopsy

**Open-source incident response management platform.**

Autopsy helps on-call teams detect, manage, and retrospect on outages with structured timelines, status pages, and AI-assisted analysis.

[![CI](https://github.com/d9705996/autopsy/actions/workflows/test.yml/badge.svg)](https://github.com/d9705996/autopsy/actions/workflows/test.yml)
[![Lint](https://github.com/d9705996/autopsy/actions/workflows/lint.yml/badge.svg)](https://github.com/d9705996/autopsy/actions/workflows/lint.yml)

---

## Quick start (5 minutes)

### Prerequisites

- Go 1.25+
- Node.js 22+ (for the React UI)

> **No Docker or Postgres required.** By default Autopsy uses an embedded SQLite database — just clone and run.

### Run locally

```bash
git clone https://github.com/d9705996/autopsy.git
cd autopsy
cp .env.example .env        # DB_DRIVER=sqlite is the default
make dev                    # starts Go backend + Vite dev server
```

Open <http://localhost:5173> in your browser.

The seed admin credentials are printed to stdout on first boot:

```
admin@autopsy.local / <generated-password>
```

### Use PostgreSQL instead (optional)

Edit `.env` and set:

```dotenv
DB_DRIVER=postgres
DB_DSN=postgres://autopsy:autopsy@localhost:5432/autopsy?sslmode=disable
```

Then run `make dev` as normal. Postgres migrations are applied automatically on startup.

### Health check

```bash
curl http://localhost:8080/api/v1/health
```

---

## Architecture

| Layer | Technology |
|---|---|
| HTTP server | Go 1.25+ `net/http` `ServeMux` |
| Database | SQLite (default, embedded) or PostgreSQL 17 via GORM |
| Migrations | AutoMigrate (SQLite) · `golang-migrate` embedded SQL (Postgres) |
| Auth | JWT HS256 (stateless) + refresh token store |
| Job queue | River (Postgres only) · no-op queue when using SQLite |
| Observability | `log/slog` + OpenTelemetry (OTLP traces + Prometheus metrics) |
| Frontend | React 19 + PatternFly 6 + Vite 6 |
| Container | Distroless `static-debian12:nonroot` |

See [docs/development.md](docs/development.md) for a full development guide.

---

## Configuration

All configuration is via environment variables.  Copy `.env.example` and
customise:

| Variable | Default | Description |
|---|---|---|
| `DB_DRIVER` | `sqlite` | Database backend: `sqlite` or `postgres` |
| `DB_FILE` | `autopsy.db` | SQLite database file path (SQLite only) |
| `DB_DSN` | — | PostgreSQL connection string (required when `DB_DRIVER=postgres`) |
| `JWT_SECRET` | — **required** | JWT signing secret (min 32 chars) |
| `HTTP_PORT` | `8080` | HTTP listener port |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `json` | `json` (prod) or `text` (dev) |
| `JWT_ACCESS_TTL` | `15m` | JWT access token lifetime |
| `JWT_REFRESH_TTL` | `720h` | Refresh token lifetime (30 days) |
| `WORKER_CONCURRENCY` | `10` | River worker concurrency (Postgres only) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(empty)* | OTLP gRPC endpoint; leave empty to disable |
| `AI_PROVIDER` | `noop` | `noop` / `openai` / `anthropic` |

---

## Make targets

```
make build            Compile the binary (embeds SPA)
make dev              Start Go backend + Vite hot-reload (SQLite by default)
make dev-api          Run backend only with air hot-reload
make test             Run Go tests (race detector enabled)
make test-cover       Run tests and open HTML coverage report
make lint             Run golangci-lint + ESLint
make docker-build     Build the Docker image
make migrate-up       Apply pending Postgres migrations (DB_DRIVER=postgres only)
make release-snapshot GoReleaser dry-run
make clean            Remove build artefacts
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

[MIT](LICENSE)
