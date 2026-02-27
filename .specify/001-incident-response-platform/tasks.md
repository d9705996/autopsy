# Tasks: Incident Response Management Platform

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Date**: 2026-02-25  
**Status**: Ready for implementation

---

## How to use this file

- Each task is one **atomic unit of work** — one PR, typically 0.5–2 days.
- Tasks within a phase can run in **parallel** unless a dependency is stated.
- **Done when** items serve as your PR self-review checklist.
- Convert tasks to GitHub Issues with `/speckit.taskstoissues`.

---

## Phase 0: Foundation (US0)

> Goal: Any contributor can clone, build, and reach a passing `GET /api/v1/health` within 5 minutes.

---

### T001 — Go module, project skeleton & version package

**Spec**: FR-000, US0  
**Plan refs**: Project Structure, Key Go Dependencies

Stand up the canonical Go source tree so that all other tasks have a place to land. No business logic — only the shape.

**Done when**:
- [ ] `go.mod` declares module `github.com/d9705996/autopsy`, Go 1.25.
- [ ] Directory tree matches plan: `cmd/autopsy/`, `internal/{api,config,db,health,worker,observability}/`, `ui/dist/` (`.gitkeep`).
- [ ] `internal/version/version.go` exposes `Version`, `Commit`, `Date` string vars (populated by goreleaser ldflags; default to `dev`/`unknown`/`unknown`).
- [ ] `go build ./...` succeeds with zero errors.
- [ ] `go vet ./...` is clean.

---

### T002 — Config package (env-based, no Viper)

**Spec**: FR-000, US0  
**Plan refs**: Technical Context (no Viper; stdlib + envconfig)  
**Depends on**: T001

Parse all runtime configuration from environment variables. No config files, no third-party config framework.

**Done when**:
- [ ] `internal/config/config.go` defines a `Config` struct covering: `HTTP.Port` (default `8080`), `DB.Driver` (`postgres` | `sqlite`, default `sqlite`), `DB.DSN` (default `./data/autopsy-dev.db` when driver is sqlite), `DB.MaxConns`, `Log.Level` (default `info`), `Log.Format` (default `json`), `JWT.Secret`, `JWT.AccessTTL` (default `15m`), `JWT.RefreshTTL` (default `720h`), `AI.Provider`, `AI.APIKey`, `App.SeedAdminEmail`, `App.SeedAdminPassword`.
- [ ] `Config.DB.DSN` is **not** required when `DB.Driver=sqlite` (auto-defaults); it **is** required when `DB.Driver=postgres`.
- [ ] `config.Load()` returns a validated `*Config` and a non-nil error if any required field is absent.
- [ ] Unit tests: missing DSN with postgres driver returns error; sqlite driver defaults DSN correctly.

---

### T003 — Observability bootstrap (slog + OpenTelemetry)

**Spec**: FR-000, US0  
**Plan refs**: Technical Context (Observability); Constitution §III  
**Depends on**: T002

Wire structured logging and OpenTelemetry SDK before any HTTP traffic is served.

**Done when**:
- [ ] `internal/observability/otel.go` initialises the OTel SDK: OTLP trace exporter (configurable endpoint; defaults to no-op when unset), Prometheus metrics exporter on `:9090/metrics`.
- [ ] `internal/observability/logger.go` constructs a `*slog.Logger`: JSON handler in production (`LOG_FORMAT=json`), text handler in development.
- [ ] `Shutdown(ctx)` drains exporters gracefully (called on SIGTERM/SIGINT in main).
- [ ] Unit tests: `Load()` with `LOG_FORMAT=text` returns a text handler; OTel no-op path exercises without panic.

---

### T004 — HTTP server bootstrap & graceful shutdown

**Spec**: FR-000e, FR-000f, US0  
**Plan refs**: Technical Context (HTTP Router = net/http stdlib)  
**Depends on**: T002, T003

Start a `net/http` server with the `ServeMux`, register a catch-all 404, and shut down cleanly on signal.

**Done when**:
- [ ] `cmd/autopsy/main.go` wires config → observability → database → server → graceful shutdown in ≤ 100 lines.
- [ ] Server uses `http.ServeMux` (Go 1.22+ method+pattern routing). No chi, no gorilla.
- [ ] `SIGTERM`/`SIGINT` triggers `server.Shutdown(ctx)` with a 30-second timeout.
- [ ] Integration test: server starts, responds to `GET /`, shuts down cleanly.

---

### T005 — `/api/v1/health` and `/api/v1/ready` endpoints

**Spec**: FR-000e, FR-000f, SC-000, US0  
**Plan refs**: Foundation UI Layout (`GET /api/v1/health` response shape)  
**Depends on**: T004

Expose liveness and readiness probes. Probes must be reachable before any database migration.

**Done when**:
- [ ] `GET /api/v1/health` returns `200 OK` with JSON:API envelope:
  ```json
  { "data": { "type": "health", "id": "1",
      "attributes": { "status": "ok", "version": "...", "uptime_seconds": 42,
                      "commit": "...", "build_date": "..." } } }
  ```
- [ ] `GET /api/v1/ready` pings PostgreSQL (`SELECT 1`) and returns `200 OK` when reachable; `503 Service Unavailable` (JSON:API error body) otherwise.
- [ ] Unit tests cover healthy and degraded states by injecting a mock DB checker.
- [ ] Both routes are registered before database migration in startup sequence.

---

### T006 — Database connection + GORM dialect abstraction

**Spec**: FR-000, US0  
**Plan refs**: Technical Context (Database, ORM, Migrations); ADR 0012  
**Depends on**: T002

Open a database connection using GORM v2 with the driver selected by `DB_DRIVER`. The rest of the application interacts only with `*gorm.DB`; the concrete driver is opaque.

**Done when**:
- [ ] `internal/db/db.go` opens a `*gorm.DB` based on `Config.DB.Driver`:
  - `postgres` → `gorm.io/driver/postgres` (wrapping `pgx/v5/stdlib`); pool limits set via `db.DB()` (`MaxOpenConns`, `MaxIdleConns`).
  - `sqlite` → `gorm.io/driver/sqlite` via `glebarez/sqlite` (pure Go, no CGO); enables WAL mode and `_foreign_keys=on` pragma.
- [ ] `db.New(ctx, cfg)` returns `(*gorm.DB, error)`; exported function used by `main.go`.
- [ ] A `Ping(ctx, db)` helper executes a dialect-safe health check (`SELECT 1`) and is used by the readiness probe.
- [ ] `DB.MaxConns` is configurable for Postgres; SQLite ignores it (single-file, no pool limit needed).
- [ ] Unit test: SQLite driver opens without error; Ping returns nil.
- [ ] Integration test (testcontainers-go, `//go:build integration`): real Postgres opens, Ping returns nil.

---

### T007 — Foundation migrations: `organizations`, `users`, `refresh_tokens`

**Spec**: FR-000f, FR-000j, FR-015a, FR-015b, US0  
**Plan refs**: Key Entities (Organization, User); ADR 0011; ADR 0009; ADR 0012  
**Depends on**: T006

Create the three tables required by auth, plus an `Organization` table as the multi-tenancy schema hook. Schema creation uses different mechanisms per driver (see ADR 0012).

**Done when**:

**SQLite (GORM AutoMigrate)**:
- [ ] `internal/model/` package defines `Organization`, `User`, and `RefreshToken` GORM model structs with all required fields and GORM struct tags.
- [ ] `Organization`: `ID` (UUID, PK), `Name`, `Slug` (unique index), `CreatedAt`.
- [ ] `User`: `ID` (UUID, PK), `OrganizationID` (nullable), `Email` (unique index), `Name`, `PasswordHash`, `Roles` (JSON array), `OIDCSub`, `CreatedAt`, `UpdatedAt`.
- [ ] `RefreshToken`: `ID` (UUID, PK), `UserID` (FK via GORM BelongsTo), `TokenHash` (unique index), `ExpiresAt`, `RevokedAt` (nullable), `CreatedAt`.
- [ ] `db.AutoMigrate(db)` calls GORM `AutoMigrate` on all three models; called on startup when `DB_DRIVER=sqlite`.

**PostgreSQL (golang-migrate SQL files)**:
- [ ] `internal/db/migrations/postgres/0001_organizations.up.sql` and `0001_organizations.down.sql`.
- [ ] `internal/db/migrations/postgres/0002_users.up.sql` and `0002_users.down.sql`.
- [ ] `internal/db/migrations/postgres/0003_refresh_tokens.up.sql` and `0003_refresh_tokens.down.sql`.
- [ ] `db.Migrate(dsn)` applies pending `golang-migrate` SQL migrations; called on startup when `DB_DRIVER=postgres`.
- [ ] All SQL migration tables include `organization_id UUID NULL` (no FK, no enforcement) per FR-000j.
- [ ] Corresponding `.down.sql` files `DROP TABLE IF EXISTS` in reverse order.

**Shared**:
- [ ] `db.ApplySchema(ctx, db, cfg)` is the single startup function that routes to `AutoMigrate` or `Migrate` based on driver.
- [ ] Integration test: SQLite AutoMigrate — all three tables created, GORM can insert and query rows.
- [ ] Integration test (testcontainers-go): Postgres SQL migrations apply and roll back cleanly.

---

### T008 — JWT auth: issue, validate, refresh, revoke

**Spec**: FR-015a, FR-015b, US0  
**Plan refs**: ADR 0009; Technical Context (Auth)  
**Depends on**: T007

Implement the auth subsystem: token generation, validation middleware, refresh, and revocation.

**Done when**:
- [ ] `internal/auth/token.go`: `IssueAccessToken(userID, roles, secret, ttl)` → signed JWT (HS256); `ParseAccessToken(token, secret)` → claims or error.
- [ ] `internal/auth/refresh.go`: `IssueRefreshToken(userID)` → random 256-bit hex token stored (hashed with SHA-256) in `refresh_tokens`; `RefreshTokens(token)` validates, revokes old, issues new pair; `RevokeRefreshToken(token)` marks revoked.
- [ ] `internal/api/middleware/auth.go`: `RequireAuth` middleware validates access JWT, injects `*Claims` into context; returns `401` on missing/invalid/expired token.
- [ ] `POST /api/v1/auth/login`: accepts `{"email","password"}`; returns access + refresh tokens.
- [ ] `POST /api/v1/auth/refresh`: accepts `{"refresh_token"}`; returns new token pair.
- [ ] `POST /api/v1/auth/logout`: revokes the supplied refresh token; returns `204`.
- [ ] Unit tests: expired token → 401; revoked refresh token → 401; valid flow → 200.
- [ ] Integration test: full login → refresh → logout → stale refresh token rejected.

---

### T009 — Seed admin user on first boot

**Spec**: FR-015b, US0  
**Depends on**: T007, T008

Create a default admin user when the `users` table is empty so the platform is usable without manual DB inserts.

**Done when**:
- [ ] On startup, after migrations, if `COUNT(*) FROM users = 0`:
  - Create user with email `SEED_ADMIN_EMAIL` (env; default `admin@autopsy.local`) and a cryptographically random 32-byte hex password.
  - Password is printed to stdout **once** in the format `[autopsy] seed admin password: <password>` (never stored in plain text).
  - User is assigned the `Admin` role.
- [ ] If `SEED_ADMIN_PASSWORD` env var is present, use that value instead of generating one.
- [ ] Unit test: idempotent — calling seed function twice does not create a second user.
- [ ] Startup log line confirms seed status: `"seed admin created"` or `"seed admin already exists"`.

---

### T010 — Job queue bootstrap with `worker.Queue` interface

**Spec**: FR-000, US0  
**Plan refs**: Technical Context (Job Queue); ADR 0012  
**Depends on**: T006

Abstract the background job queue behind a `worker.Queue` interface so the application is not coupled to River or any Postgres-specific queue when running with SQLite.

**Done when**:
- [ ] `internal/worker/queue.go` defines the `Queue` interface and `JobArgs` type:
  ```go
  type Queue interface {
      Enqueue(ctx context.Context, job JobArgs) error
      Start(ctx context.Context) error
      Stop(ctx context.Context) error
  }
  ```
- [ ] `internal/worker/noop.go` implements `noopQueue`: `Enqueue` logs at debug and returns nil; used when `DB_DRIVER=sqlite`.
- [ ] `internal/worker/river.go` implements `riverQueue` wrapping `riverqueue/river` + `riverpgxv5`; used when `DB_DRIVER=postgres`. River's schema migrations are applied via `rivermigrate` before `Start()`.
- [ ] `worker.New(ctx, cfg, db, pgxPool, log)` returns the correct `Queue` implementation based on `cfg.DB.Driver`; `pgxPool` is `nil` when `DB_DRIVER=sqlite`.
- [ ] A no-op `HealthCheckArgs` job is registered in the River implementation to validate queue wiring.
- [ ] Concurrency is configurable (`WORKER_CONCURRENCY`, default `10`); ignored by `noopQueue`.
- [ ] Unit test: `noopQueue.Enqueue` returns nil; `noopQueue.Start/Stop` return nil.
- [ ] Integration test (testcontainers-go, postgres driver): enqueue a health-check job, verify it runs within 5 seconds.

---

### T011 — Frontend scaffold: Vite + React 19 + PatternFly 6 + React Router

**Spec**: FR-000, US0  
**Plan refs**: Foundation UI Layout; Frontend Dev Experience  
**Depends on**: T001

Stand up the frontend project so all subsequent UI tasks have a working shell.

**Done when**:
- [ ] `web/` contains a Vite 6 + React 19 + TypeScript project with PatternFly 6 installed.
- [ ] `web/vite.config.ts` sets `build.outDir: '../ui/dist'` and proxies `/api` → `http://localhost:8080`.
- [ ] `web/src/App.tsx` renders a PatternFly `Page` with:
  - Top-level `Masthead` (logo "Autopsy", user dropdown).
  - Vertical `Nav` sidebar with all 8 routes from the plan: Dashboard, Alerts, Incidents, On-Call, SLOs, Status Page, Settings, 404.
  - Each route renders a PatternFly `EmptyState` placeholder with the route name as the title.
- [ ] `npm run build --prefix web` compiles to `ui/dist/` with zero TypeScript errors.
- [ ] ESLint + Prettier pass on all files in `web/src/`.
- [ ] `go:embed ui/dist/*` in `cmd/autopsy/main.go` serves the SPA via `http.FileServer`; `GET /` serves `index.html`.
- [ ] `npm run dev --prefix web` starts Vite dev server; proxy to `:8080` is reachable (verified manually after `make dev`).

---

### T012 — `compose.yaml`, `.env.example`, and local dev workflow

**Spec**: FR-000c, SC-009, US0  
**Plan refs**: Frontend Dev Experience; Database Abstraction Strategy; compose.yaml section  
**Depends on**: T006

Wire `compose.yaml` for Postgres-based dev, update `.env.example` with both driver flavours, and ensure `make dev` works with **zero infrastructure** by defaulting to SQLite.

**Done when**:
- [ ] `.env.example` documents both drivers:
  ```
  # Local dev (default — no Docker required)
  DB_DRIVER=sqlite
  DB_DSN=./data/autopsy-dev.db

  # Production-equivalent local dev
  # DB_DRIVER=postgres
  # DB_DSN=postgres://autopsy:autopsy@localhost:5432/autopsy?sslmode=disable
  ```
- [ ] `compose.yaml` at repo root defines a single service: `postgres` (image `postgres:17`, health-check, named volume) — used only when `DB_DRIVER=postgres`.
- [ ] `make dev` target: copies `.env.example` → `.env` if absent; detects whether Postgres is reachable (`pg_isready`) and auto-starts native Postgres if `DB_DRIVER=postgres`; otherwise skips the DB start; then runs `air` (hot-reload backend) + `npm run dev --prefix web` in parallel.
- [ ] `make dev-ui` target: `npm run dev --prefix web` (runs Vite dev server; assumes `make dev` is already running).
- [ ] `make stop` target: `docker compose down` if Docker is active; `sudo pg_ctlcluster 17 main stop` if native.
- [ ] **Primary success criterion**: a contributor with only Go + Node installed (no Docker) can reach `GET /api/v1/health` by running `make dev` with no other setup.

---

### T013 — Multi-stage Dockerfile

**Spec**: FR-000b, US0  
**Plan refs**: Dockerfile section  
**Depends on**: T011

Build a production-grade Docker image usable by both CI and end users.

**Done when**:
- [ ] `Dockerfile` uses three stages:
  1. `frontend-builder` (Node 22 alpine): `npm ci && npm run build`.
  2. `go-builder` (golang:1.25-alpine): copies `ui/dist/` from stage 1, `go build -trimpath -ldflags ...`, outputs `/autopsy`.
  3. `runtime` (gcr.io/distroless/static-debian12 nonroot): copies `/autopsy`, `ENTRYPOINT ["/autopsy"]`.
- [ ] Image runs as non-root.
- [ ] `docker build -t autopsy:dev .` succeeds locally.
- [ ] `docker run --rm -e DB_DSN=... autopsy:dev` starts and `/api/v1/health` responds.
- [ ] Image is multi-arch-capable (build args for `TARGETPLATFORM` are present; actual multi-arch build happens in CI via QEMU).

---

### T014 — CI: `lint.yml` workflow

**Spec**: FR-000a, FR-000h, US0  
**Plan refs**: Workflow: lint.yml; .golangci.yml  
**Depends on**: T001, T011

Gate every PR and push to `main` against the full linting suite.

**Done when**:
- [ ] `.github/workflows/lint.yml` runs `golangci-lint-action@v9` with `version: v2.3`, no `only-new-issues`, `--timeout=10m`.
- [ ] `.golangci.yml` is present with the configuration from the plan: v2 format, `enable-all: true`, `fast: false`, `issues.max-issues-per-linter: 0`, and the disabled-linter exclusions documented in the plan.
- [ ] Workflow also runs `eslint` and `prettier --check` on `web/src/`.
- [ ] Workflow passes on the current codebase (no pre-existing violations).

---

### T015 — CI: `test.yml` workflow

**Spec**: FR-000a, SC-007, US0  
**Plan refs**: Workflow: test.yml  
**Depends on**: T006

Run the full test suite in CI with a real Postgres service container.

**Done when**:
- [ ] `.github/workflows/test.yml` runs `go test -race -coverprofile=coverage.out ./...` against a `postgres:17` service container.
- [ ] Coverage upload step saves `coverage.out` as an artefact.
- [ ] Workflow fails if any test fails (including race detector violations).
- [ ] Frontend unit test step runs `npm test --prefix web -- --run` (Vitest).

---

### T016 — CI: `security.yml` workflow

**Spec**: FR-000a, SC-011, US0  
**Plan refs**: Workflow: security.yml  
**Depends on**: T001

Run `govulncheck` on every push and report findings as SARIF to the GitHub Security tab.

**Done when**:
- [ ] `.github/workflows/security.yml` runs `golang/govulncheck-action@v1` with `output-format: sarif`.
- [ ] SARIF output is uploaded via `github/codeql-action/upload-sarif@v3`.
- [ ] Workflow runs on push to `main` and on pull requests.

---

### T017 — CI: `semantic-pr.yml` workflow

**Spec**: FR-000a, US0  
**Plan refs**: Workflow: semantic-pr.yml

Enforce conventional commits on PR titles and all commits in a PR.

**Done when**:
- [ ] `.github/workflows/semantic-pr.yml` runs `amannn/action-semantic-pull-request@v6` to validate PR title.
- [ ] `wagoid/commitlint-github-action@v6` validates every commit in the PR.
- [ ] `commitlint.config.js` at repo root configures `@commitlint/config-conventional`.

---

### T018 — CI: `build.yml` workflow

**Spec**: FR-000a, US0  
**Plan refs**: Workflow: build.yml  
**Depends on**: T013

Compile the binary and build the Docker image on every PR to catch build breakage early.

**Done when**:
- [ ] `.github/workflows/build.yml` runs `go build -v ./...` and `docker buildx build --platform linux/amd64,linux/arm64 --no-push .`.
- [ ] Build artefact (binary) is uploaded as a workflow artefact for inspection.

---

### T019 — CI: `release.yml` workflow & `.goreleaser.yml`

**Spec**: FR-000b, US0  
**Plan refs**: Workflow: release.yml; .goreleaser.yml  
**Depends on**: T013

Automate the full release pipeline triggered by closing a GitHub Milestone.

**Done when**:
- [ ] `.github/workflows/release.yml` triggers on `milestone.closed`, validates milestone title is semver, creates and pushes a `vX.Y.Z` git tag, then runs goreleaser.
- [ ] `.goreleaser.yml` builds `linux/amd64` + `linux/arm64` + `darwin/amd64` + `darwin/arm64` + `windows/amd64`; injects version ldflags; creates archives with `README.md` + `LICENSE`; publishes Docker image to `ghcr.io/d9705996/autopsy:vX.Y.Z` and `:latest`.
- [ ] `GITHUB_TOKEN` is used for both the tag push and goreleaser (no PAT required).
- [ ] Dry-run test: `goreleaser release --snapshot --clean` succeeds locally (documented in `docs/development.md`).

---

### T020 — Helm chart skeleton

**Spec**: FR-000d, US0  
**Plan refs**: Project Structure (`charts/autopsy/`)

Provide a minimal but deployable Helm chart so the Kubernetes acceptance scenario can pass.

**Done when**:
- [ ] `charts/autopsy/Chart.yaml`, `values.yaml`, `templates/` with: `deployment.yaml`, `service.yaml`, `ingress.yaml` (disabled by default), `configmap.yaml`, `secret.yaml` (external secret ref).
- [ ] Chart lints cleanly with `helm lint charts/autopsy`.
- [ ] `helm template charts/autopsy` renders without errors with default values.
- [ ] `values.yaml` exposes: `image.repository`, `image.tag`, `replicaCount`, `ingress.enabled`, `ingress.host`, `resources`, `env` (for `DB_DSN` etc.), `serviceAccount.create`.
- [ ] Chart has a readiness probe wired to `GET /api/v1/ready` and a liveness probe wired to `GET /api/v1/health`.

---

### T021 — Makefile with all required targets

**Spec**: FR-000, US0  
**Plan refs**: Makefile section  
**Depends on**: T012, T013

Single entry-point for every developer action. Documented in `docs/development.md`.

**Done when**:
- [ ] `Makefile` targets: `build`, `test`, `lint`, `dev`, `dev-ui`, `stop`, `docker-build`, `release` (dry-run), `migrate-up`, `migrate-down`, `generate`, `clean`.
- [ ] `make help` prints a formatted usage summary.
- [ ] `make build` compiles the binary to `bin/autopsy`.
- [ ] `make test` runs `go test -race -count=1 ./...` with `testcontainers-go` Postgres.
- [ ] `make lint` runs `golangci-lint run ./...` locally.
- [ ] Each target is self-contained (no undocumented prerequisites).

---

### T022 — Documentation: README, CONTRIBUTING, docs/

**Spec**: FR-000g, SC-000, US0  
**Plan refs**: ADR list

Write the "front door" documentation a new contributor encounters.

**Done when**:
- [ ] `README.md`: project overview, status badge, quick-start (3 steps: clone, `make dev`, open browser), architecture diagram (Mermaid), links to CONTRIBUTING and docs/.
- [ ] `CONTRIBUTING.md`: development environment setup, branching strategy (`feature/...`, `fix/...`, `chore/...`), PR process, conventional commit rules, how to run tests, how to cut a release.
- [ ] `docs/development.md`: detailed local setup, environment variables reference, migration workflow, linting rules, hot-reload workflow, goreleaser dry-run, devcontainer instructions.
- [ ] `docs/adr/0001-record-architecture-decisions.md`: foundation ADR documenting the ADR format itself.
- [ ] `docs/adr/0002-postgresql-as-primary-store.md` through `docs/adr/0011-organization-id-schema-hook.md`: all ADRs from the plan.
- [ ] `CHANGELOG.md`: initialised with `## [Unreleased]` section.

---

### T023 — `.devcontainer/devcontainer.json`

**Spec**: FR-000i, US0

Configure a Codespaces / VS Code Dev Container so contributors need zero local tool installation.

**Done when**:
- [ ] `.devcontainer/devcontainer.json` uses a base image with Go 1.25 + Node 22 pre-installed (or `mcr.microsoft.com/devcontainers/go:1.25` + Node feature).
- [ ] `postCreateCommand` runs: `go mod download && npm ci --prefix web && cp .env.example .env`.
- [ ] VS Code extensions in `customizations.vscode.extensions`: `golang.go`, `esbenp.prettier-vscode`, `dbaeumer.vscode-eslint`, `ms-azuretools.vscode-docker`.
- [ ] `forwardPorts`: `[8080, 5173, 5432]`.
- [ ] Opening the repo in Codespaces and running `make dev` requires no additional manual steps.

---

### T024 — JSON:API envelope types (`internal/api/jsonapi/`)

**Spec**: FR-034, US0  
**Plan refs**: ADR I6 (hand-written ~200-line envelope)  
**Depends on**: T001

Implement the JSON:API 1.1 serialisation layer used by every handler. No external library.

**Done when**:
- [ ] `internal/api/jsonapi/` contains:
  - `document.go`: `Document[T]`, `ListDocument[T]` generic types; `ResourceObject`, `Relationship`, `Links`, `Meta`, `Pagination`.
  - `errors.go`: `ErrorDocument`, `ErrorObject` (with `status`, `code`, `title`, `detail`, `source`).
  - `marshal.go`: `Marshal(data any, meta ...Meta) ([]byte, error)` and `MarshalList(data []any, pagination *Pagination, meta ...Meta) ([]byte, error)`.
  - `render.go`: `Render(w, status, doc)` helper that sets `Content-Type: application/vnd.api+json` and encodes.
  - `errors_render.go`: `RenderError(w, status, code, title, detail string)` and `RenderErrors(w, status, errs []ErrorObject)`.
- [ ] All error responses in the health handler are migrated to `RenderError`.
- [ ] Unit tests: round-trip marshal/unmarshal for a sample resource type; error document shape matches JSON:API 1.1 spec.

---

## Phase 0 Addendum: Database Abstraction

> These tasks were added to the plan in response to the requirement for interchangeable database drivers (ADR 0012). They extend Phase 0 and must be completed before any PoC data-layer tasks.

---

### T024a — GORM integration: refactor DB, auth, seed, worker packages

**Spec**: FR-000, US0  
**Plan refs**: ADR 0012; Database Abstraction Strategy  
**Depends on**: T006 (updated), T007 (updated), T008, T009, T010 (updated)

Replace direct `pgxpool.Pool` and raw `pgx` usage in `internal/auth`, `internal/seed`, and `internal/db` with GORM `*gorm.DB` queries, so all packages work against both SQLite and Postgres.

**Done when**:
- [ ] `internal/auth/token.go` and `internal/auth/refresh.go` use GORM queries (no raw `pgx` or `sql.Row` scanning).
- [ ] `internal/seed/seed.go` uses GORM `First`/`Create` — no raw SQL.
- [ ] `internal/db/db.go` exposes `*gorm.DB` (not `*pgxpool.Pool`) as the primary DB handle; `pgxpool.Pool` is only created internally when `DB_DRIVER=postgres` and passed to the River worker.
- [ ] `cmd/autopsy/main.go` wires `*gorm.DB` through config → db → worker → handlers; never passes `*pgxpool.Pool` outside the worker package.
- [ ] `DB_DRIVER=sqlite make dev-api` starts the server, applies AutoMigrate, seeds admin, and serves `GET /api/v1/health` → 200 with no external processes running.
- [ ] `DB_DRIVER=postgres make dev-api` (with Postgres available) produces the same result via golang-migrate SQL migrations.
- [ ] All existing unit tests pass; new unit tests added for SQLite code paths.
- [ ] Integration tests tagged `//go:build integration` pass for both drivers.

---

### T024b — Write ADR 0012: database driver abstraction

**Spec**: FR-000, US0  
**Plan refs**: ADR 0012; Constitution §V (docs shipped with feature)  
**Depends on**: T024a

Document the decision to support interchangeable database drivers as a MADR-format ADR.

**Done when**:
- [ ] `docs/adr/0012-database-driver-abstraction.md` exists and follows the MADR format used by ADR 0001–0011.
- [ ] ADR covers: context (zero-infrastructure dev requirement), decision (GORM v2 + driver injection), consequences (ORM magic, migration split, River constraint), alternatives considered (sqlx + raw SQL, ent, database/sql only).
- [ ] `README.md` quick-start section updated: `make dev` no longer lists Docker as a prerequisite.
- [ ] `.env.example` updated to default to `DB_DRIVER=sqlite` with the Postgres variant commented out.

---

## Phase 1: PoC (US1, US2)

> Goal: Alerts arrive via Grafana webhook → AI triages → SEV1/2 auto-declares an incident → incident visible in UI.

---

### T025 — Alert entity, migration & repository

**Spec**: FR-001–FR-005, US1  
**Depends on**: T006, T007, T024a

**Done when**:
- [ ] `internal/model/alert.go` defines the `Alert` GORM model with all attributes from the spec entity: `id`, `organization_id`, `source`, `fingerprint`, `title`, `labels` (JSON, stored as `TEXT` for SQLite / `JSONB` for Postgres via GORM serializer tag), `severity`, `triage_status`, `ai_summary`, `probable_cause`, `suggested_actions` (JSON array), `confidence_score`, `ai_prompt`, `ai_response`, `deduplicated`, `received_at`, `resolved_at`.
- [ ] `db.AutoMigrate` (SQLite) and `internal/db/migrations/postgres/0004_alerts.up.sql` (Postgres) both produce the same logical schema.
- [ ] `internal/domain/alert/` defines `Repository` interface (`CreateOrUpdate`, `GetByID`, `ListByFilter`, `MarkResolved`).
- [ ] `internal/domain/alert/gorm_repo.go` implements `Repository` using `*gorm.DB` — works against both SQLite and Postgres.
- [ ] Unit tests for deduplication logic (same fingerprint within window → update; outside window → new); tests run against SQLite (no container required).

---

### T026 — Webhook ingestion handler & HMAC middleware

**Spec**: FR-001, FR-002, FR-003, FR-004, FR-005, US1  
**Depends on**: T025, T008, T024

**Done when**:
- [ ] `POST /api/v1/webhooks/:source` registered; supported sources: `grafana`, `generic`.
- [ ] `internal/api/middleware/hmac.go`: reads `X-Hub-Signature-256` header, validates against source's configured secret from DB. Rejects with `401` on failure, proceeds on success or when no secret is configured.
- [ ] Grafana Alerting payload parser handles both `alerting` and `resolved` states; maps to `Alert` struct.
- [ ] Generic source parser accepts any JSON; maps fields to `Alert` using a configurable `field_mapping` JSON structure (stored on `WebhookSource`).
- [ ] Deduplication: look up `fingerprint` in the last `dedup_window_seconds` (default 300); if found, update existing alert and set `deduplicated: true`; do not enqueue a second triage job.
- [ ] Response: `202 Accepted` with the alert ID on success; `422` with JSON:API error body on invalid payload.
- [ ] Integration tests for both sources using a real Postgres via testcontainers.

---

### T027 — `WebhookSource` entity, migration & CRUD API

**Spec**: FR-001, FR-002, FR-005, US1  
**Depends on**: T024, T008

**Done when**:
- [ ] `0005_webhook_sources.up.sql`: table `webhook_sources`.
- [ ] `GET /api/v1/webhook-sources` (list), `POST` (create), `GET /:id` (read), `PATCH /:id` (update), `DELETE /:id` (delete) — all require `Admin` role.
- [ ] HMAC secret is stored as a bcrypt hash; never returned in API responses (redacted to `"***"`).
- [ ] Acceptance test: create source → POST to webhook endpoint with wrong HMAC → 401; with correct HMAC → 202.

---

### T028 — AI provider interface & OpenAI-compatible implementation

**Spec**: FR-006–FR-011, FR-015d, US1  
**Plan refs**: ADR 0010; Technical Context (AI Provider)  
**Depends on**: T002

**Done when**:
- [ ] `internal/ai/provider.go` defines:
  ```go
  type Provider interface {
      Triage(ctx context.Context, req TriageRequest) (TriageResult, error)
  }
  type TriageRequest struct { AlertTitle, Labels, Source string }
  type TriageResult struct {
      Severity       string
      Summary        string
      ProbableCause  string
      SuggestedActions []string
      ConfidenceScore  float64
      Prompt         string
      Response       string
  }
  ```
- [ ] `internal/ai/openai.go` implements `Provider` against the OpenAI Chat Completions API (or any OpenAI-compatible endpoint); endpoint and model configurable via `AI_API_BASE`, `AI_MODEL`.
- [ ] `internal/ai/noop.go` implements `Provider` that returns a canned "no AI provider configured" result; used when `AI_PROVIDER=noop`.
- [ ] `Config.AI.Provider` selects the implementation: `openai` (default) or `noop`.
- [ ] Unit tests mock HTTP client; test exponential retry on rate-limit (HTTP 429).

---

### T029 — AI triage River job

**Spec**: FR-006–FR-011, FR-009, US1  
**Depends on**: T010, T025, T028

**Done when**:
- [ ] `internal/worker/jobs/ai_triage.go` defines `AITriageArgs { AlertID uuid.UUID }`.
- [ ] Job fetches the alert, calls `Provider.Triage()`, updates the alert record with all triage fields including stored `ai_prompt` and `ai_response` (FR-011 explainability).
- [ ] On `Provider` error, job records `triage_status: "failed"` with error detail; logged with trace ID (FR-009: max 3 attempts with exponential backoff — configured on the River job args).
- [ ] On SEV1 or SEV2 result, job enqueues an `AutoDeclareIncidentArgs` job (see T030).
- [ ] Unit tests: mock provider returning each severity level; verify correct `triage_status` and incident enqueue decisions.
- [ ] Integration test: ingest alert → triage job runs → alert record updated within 30 s.

---

### T030 — Incident entity, migration & repository

**Spec**: FR-012–FR-015, US2  
**Depends on**: T006, T007, T024

**Done when**:
- [ ] `0006_incidents.up.sql`: table `incidents` and `timeline_entries` with all attributes from spec entities.
- [ ] Incident status enum: `declared`, `investigating`, `identified`, `monitoring`, `resolved`.
- [ ] TimelineEntry type enum: `state_change`, `ai_hypothesis`, `comment`, `page`, `action`.
- [ ] `internal/domain/incident/repository.go`: `Create`, `GetByID`, `UpdateStatus`, `ListByFilter`, `AppendTimeline`.
- [ ] Immutability enforced at repository level: `AppendTimeline` is insert-only; no update/delete on `timeline_entries`.
- [ ] Unit tests for status transition validation.

---

### T031 — Incident lifecycle API

**Spec**: FR-012–FR-016, SC-003, US2  
**Depends on**: T030, T008, T024

**Done when**:
- [ ] `POST /api/v1/incidents` — create manual incident; requires `incident:create` permission.
- [ ] `GET /api/v1/incidents` — list with cursor pagination (FR-039); `page[cursor]` + `page[size]`.
- [ ] `GET /api/v1/incidents/:id` — fetch with timeline embedded in `included`.
- [ ] `PATCH /api/v1/incidents/:id` — update status; forbidden transitions (e.g., re-opening resolved) return `409`; each transition appends a `state_change` `TimelineEntry`.
- [ ] `POST /api/v1/incidents/:id/timeline` — add a `comment` entry; any authenticated user.
- [ ] Resolving an incident (status → `resolved`) enqueues `CreatePostmortemStubArgs` (T051).
- [ ] All responses are JSON:API 1.1 compliant.
- [ ] Integration tests covering the full state machine including forbidden transitions and RBAC gating.

---

### T032 — Auto-declare incident from triage (River job)

**Spec**: FR-012, FR-015, US2  
**Depends on**: T029, T030

**Done when**:
- [ ] `AutoDeclareIncidentArgs { AlertID uuid.UUID }` River job: looks up the alert's triage result, creates an incident with `severity` from triage, links the alert.
- [ ] `SystemActorID` constant used as `commander_user_id` for AI-declared incidents; distinguishable in the timeline.
- [ ] Integration test: POST synthetic SEV1 webhook → triage job runs → incident auto-declared → `GET /api/v1/incidents` returns it.

---

### T033 — AI hypothesis River job

**Spec**: FR-016, US2  
**Depends on**: T029, T030, T028

**Done when**:
- [ ] `PostHypothesisArgs { IncidentID uuid.UUID }` River job: triggered when incident enters `investigating` state.
- [ ] Calls `Provider.Triage()` with enriched context (incident title + linked alerts + recent timeline).
- [ ] Posts result as a `TimelineEntry` of type `ai_hypothesis` with `content` containing hypothesis and confidence score.
- [ ] Integration test: incident transitions to `investigating` → hypothesis timeline entry appears.

---

### T034 — Alert list and incident list UI (React)

**Spec**: US1, US2  
**Depends on**: T011, T026, T031

Minimal read-only UI pages so the PoC demo is browser-visible.

**Done when**:
- [ ] `AlertsPage`: PatternFly `Table` listing alerts (ID, source, severity badge, triage status, received_at). Polling or manual refresh. Empty state when no alerts.
- [ ] `IncidentsPage`: PatternFly `Table` listing incidents (ID, severity badge, status, commander, declared_at). Clicking a row opens `IncidentDetailPage`.
- [ ] `IncidentDetailPage`: show incident attributes + timeline as a PatternFly `TimestampList`. AI hypotheses styled differently from human comments.
- [ ] All pages are protected behind the auth route guard (redirect to login if no JWT in memory).
- [ ] `LoginPage`: email + password form; calls `POST /api/v1/auth/login`; stores access token in-memory (not localStorage), refresh token in `HttpOnly` cookie.
- [ ] TypeScript strict mode; no `any` casts; ESLint clean.

---

## Phase 2: MVP (US3–US7)

> Goal: A small multi-team engineering organisation can self-host and use Autopsy as their primary IRM tool.

---

### T035 — On-call schedule entity, migration & API

**Spec**: FR-017, FR-018, US3  
**Depends on**: T008, T024

**Done when**:
- [ ] `0007_oncall.up.sql`: tables `oncall_schedules`, `oncall_layers`, `oncall_overrides`.
- [ ] `GET/POST /api/v1/oncall-schedules`, `GET/PATCH/DELETE /api/v1/oncall-schedules/:id`.
- [ ] `GET /api/v1/oncall-schedules/:id/current` returns the currently on-call user for the schedule at the current time.
- [ ] Override logic: if a manual override covers the current time, override user is returned.
- [ ] Unit tests: rotation calculation with DST boundary (edge case from spec).

---

### T036 — Escalation policy entity, migration & API

**Spec**: FR-018, US3  
**Depends on**: T035

**Done when**:
- [ ] `0008_escalation_policies.up.sql`: tables `escalation_policies`, `escalation_tiers`.
- [ ] `GET/POST /api/v1/escalation-policies`, `GET/PATCH/DELETE /:id`.
- [ ] Each tier has: `schedule_id`, `timeout_seconds`, `notification_channels[]`.

---

### T037 — Notification dispatch & escalation River job

**Spec**: FR-018, FR-019, FR-020, US3  
**Depends on**: T036, T010

**Done when**:
- [ ] `EscalationArgs { IncidentID, PolicyID uuid.UUID, TierIndex int }` River job.
- [ ] Dispatches notifications to all channels in the tier: webhook (Slack/Teams POST) and email (deferred to T041 per FR-015c; webhook is PoC-sufficient here).
- [ ] Records every page attempt as a `page` `TimelineEntry`.
- [ ] On no acknowledgement within `timeout_seconds`, enqueues next tier (or `CRITICAL_UNACKNOWLEDGED` event at final tier).
- [ ] `PATCH /api/v1/incidents/:id/acknowledge` sets `acknowledged_at`, `acknowledging_user`; cancels pending escalation jobs for the current tier.
- [ ] Integration test: incident declared → escalation job runs → page timeline entry created → acknowledge → further pages suppressed.

---

### T038 — On-Call UI page

**Spec**: US3  
**Depends on**: T035, T036, T034

**Done when**:
- [ ] `OnCallPage`: list schedules with current on-call user; create/edit schedule form (PatternFly `Modal`).
- [ ] Current on-call user highlighted with a PatternFly `Label`.

---

### T039 — Status page entities, migration & public endpoint

**Spec**: FR-021–FR-025, SC-004, US4  
**Depends on**: T030, T024

**Done when**:
- [ ] `0009_status_page.up.sql`: tables `components`, `component_groups`, `status_pages`, `status_subscriptions`.
- [ ] `GET /status` — public, unauthenticated, returns the rendered status page JSON (JSON:API); served with `Cache-Control: public, max-age=30`.
- [ ] `GET /status.rss` — valid RSS 2.0 with ten most recent incidents.
- [ ] Admin API: `CRUD /api/v1/components`, `CRUD /api/v1/status-pages`.
- [ ] Incident lifecycle hook: on `status: investigating`, linked components auto-update to `degraded_performance` or above based on severity; on `resolved`, revert to `operational`.
- [ ] Response time ≤ 500 ms from application cache (`patrickmn/go-cache`, 30-second TTL) on repeated requests (SC-004).
- [ ] Integration test: declare incident → component status changes; resolve → reverts.

---

### T040 — Status page UI

**Spec**: US4  
**Depends on**: T039, T034

**Done when**:
- [ ] `StatusPage` (public, no auth): PatternFly `Card` grid showing component status with colour-coded `Label` badges; active incident banner; historical incident list.
- [ ] "All Systems Operational" empty state when no active incidents.
- [ ] Subscribe form (email → `POST /api/v1/status-subscriptions`) — visible but email delivery deferred to T041.

---

### T041 — Email notification dispatch (MVP gate)

**Spec**: FR-019, FR-024, FR-015c, US3, US4  
**Depends on**: T037, T039

Email is deferred to MVP per decision Q3. This task gates its delivery.

**Done when**:
- [ ] `internal/notify/email.go`: `EmailSender` interface with `Send(ctx, to, subject, body string) error`.
- [ ] SMTP implementation using `net/smtp` (stdlib); configurable `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`.
- [ ] Escalation job (T037) calls `EmailSender` when `email` is in channel list; skips gracefully when `SMTP_HOST` is unset.
- [ ] Status page subscription emails sent on incident create/resolve.
- [ ] User onboarding email sent on `POST /api/v1/users` (FR-015c, US7).
- [ ] Integration test: mock SMTP server (mailhog or smtp4dev via testcontainers) receives expected messages.

---

### T042 — SLO/SLI entities, migration & evaluation job

**Spec**: FR-026–FR-029, US5  
**Depends on**: T025, T010

**Done when**:
- [ ] `0010_slos.up.sql`: tables `slos`, `sli_samples`.
- [ ] Admin API: `CRUD /api/v1/slos`.
- [ ] `SLOEvaluationArgs { SLOID uuid.UUID }` River job (scheduled periodically): queries Prometheus-compatible API at `sli_query`; calculates error budget remaining; persists sample.
- [ ] Fast-burn (1 h × 14) and slow-burn (6 h × 6) thresholds trigger `slo_burn_rate` alert via webhook ingestion path.
- [ ] On incident resolution, `incident_duration × affected_slo_weight` is attributed to error budget (FR-029).
- [ ] Integration test: synthetic SLI data → budget correctly calculated; fast-burn threshold crossed → alert created.

---

### T043 — SLO history endpoint

**Spec**: FR-026, US5  
**Depends on**: T042

**Done when**:
- [ ] `GET /api/v1/slos/:id/history?window=30d` returns a JSON array of daily `{ date, error_budget_consumed, error_budget_remaining }` objects.
- [ ] Window parameter supported: `7d`, `30d`, `90d`.

---

### T044 — SLO UI page

**Spec**: US5  
**Depends on**: T042, T043, T034

**Done when**:
- [ ] `SLOsPage`: PatternFly `Table` of SLOs with error budget bar (PatternFly `Progress` component), status badge (`ok` / `at_risk` / `exhausted`).
- [ ] Clicking an SLO shows a time-series sparkline (PatternFly Charts) of the 30-day history.

---

### T045 — Postmortem entity, migration & auto-stub

**Spec**: FR-030–FR-033, US6  
**Depends on**: T030, T042, T024

**Done when**:
- [ ] `0011_postmortems.up.sql`: tables `postmortems`, `action_items`.
- [ ] `CreatePostmortemStubArgs { IncidentID uuid.UUID }` River job (enqueued on incident resolution): creates `Postmortem` with `status: draft`, pre-populates timeline snapshot, linked alert IDs, AI hypothesis entries, impacted SLO IDs, incident metadata.
- [ ] Admin/Commander API: `GET/PATCH /api/v1/postmortems/:id`, `GET /api/v1/postmortems` (list).
- [ ] `POST /api/v1/postmortems/:id/publish` transitions to `published`; validates `root_cause` and at least one `action_item` are present.
- [ ] `GET /api/v1/action-items` with `?postmortem_id=` filter; `PATCH /api/v1/action-items/:id`.
- [ ] Published postmortems searchable via `GET /api/v1/postmortems?q=` full-text search (Postgres `tsvector`).

---

### T046 — AI context retrieval for triage (postmortem similarity)

**Spec**: FR-033, US6  
**Depends on**: T028, T029, T045

**Done when**:
- [ ] Triage job (T029) queries published postmortems for semantic similarity on `root_cause` (Postgres full-text `ts_rank` on `tsvector`).
- [ ] Matching postmortems (top 3 by rank score ≥ threshold) are added to `TriageResult.RelatedPostmortems`.
- [ ] `related_postmortems` field is stored on the alert and returned in the API response.

---

### T047 — Postmortem UI page

**Spec**: US6  
**Depends on**: T045, T034

**Done when**:
- [ ] `PostmortemsPage` (hidden in nav until MVP — placeholder until this task): PatternFly `Table` of postmortems by status.
- [ ] `PostmortemDetailPage`: structured form (root cause, contributing factors, action items); `Publish` button; read-only view when published.
- [ ] Action items rendered as PatternFly `CheckList` with owner and due date.

---

### T048 — RBAC: roles, permissions & middleware

**Spec**: FR-036, US7  
**Depends on**: T008, T024

**Done when**:
- [ ] Built-in roles defined as Go constants: `Viewer`, `Responder`, `IncidentCommander`, `Admin`.
- [ ] Permission table: every API route pair `(method, path pattern)` maps to a required permission string (e.g., `incident:create`, `incident:read`, `slo:write`).
- [ ] `internal/api/middleware/rbac.go`: `RequirePermission(perm string)` middleware reads `Claims.Roles` from context; checks against permission map; returns `403` with JSON:API error body on failure.
- [ ] All existing handlers are wrapped with appropriate `RequirePermission` middleware.
- [ ] Unit tests: `Viewer` → `POST /api/v1/incidents` → 403; `Responder` → same → 200.

---

### T049 — User management API

**Spec**: FR-036, US7  
**Depends on**: T048, T041

**Done when**:
- [ ] `POST /api/v1/users` — Admin only; creates user, sends onboarding email (T041).
- [ ] `GET /api/v1/users` — Admin only; list with pagination.
- [ ] `GET /api/v1/users/:id` — self or Admin.
- [ ] `PATCH /api/v1/users/:id` — Admin can update roles; self can update name/notification channels.
- [ ] `DELETE /api/v1/users/:id` — Admin only; sets `deactivated_at` (soft delete); does not cascade delete action items (ownership transferred to Admin).
- [ ] Integration test: Viewer calls `POST /api/v1/users` → 403; Admin calls → 201 → onboarding email received.

---

### T050 — Multi-tenancy enforcement (MVP)

**Spec**: FR-000j, US0 (enforcement in MVP stage), G3 decision  
**Depends on**: T048, T007

**Done when**:
- [ ] `Organization` CRUD API for Admin: `POST /api/v1/organizations`, `GET /api/v1/organizations`, `PATCH /:id`.
- [ ] JWT claims include `organization_id` (populated on login from user record).
- [ ] All repository queries that touch `organization_id`-bearing tables add `WHERE organization_id = $1` when `organization_id` is non-null in claims.
- [ ] Org isolation integration test: create two orgs, two users; verify org-A user cannot read org-B incidents.
- [ ] `organization_id` column constraint added via `0012_org_id_not_null.up.sql` (sets `NOT NULL DEFAULT <system-org-id>` for existing rows, then removes default).

---

### T051 — Audit log

**Spec**: FR-038, US7  
**Depends on**: T048, T024

**Done when**:
- [ ] `0013_audit_log.up.sql`: table `audit_log (id, organization_id, actor_id, action, resource_type, resource_id, before JSONB, after JSONB, created_at)`.
- [ ] `internal/api/middleware/audit.go`: `AuditMutations` middleware wraps all `POST/PATCH/DELETE` handlers; captures actor, before-state (GET before write), after-state (from response), and writes to `audit_log` in the same DB transaction where possible.
- [ ] `GET /api/v1/audit-log` — Admin only; filter by `resource_type`, `actor_id`, `from`, `to`; paginated.
- [ ] Unit test: mutation on incident inserts audit row with correct before/after JSON.

---

### T052 — OpenAPI 3.x spec generation & enforcement

**Spec**: FR-035, SC-012, US0 (shipped incrementally per endpoint)  
**Depends on**: T005

**Done when**:
- [ ] `docs/openapi.yaml` (or `.json`) is committed; covers all endpoints shipped through MVP.
- [ ] A `make generate` target runs a Go tool (e.g., hand-maintained with `go generate` + validation, or via `kin-openapi` for validation) to verify all registered routes have an OpenAPI operation.
- [ ] CI lint step fails if `docs/openapi.yaml` is out of date relative to registered routes.
- [ ] Each endpoint has: operation ID, summary, all request/response schemas, 4xx/5xx error schemas.

---

## Phase 3: Full Product (US8 + remaining FRs)

> Goal: Feature parity with commercial IRM tools; production-grade for enterprise.

---

### T053 — Multi-source webhook ingestion (Alertmanager, Datadog, generic DSL)

**Spec**: FR-001, FR-005, US8  
**Depends on**: T026

**Done when**:
- [ ] Prometheus Alertmanager payload parser registered for source `alertmanager`.
- [ ] Datadog webhook parser registered for source `datadog`.
- [ ] Generic DSL: `field_mapping` on `WebhookSource` supports a simple JSONPath-based mapping config; fields: `title`, `severity_hint`, `source`, `labels`, `fingerprint`.
- [ ] Each source has its own HMAC endpoint path from `webhook_sources.endpoint_path`.
- [ ] Integration tests for each new source type.

---

### T054 — PagerDuty bi-directional sync

**Spec**: US8 (AC3)  
**Depends on**: T031

**Done when**:
- [ ] On incident creation in Autopsy, a PagerDuty incident is opened via Events API v2 when `pd_integration_key` is configured on the team.
- [ ] `POST /api/v1/webhooks/pagerduty` ingests PD alerts and links to Autopsy incidents bi-directionally.
- [ ] Incident resolution in either system updates the other within 30 seconds.

---

### T055 — OIDC SSO

**Spec**: FR-037, US7  
**Depends on**: T049

**Done when**:
- [ ] `GET /api/v1/auth/oidc/login` redirects to OIDC provider.
- [ ] `GET /api/v1/auth/oidc/callback` exchanges code, maps `sub` claim to local user (auto-provisions on first login), issues JWT pair.
- [ ] `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URI` env vars.
- [ ] Integration test: mock OIDC provider (testcontainers); full PKCE flow succeeds; user auto-provisioned.

---

### T056 — Custom RBAC roles

**Spec**: FR-036, US7  
**Depends on**: T048

**Done when**:
- [ ] `POST /api/v1/roles` — Admin; creates a custom role with arbitrary permission set.
- [ ] Custom roles assigned to users via `PATCH /api/v1/users/:id`.
- [ ] RBAC middleware resolves custom role permissions from DB (cached in `patrickmn/go-cache`, 60-second TTL).
- [ ] Integration test (US7 AC4): custom role with `incident:read` + `incident:update` → GET/PATCH succeed; POST → 403.

---

### T057 — Advanced SLO analytics

**Spec**: FR-026–FR-029, US5  
**Depends on**: T042, T043

**Done when**:
- [ ] Error budget burn-rate history chart in UI (T044 extended).
- [ ] `GET /api/v1/slos/:id/burn-rate?window=1h` returns fast/slow burn rates.
- [ ] SLO status history: track every transition `ok → at_risk → exhausted` with timestamp.
- [ ] Prometheus `/metrics` endpoint includes `autopsy_slo_error_budget_remaining` gauge per SLO.

---

### T058 — Production Helm chart (full values)

**Spec**: FR-000d, US0 (full product polish)  
**Depends on**: T020

**Done when**:
- [ ] `values.yaml` complete with: HPA config, PodDisruptionBudget, NetworkPolicy, external secret references (AWS Secrets Manager, Vault annotations), multiple replica support with pod anti-affinity.
- [ ] `helm lint` and `helm template` produce no warnings.
- [ ] Chart published to a GitHub Pages OCI registry (goreleaser helm plugin or manual chart artifact).
- [ ] `docs/deployment.md` covers a complete production Kubernetes deployment walkthrough.

---

## Appendix: Task → Spec traceability

| Task | User Story | Functional Requirements |
|---|---|---|
| T001–T024 | US0 | FR-000, FR-000a–j, FR-015a–d, FR-034 |
| T024a–T024b | US0 | FR-000 (database abstraction, ADR 0012) |
| T025–T027 | US1 | FR-001–FR-005 |
| T028–T029 | US1 | FR-006–FR-011 |
| T030–T033 | US2 | FR-012–FR-016 |
| T034 | US1, US2 | — (UI) |
| T035–T038 | US3 | FR-017–FR-020 |
| T039–T041 | US4 | FR-021–FR-025 |
| T042–T044 | US5 | FR-026–FR-029 |
| T045–T047 | US6 | FR-030–FR-033 |
| T048–T052 | US7 | FR-036–FR-039 |
| T053–T054 | US8 | FR-001, FR-005 |
| T055 | US7 | FR-037 |
| T056 | US7 | FR-036 |
| T057 | US5 | FR-026–FR-029 |
| T058 | US0 | FR-000d |
