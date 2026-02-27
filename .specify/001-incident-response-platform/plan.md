# Implementation Plan: Incident Response Management Platform

**Branch**: `001-incident-response-platform` | **Date**: 2026-02-25 | **Spec**: [spec.md](./spec.md)

## Summary

Autopsy is a single-binary, self-hostable incident response platform. The Go backend compiles the React/PatternFly frontend directly into the binary using `go:embed`, removing any runtime dependency on a separate static file server. The API is JSON:API 1.1 conformant. CI is a composed set of independent GitHub Actions workflows (lint, test, security, semantic-PR, build, release) that run in parallel, with caching at every layer. Releases are triggered by closing a GitHub Milestone; goreleaser creates multi-arch binaries, a Docker image published exclusively to GHCR, and a GitHub Release with a conventional-commit-derived changelog. Every linter is on, every nolint comment requires a human-approved justification, and no deprecated linters are used.

---

## Technical Context

| Attribute | Decision |
|---|---|
| **Language/Version** | Go 1.25 (from `go.mod`); Node 22 LTS (frontend tooling) |
| **Frontend Framework** | React 19 + TypeScript + [PatternFly 6](https://www.patternfly.org/) |
| **Frontend Build** | Vite 6; output embedded into Go binary via `go:embed` |
| **API Standard** | JSON:API 1.1 |
| **HTTP Router** | Go 1.22+ `net/http` stdlib (`ServeMux` with method+path patterns) |
| **Database** | PostgreSQL 17 for production; SQLite for local development — driver selected via `DB_DRIVER` env var (`postgres` \| `sqlite`, default `sqlite`) |
| **ORM** | GORM v2 (`gorm.io/gorm`) — dialect-neutral query building and AutoMigrate; `pgx/v5` stdlib adapter retained for PostgreSQL only (needed by River) |
| **Migrations** | Dialect-split: `golang-migrate` SQL files in `internal/db/migrations/postgres/` for production; GORM `AutoMigrate` for SQLite dev |
| **Job Queue** | `riverqueue/river` (Postgres-backed); a thin `worker.Queue` interface decouples app code from the driver — `noopQueue` used when `DB_DRIVER=sqlite` |
| **Short-lived Cache** | Go `sync.Map` / in-process TTL cache (`patrickmn/go-cache`) — no external cache process |
| **Observability** | `log/slog` (JSON in prod, text in dev) + OpenTelemetry Go SDK (traces + metrics) |
| **Testing** | `go test -race ./...` + `testify/assert` + `testcontainers-go` for integration |
| **Container** | Multi-stage Dockerfile; multi-arch (`linux/amd64`, `linux/arm64`) |
| **Kubernetes** | Helm chart (Kubernetes 1.28+), `charts/autopsy/` |
| **Local Dev** | `make dev` uses SQLite by default (zero infrastructure — no Docker, no Postgres); `docker compose up` available for production-like local testing with Postgres |
| **Linter** | `golangci-lint` v2 (strict, no deprecated linters, no unapproved nolint) |
| **Release** | Milestone close → git tag → `goreleaser` → GHCR + GitHub Release |
| **Versioning** | Semver git tag created directly from milestone title (validated by release workflow) |
| **AI Provider** | Provider-agnostic `AIProvider` interface; concrete implementation injected via config; OpenAI-compatible at PoC |
| **Email** | Deferred to MVP; no SMTP/SES infrastructure in Foundation or PoC |
| **Auth** | Stateless JWT (15-min access token + 30-day refresh token); PostgreSQL refresh-token blocklist; seed admin on first boot |
| **Performance Goals** | Webhook P99 ≤ 200 ms; AI triage ≤ 30 s; status page TTFB ≤ 500 ms |
| **Project Type** | Web service (single binary, embedded UI) |

---

## Database Abstraction Strategy

The `DB_DRIVER` environment variable (`postgres` | `sqlite`, default `sqlite`) selects the active database engine at startup. This makes `make dev` require **zero infrastructure**: no Docker, no Postgres — the binary creates an SQLite file and is ready to serve in under a second.

### Driver matrix

| `DB_DRIVER` | DSN example | Target environment |
|---|---|---|
| `sqlite` *(default)* | `./data/autopsy-dev.db` | Local development |
| `postgres` | `postgres://autopsy:pass@localhost:5432/autopsy?sslmode=disable` | CI, staging, production |

### ORM — GORM v2

GORM v2 (`gorm.io/gorm`, MIT, 37 k+ stars) is used as the ORM and query builder. It provides dialect-neutral CRUD via two drivers:

- **`gorm.io/driver/postgres`** — wraps `jackc/pgx/v5/stdlib` for production.
- **`gorm.io/driver/sqlite`** (backed by `glebarez/sqlite`, pure Go, no CGO) — wraps `modernc.org/sqlite` for dev.

GORM was chosen over raw `database/sql` + sqlx because it eliminates dialect-specific SQL syntax differences (UUID vs TEXT primary keys, boolean literals, placeholder style) that would otherwise require dual query sets. Constitution §IV is satisfied: 37 k+ stars, MIT license, actively maintained.

> **Alternative (if GORM proves too magic):** `jmoiron/sqlx` + manually maintained per-dialect SQL dirs (`queries/postgres/`, `queries/sqlite/`). Document the swap as an ADR amendment.

### Migration strategy

| Driver | Mechanism | Location |
|---|---|---|
| `postgres` | `golang-migrate` SQL files — explicit, reviewable, zero-downtime-compatible | `internal/db/migrations/postgres/` |
| `sqlite` | GORM `AutoMigrate` on Go model structs — no SQL files to maintain for dev | Go structs in `internal/model/` |

PostgreSQL keeps explicit SQL migrations because production requires careful, reviewed schema changes. SQLite uses `AutoMigrate` because schema drift in a dev-only file database has no consequences.

### Worker queue interface

River is Postgres-only. The app layer defines a `worker.Queue` interface to decouple all enqueue calls from the concrete implementation:

```go
// worker.Queue is the minimal interface for background job submission.
type Queue interface {
    Enqueue(ctx context.Context, job JobArgs) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

| `DB_DRIVER` | Implementation | Behaviour |
|---|---|---|
| `postgres` | `riverQueue` — wraps `riverqueue/river` + `riverpgxv5` | Full async processing |
| `sqlite` | `noopQueue` — logs at DEBUG, returns nil | Jobs silently dropped (acceptable in dev) |

The `riverqueue/river` package and its `pgxpool` dependency are only imported when the postgres driver is active (build-tag or runtime guard).

---

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Code Quality | ✅ | `golangci-lint` v2 strict; `go vet`; `staticcheck`; `nolintlint` enforcement |
| II. Testing Standards | ✅ | `go test -race`, coverage gate ≥ 80%, TDD cycle enforced in workflow |
| III. Observability First | ✅ | `log/slog` + OTel from day one; `/health` and `/ready` endpoints in skeleton |
| IV. Borrow Before Build | ✅ | `golang-migrate`, `river`, `testcontainers-go`, PatternFly, GORM v2 — all established OSS; `net/http` stdlib router (no framework dependency) |
| V. AI-Augmented | ✅ | Spec and plan AI-generated; prompts preserved in `/docs/ai-prompts/` |
| VI. UX Consistency | ✅ | JSON:API 1.1; OpenAPI 3.x spec committed alongside code |
| VII. Performance | ✅ | Baseline benchmarks required before optimisation; P99 targets in SC-001 |

---

## Project Structure

### Documentation (this feature)

```text
.specify/001-incident-response-platform/
├── plan.md          ← this file
├── spec.md          ← feature specification
└── tasks.md         ← created by /speckit.tasks
```

### Source Code (repository root)

```text
autopsy/
├── .github/
│   ├── prompts/                       # spec-kit prompts (already present)
│   └── workflows/
│       ├── lint.yml                   # Go + frontend lint (golangci-lint, eslint, prettier)
│       ├── test.yml                   # go test -race, coverage, frontend unit tests
│       ├── security.yml               # govulncheck (SARIF → Security tab)
│       ├── semantic-pr.yml            # PR title + all-commit conventional commit lint
│       ├── build.yml                  # compile + docker build (no push, on every PR)
│       └── release.yml                # milestone closed → tag → goreleaser → GHCR + Release
│
├── cmd/
│   └── autopsy/
│       └── main.go                    # entrypoint; wires server, config, OTel shutdown
│
├── internal/
│   ├── api/
│   │   ├── handler/                   # HTTP handlers, one file per resource group
│   │   ├── middleware/                # auth, logging, tracing, rate-limit, CORS
│   │   ├── jsonapi/                   # JSON:API 1.1 serialiser/deserialiser helpers
│   │   └── router.go                  # net/http ServeMux route registration
│   ├── health/
│   │   └── handler.go                 # /api/v1/health and /api/v1/ready
│   ├── config/
│   │   └── config.go                  # env-based config (no viper; stdlib + envconfig)
│   ├── db/
│   │   ├── migrations/                # SQL migration files (golang-migrate)
│   │   └── db.go                      # pgx pool setup
│   ├── worker/
│   │   └── worker.go                  # River queue workers
│   └── observability/
│       └── otel.go                    # OTel SDK bootstrap (tracer, meter, logger)
│
├── web/                               # frontend source (NOT embedded until built)
│   ├── src/
│   │   ├── components/                # PatternFly-based shared components
│   │   ├── pages/                     # route-level page components
│   │   ├── api/                       # typed API client (generated from OpenAPI spec)
│   │   └── main.tsx                   # React entrypoint
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── .eslintrc.cjs
│   ├── .prettierrc.json
│   └── package.json                   # @patternfly/react-core, react, vite, typescript
│
├── ui/                                # ← generated by `make build-ui`; committed to .gitignore
│   └── dist/                          # Vite output; embedded via go:embed in cmd/autopsy
│
├── charts/
│   └── autopsy/                       # Helm chart (Kubernetes 1.28+)
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── templates/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── ingress.yaml
│       │   ├── configmap.yaml
│       │   ├── secret.yaml
│       │   └── _helpers.tpl
│       └── README.md
│
├── docs/
│   ├── adr/
│   │   ├── 0001-record-architecture-decisions.md
│   │   ├── 0002-single-binary-embedded-ui.md
│   │   ├── 0003-jsonapi-1.1-standard.md
│   │   ├── 0004-river-queue-over-redis.md
│   │   └── 0005-patternfly-ui-framework.md
│   │   └── 0012-database-driver-abstraction.md
│   ├── runbooks/
│   │   └── local-development.md
│   ├── ai-prompts/                    # Preserved prompts per constitution V
│   └── development.md                 # Full dev guide (setup, test, lint, release)
│
├── .golangci.yml                      # strict lint config (v2 format)
├── .goreleaser.yml                    # release config
├── .commitlintrc.mjs                  # commitlint config
├── Dockerfile                         # multi-stage build
├── compose.yaml                       # local dev stack
├── Makefile                           # build, test, lint, dev, docker-build, release targets
├── CHANGELOG.md
├── CONTRIBUTING.md
├── README.md
├── go.mod
└── go.sum
```

---

## Foundation UI Layout (First Pass)

The Foundation stage ships a minimal but **visually correct** PatternFly shell. The goal is to confirm that the embedded frontend is served correctly, PatternFly renders as expected, and the developer experience (hot-reload in dev, serve-from-binary in prod) works end-to-end. No real data is wired up — all content is placeholder.

### Shell Structure

```text
┌─────────────────────────────────────────────────────────────────┐
│  Masthead (Page header)                                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 🔴 Autopsy          [Nav links]           [User menu ▾] │   │
│  └─────────────────────────────────────────────────────────┘   │
│  ┌────────────┬────────────────────────────────────────────┐   │
│  │            │                                            │   │
│  │  Nav       │  Page content area                        │   │
│  │  Sidebar   │                                            │   │
│  │            │  (route-rendered page component)           │   │
│  │  Dashboard │                                            │   │
│  │  Alerts    │                                            │   │
│  │  Incidents │                                            │   │
│  │  On-Call   │                                            │   │
│  │  SLOs      │                                            │   │
│  │  Status    │                                            │   │
│  │  Settings  │                                            │   │
│  │            │                                            │   │
│  └────────────┴────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### PatternFly Components Used in Shell

| Area | PatternFly Component |
|---|---|
| Top bar | `Page`, `Masthead`, `MastheadMain`, `MastheadBrand`, `MastheadContent` |
| Navigation | `Nav`, `NavList`, `NavItem`, `PageSidebar` |
| User menu | `Dropdown` in `MastheadContent` |
| Page wrapper | `Page` + `PageSection` |
| Placeholder content | `EmptyState`, `EmptyStateIcon`, `EmptyStateBody` |
| Loading state | `Spinner` + `EmptyState` |
| Notifications | `AlertGroup`, `Alert` (wired to an in-app notification queue) |

### Routes in the First Pass

All routes render an `EmptyState` with the page name, a descriptive icon, and a "Coming soon" body. This confirms routing, navigation highlighting, and the layout all work before any real data is wired.

| Path | Page Component | Nav Label |
|---|---|---|
| `/` | `<DashboardPage />` | Dashboard |
| `/alerts` | `<AlertsPage />` | Alerts |
| `/incidents` | `<IncidentsPage />` | Incidents |
| `/on-call` | `<OnCallPage />` | On-Call |
| `/slos` | `<SLOsPage />` | SLOs |
| `/status` | `<StatusPage />` | Status Page |
| `/settings` | `<SettingsPage />` | Settings |
| `*` | `<NotFoundPage />` | — (404) |

### Frontend Dev Experience

`make dev` now starts with **zero external infrastructure** by defaulting to SQLite. No Docker, no Postgres.

#### SQLite mode (default) — zero infrastructure

```
# .env (auto-copied from .env.example if absent)
DB_DRIVER=sqlite
DB_DSN=./data/autopsy-dev.db
```

```
make dev          # copies .env.example → .env, starts air (hot-reload), starts Vite dev server
```

- `air` builds and restarts the Go binary on file changes; the binary creates/migrates the SQLite file on every start.
- Vite dev server starts on `:5173` and proxies `/api` → `:8080`.
- No Docker socket, no running containers required.

#### Postgres mode — production-equivalent local dev

```
# Override .env before running
DB_DRIVER=postgres
DB_DSN=postgres://autopsy:autopsy@localhost:5432/autopsy?sslmode=disable
```

```
docker compose up -d postgres   # start Postgres only
make dev-api                    # start Go backend
make dev-ui                     # start Vite dev server
```

`make dev` detects whether Postgres is already reachable (`pg_isready`) and falls back to the native Postgres cluster (if installed) before requiring Docker.

#### Vite proxy config (unchanged)

```ts
// web/vite.config.ts
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: '../ui/dist',
    emptyOutDir: true,
  },
});
```

### `GET /api/v1/health` Response (Foundation Target)

```json
{
  "data": {
    "type": "health",
    "id": "1",
    "attributes": {
      "status": "ok",
      "version": "0.0.1-dev",
      "uptime_seconds": 42,
      "commit": "abc1234",
      "build_date": "2026-02-25T00:00:00Z"
    }
  }
}
```

### `GET /api/v1/ready` Response

Returns `200 OK` when the active database (SQLite file accessible, or Postgres connection pool pingable) reports healthy. Returns `503 Service Unavailable` with a JSON error body otherwise — used by Kubernetes readiness probes and `compose.yaml` `healthcheck`.

---

## CI/CD Design

### Workflow Overview

```text
On every PR:
  ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
  │  semantic-pr.yml│   │    lint.yml      │   │   security.yml  │
  │  (PR title +    │   │  (golangci-lint, │   │  (govulncheck   │
  │   all commits)  │   │   eslint,        │   │   SARIF upload) │
  └─────────────────┘   │   prettier)      │   └─────────────────┘
                        └─────────────────┘
  ┌─────────────────┐   ┌─────────────────┐
  │    test.yml     │   │    build.yml     │
  │  (go test -race,│   │  (go build,      │
  │   coverage,     │   │   docker build   │
  │   frontend test)│   │   no push)       │
  └─────────────────┘   └─────────────────┘

On milestone closed:
  release.yml → create git tag → goreleaser → GHCR + GitHub Release
```

All workflows are **required status checks** on `main`. Each runs independently so a lint failure does not block the test result from being reported.

---

### Workflow: `lint.yml`

```yaml
name: Lint
on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  go-lint:
    name: Go Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.3
          args: --timeout=10m   # always full scan; no only-new-issues exemptions

  frontend-lint:
    name: Frontend Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: web/package-lock.json

      - run: npm ci --prefix web
      - run: npm run lint --prefix web        # eslint
      - run: npm run format:check --prefix web # prettier --check
```

**Why `golangci/golangci-lint-action@v9`**: Official action with built-in caching (`~/.cache/golangci-lint`), problem-matcher registration (inline GitHub annotations in the Files Changed tab). `@v9` is the latest stable release (Dec 2025). `only-new-issues` is intentionally disabled — every lint violation is reported on every run to prevent gradual quality decay.

---

### Workflow: `test.yml`

```yaml
name: Test
on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  go-test:
    name: Go Test
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_PASSWORD: autopsy
          POSTGRES_DB: autopsy_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports: ["5432:5432"]

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...
        env:
          DATABASE_URL: postgres://postgres:autopsy@localhost:5432/autopsy_test?sslmode=disable

      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
          echo "Total coverage: ${COVERAGE}%"
          awk "BEGIN {exit ($COVERAGE < 80)}" || (echo "Coverage below 80%" && exit 1)

      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out

  frontend-test:
    name: Frontend Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: web/package-lock.json

      - run: npm ci --prefix web
      - run: npm test --prefix web -- --run
```

---

### Workflow: `security.yml`

```yaml
name: Security
on:
  pull_request:
  push:
    branches: [main]
  schedule:
    - cron: '0 3 * * 1'    # weekly Monday 03:00 UTC

permissions:
  contents: read
  security-events: write    # required for SARIF upload → Security tab annotations

jobs:
  govulncheck:
    name: govulncheck
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: golang/govulncheck-action@v1
        with:
          go-version-file: go.mod
          go-package: ./...
          output-format: sarif
          output-file: govulncheck.sarif

      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: govulncheck.sarif
          category: govulncheck
```

**Why SARIF**: The `text` output only fails the job log. SARIF + `upload-sarif` creates Security tab findings and inline PR code annotations (requires `security-events: write`), giving reviewers direct visibility into vulnerable import locations.

---

### Workflow: `semantic-pr.yml`

```yaml
name: Semantic PR
on:
  pull_request_target:
    types: [opened, reopened, edited, synchronize]

permissions:
  pull-requests: read

jobs:
  pr-title:
    name: PR Title (Conventional Commits)
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v6
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          types: |
            fix
            feat
            chore
            docs
            refactor
            test
            ci
            build
            perf
            revert
          requireScope: false
          subjectPattern: ^(?![A-Z]).+$
          subjectPatternError: |
            The subject "{subject}" must not start with an uppercase letter.
            Use lowercase: e.g. "feat: add webhook ingestion endpoint"

  commits:
    name: All Commits (Conventional Commits)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: wagoid/commitlint-github-action@v6
        with:
          configFile: .commitlintrc.mjs
          failOnWarnings: false
          failOnErrors: true
```

**Why `pull_request_target` for pr-title**: Runs with write access to the base branch, ensuring it works for fork PRs without exposing secrets.

**Why two separate jobs**: PR title and commit message lint can fail independently, giving cleaner failure signals.

**.commitlintrc.mjs** (to be committed to repo root):
```js
export default { extends: ['@commitlint/config-conventional'] };
```

---

### Workflow: `build.yml`

```yaml
name: Build
on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  go-build:
    name: Go Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - uses: actions/setup-node@v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: web/package-lock.json

      - name: Build frontend
        run: npm ci --prefix web && npm run build --prefix web

      - name: Build Go binary
        run: go build -v ./cmd/autopsy

  docker-build:
    name: Docker Build (no push)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-buildx-action@v3

      - uses: docker/build-push-action@v6
        with:
          context: .
          push: false
          platforms: linux/amd64,linux/arm64
          cache-from: type=gha
          cache-to: type=gha,mode=max
          tags: ghcr.io/${{ github.repository }}:pr-${{ github.event.number }}
```

---

### Workflow: `release.yml`

Triggered when a GitHub Milestone is closed. The Milestone title **must** be a valid semver string (e.g., `v1.2.0`). This is enforced by a job that validates the title before tagging.

```yaml
name: Release
on:
  milestone:
    types: [closed]

permissions:
  contents: write
  packages: write

jobs:
  validate-milestone:
    name: Validate Milestone Title
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.check.outputs.version }}
    steps:
      - name: Check semver format
        id: check
        run: |
          TITLE="${{ github.event.milestone.title }}"
          if [[ ! "$TITLE" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
            echo "Milestone title '${TITLE}' is not a valid semver (expected: vX.Y.Z)"
            exit 1
          fi
          echo "version=${TITLE}" >> "$GITHUB_OUTPUT"

  release:
    name: Tag & Release
    needs: validate-milestone
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0              # goreleaser requires full history for changelog

      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - uses: actions/setup-node@v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: web/package-lock.json

      - name: Build frontend
        run: npm ci --prefix web && npm run build --prefix web

      - name: Create and push tag
        run: |
          git config user.name  "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag "${{ needs.validate-milestone.outputs.version }}"
          git push origin "${{ needs.validate-milestone.outputs.version }}"
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/setup-qemu-action@v3       # for arm64 emulation

      - uses: docker/setup-buildx-action@v3

      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Note on the `GITHUB_TOKEN` tag push limitation**: The default `GITHUB_TOKEN` can push tags but those pushes do **not** trigger other workflows (GitHub security restriction). Goreleaser is invoked directly in the same job, so this is not an issue here.

---

### `.goreleaser.yml`

```yaml
version: 2

project_name: autopsy

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - id: autopsy
    main: ./cmd/autopsy
    binary: autopsy
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w
      - -X github.com/d9705996/autopsy/internal/version.Version={{.Version}}
      - -X github.com/d9705996/autopsy/internal/version.Commit={{.Commit}}
      - -X github.com/d9705996/autopsy/internal/version.Date={{.Date}}

archives:
  - id: autopsy
    name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'
    format_overrides:
      - goos: windows
        formats: [zip]

dockers:
  - image_templates:
      - 'ghcr.io/d9705996/autopsy:{{ .Tag }}-amd64'
    use: buildx
    build_flag_templates:
      - '--platform=linux/amd64'
      - '--label=org.opencontainers.image.source=https://github.com/d9705996/autopsy'
      - '--label=org.opencontainers.image.version={{ .Version }}'
      - '--label=org.opencontainers.image.revision={{ .Commit }}'
  - image_templates:
      - 'ghcr.io/d9705996/autopsy:{{ .Tag }}-arm64'
    use: buildx
    goarch: arm64
    build_flag_templates:
      - '--platform=linux/arm64'
      - '--label=org.opencontainers.image.source=https://github.com/d9705996/autopsy'
      - '--label=org.opencontainers.image.version={{ .Version }}'
      - '--label=org.opencontainers.image.revision={{ .Commit }}'

docker_manifests:
  - name_template: 'ghcr.io/d9705996/autopsy:{{ .Tag }}'
    image_templates:
      - 'ghcr.io/d9705996/autopsy:{{ .Tag }}-amd64'
      - 'ghcr.io/d9705996/autopsy:{{ .Tag }}-arm64'
  - name_template: 'ghcr.io/d9705996/autopsy:latest'
    image_templates:
      - 'ghcr.io/d9705996/autopsy:{{ .Tag }}-amd64'
      - 'ghcr.io/d9705996/autopsy:{{ .Tag }}-arm64'

changelog:
  use: github
  sort: asc
  groups:
    - title: Features
      regexp: '^feat'
      order: 0
    - title: Bug Fixes
      regexp: '^fix'
      order: 1
    - title: Performance
      regexp: '^perf'
      order: 2
    - title: Other Changes
      order: 999
  filters:
    exclude:
      - '^chore\(deps\)'
      - '^ci'
      - Merge pull request
      - Merge branch

release:
  github:
    owner: d9705996
    name: autopsy
  draft: false
  prerelease: auto
```

---

## Go Linting Config (`.golangci.yml`)

This uses the **golangci-lint v2 config format** (`version: "2"` at the top). All enabled linters are current (non-deprecated). No linter may be disabled without a human-approved comment in the `linters.disable` section.

```yaml
version: "2"

run:
  timeout: 10m
  tests: true
  modules-download-mode: readonly

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
  # Severity threshold: treat all issues as errors
  uniq-by-line: false

  # nolintlint must always include a reason and valid linter name.
  # Any //nolint without an explanation or with a non-existent linter name
  # is itself a lint error. Disabling a linter requires human approval via
  # the linters.disable list below — never via inline comments.
  exclude-rules: []

linters:
  default: none
  enable:
    # --- Correctness ---
    - errcheck           # unchecked error returns
    - govet              # go vet checks
    - staticcheck        # SA, S, ST, QF checks
    - ineffassign        # assigned but never used
    - unused             # unused code
    - bodyclose          # HTTP response body not closed
    - contextcheck       # context not propagated
    - durationcheck      # time.Duration multiplication bugs
    - errname            # error type naming conventions
    - errorlint          # wrapping and comparison errors (errors.As, errors.Is)
    - exhaustive         # exhaustive enum switches
    - nilerr             # returning nil error when err != nil
    - nilnil             # returning (nil, nil) from func returning (*T, error)
    - noctx              # HTTP request without context
    - rowserrcheck       # sql.Rows.Err() check
    - sqlclosecheck      # sql.Rows/Stmt.Close() check
    - wastedassign       # wasted assignment
    - wrapcheck          # errors must be wrapped when crossing package boundaries

    # --- Style & Idiom ---
    - gofmt              # gofmt formatting
    - goimports          # import ordering and missing imports
    - gci                # import grouping (stdlib, external, internal)
    - godot              # comment sentences end with a period
    - misspell           # common spelling mistakes
    - predeclared        # shadowing of predeclared identifiers
    - unconvert          # unnecessary type conversions
    - unparam            # unused function parameters
    - usestdlibvars      # use stdlib constants (http.StatusOK etc.)
    - whitespace         # unnecessary blank lines

    # --- Complexity ---
    - cyclop             # cyclomatic complexity (max 15)
    - funlen             # function length (max 80 lines)
    - gocognit           # cognitive complexity (max 20)
    - gocyclo            # cyclomatic complexity guard (max 15)
    - maintidx           # maintainability index
    - nestif             # deeply nested if blocks
    - nakedret           # naked returns in long functions

    # --- Security ---
    - gosec              # security issues (G-series checks)

    # --- Quality ---
    - goconst            # repeated strings that could be constants
    - gocritic           # assorted micro-optimisations and style checks
    - dupl               # code duplication
    - godox              # TODO/FIXME/HACK without issue reference
    - gomoddirectives    # disallowed go.mod directives
    - lll                # line length (max 120)
    - musttag            # struct tags on marshalled types
    - reassign           # reassignment of top-level package vars
    - tagliatelle        # struct tag naming consistency
    - nonamedreturns     # no named return values (except single bool)

    # --- Testing ---
    - paralleltest       # test cases must call t.Parallel()
    - testifylint        # testify usage correctness
    - thelper            # test helpers must call t.Helper()
    - tparallel          # subtests must call t.Parallel()

    # --- Logging ---
    - sloglint           # slog usage correctness
    - loggercheck        # key-value pair correctness in log calls

    # --- nolint enforcement (NON-NEGOTIABLE) ---
    - nolintlint         # every //nolint must specify linter(s) and a reason

    # --- Misc ---
    - gochecknoglobals   # no package-level variables (exceptions via config below)
    - ireturn            # accept interfaces, return concrete types
    - prealloc           # slice pre-allocation opportunities
    - nlreturn           # blank lines before return statements
    - wsl                # whitespace linter (Go idiom enforcement)
    - forbidigo          # forbid specific function calls (fmt.Print* in prod code)

linters-settings:
  cyclop:
    max-complexity: 15

  gocyclo:
    min-complexity: 15

  gocognit:
    min-complexity: 20

  funlen:
    lines: 80
    statements: 50

  lll:
    line-length: 120

  gci:
    sections:
      - standard
      - default
      - prefix(github.com/d9705996/autopsy)
    skip-generated: true

  godox:
    keywords: [TODO, FIXME, HACK, BUG, OPTIMIZE]

  gosec:
    excludes: []    # no exclusions; all G-series checks active

  nolintlint:
    allow-unused: false          # fail if the suppressed linter wouldn't fire
    allow-no-explanation: []     # every nolint MUST have a reason comment
    require-explanation: true
    require-specific: true       # must name the linter(s) e.g. //nolint:gosec // reason

  tagliatelle:
    case:
      rules:
        json: snake
        yaml: snake
        db: snake

  gochecknoglobals:
    # Only these package-level vars are permitted:
    check-tests: false          # test files exempt

  ireturn:
    allow:
      - error
      - empty
      - anon
      - stdlib

  forbidigo:
    forbid:
      - pattern: ^fmt\.Print
        msg: "Use log/slog for all output in production code; fmt.Print* is only allowed in cmd/ main functions and test helpers"
    exclude-godoc-examples: true

  wsl:
    allow-assign-and-anything: false
    allow-multiline-assign: true

  gocritic:
    enabled-tags: [diagnostic, style, performance]
    disabled-checks: []   # no checks disabled without human approval
```

---

## Frontend: PatternFly + Vite + TypeScript

### Rationale for PatternFly

PatternFly is Red Hat's open-source design system, purpose-built for enterprise application UIs. It provides production-quality React components that match the visual language of infrastructure/operations tooling (Grafana, OpenShift, RH SSO). It avoids the need to build data table, alert severity badge, timeline, status indicator, and navigation components from scratch — all of which are core to Autopsy's UI.

### `package.json` (key dependencies)

```json
{
  "dependencies": {
    "@patternfly/react-core": "^6",
    "@patternfly/react-icons": "^6",
    "@patternfly/react-table": "^6",
    "@patternfly/react-charts": "^8",
    "react": "^19",
    "react-dom": "^19",
    "react-router-dom": "^7"
  },
  "devDependencies": {
    "@types/react": "^19",
    "@types/react-dom": "^19",
    "@typescript-eslint/eslint-plugin": "^8",
    "@typescript-eslint/parser": "^8",
    "@vitejs/plugin-react": "^4",
    "eslint": "^9",
    "eslint-config-prettier": "^10",
    "prettier": "^3",
    "typescript": "^5",
    "vite": "^6",
    "vitest": "^3"
  },
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build --outDir ../ui/dist --emptyOutDir",
    "lint": "eslint src",
    "format:check": "prettier --check src",
    "format": "prettier --write src",
    "test": "vitest"
  }
}
```

**Note**: `vite build --outDir ../ui/dist` writes the production bundle to `ui/dist/`, which is the path embedded by `go:embed` in `cmd/autopsy/main.go`. The `ui/dist/` directory is `.gitignore`d; the CI `build.yml` workflow builds it before `go build`.

### Frontend ESLint Config (`.eslintrc.cjs`)

```js
module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  plugins: ['@typescript-eslint'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/strict-type-checked',
    'plugin:@typescript-eslint/stylistic-type-checked',
    'prettier',
  ],
  parserOptions: { project: ['./tsconfig.json'] },
  rules: {
    '@typescript-eslint/no-unused-vars': 'error',
    '@typescript-eslint/explicit-function-return-type': 'error',
    'no-console': 'error',          // use structured API calls; no console.log
  },
};
```

---

## Dockerfile (Multi-Stage)

```dockerfile
# ── Stage 1: Frontend build ──────────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

# ── Stage 2: Go build ────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/ui/dist ./ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /autopsy ./cmd/autopsy

# ── Stage 3: Runtime ─────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-builder /autopsy /autopsy
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/autopsy"]
```

**Why distroless**: No shell, no OS package manager, minimal CVE surface. `nonroot` user satisfies security scanners and Kubernetes `securityContext.runAsNonRoot: true`.

---

## `compose.yaml` (Local Dev Stack)

```yaml
services:
  autopsy:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://autopsy:autopsy@postgres:5432/autopsy?sslmode=disable
      LOG_FORMAT: text
      LOG_LEVEL: debug
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/api/v1/health"]
      interval: 10s
      timeout: 5s
      retries: 5

  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: autopsy
      POSTGRES_PASSWORD: autopsy
      POSTGRES_DB: autopsy
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "autopsy"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  postgres-data:
```

---

## Makefile

```makefile
.PHONY: build build-ui test lint lint-go lint-ui dev docker-build release clean

GO_CMD    := go
NPM_CMD   := npm
BINARY    := autopsy
BUILD_DIR := ./cmd/autopsy

build: build-ui
	$(GO_CMD) build -v -o $(BINARY) $(BUILD_DIR)

build-ui:
	$(NPM_CMD) ci --prefix web
	$(NPM_CMD) run build --prefix web

test:
	$(GO_CMD) test -race -coverprofile=coverage.out ./...

lint: lint-go lint-ui

lint-go:
	golangci-lint run ./...

lint-ui:
	$(NPM_CMD) run lint --prefix web
	$(NPM_CMD) run format:check --prefix web

dev:
	docker compose up --build

docker-build:
	docker build -t ghcr.io/d9705996/autopsy:dev .

release:
	goreleaser release --clean

clean:
	rm -f $(BINARY) coverage.out
	rm -rf ui/dist
```

---

## Architecture Decision Records (to be created as part of Foundation stage)

| ADR | Decision | Rationale |
|---|---|---|
| 0001 | Adopt MADR format for ADRs | Standard RFC-like format; tooling available |
| 0002 | Single binary with `go:embed` for frontend | Eliminates deployment complexity; single artefact to distribute and version |
| 0003 | JSON:API 1.1 as API standard | Client-agnostic, well-specified, strong OSS tooling for Go and React |
| 0004 | `river` for job queue; no Redis/Valkey at any stage | Postgres-backed queue eliminates an entire infrastructure dependency; `patrickmn/go-cache` covers all short-lived caching needs in-process |
| 0005 | PatternFly 6 as UI framework | Enterprise-grade component library; purpose-built for operational tooling; avoids building data tables, alert severity components, navigation from scratch |
| 0006 | `chi` as HTTP router | Stdlib-compatible; `http.Handler`-based; no magic; minimal footprint |
| 0007 | Milestone-driven releases | Milestones map directly to planned versions; closing a milestone is an explicit human intent signal for a release |
| 0012 | Database driver abstraction via `DB_DRIVER` | `sqlite` default eliminates infrastructure for local dev; `postgres` for CI/staging/production; GORM v2 provides dialect-neutral ORM; `worker.Queue` interface decouples River (postgres-only) from the rest of the app |

---

## Key Go Dependencies (with justification)

| Package | Version | Justification |
|---|---|---|
| `gorm.io/gorm` | v2 | ORM and query builder — dialect-neutral CRUD + AutoMigrate; dialect switching via driver injection |
| `gorm.io/driver/postgres` | v2 | GORM Postgres dialect; wraps `pgx/v5/stdlib` |
| `glebarez/sqlite` | latest | GORM SQLite dialect via `gorm.io/driver/sqlite`; pure Go (no CGO), uses `modernc.org/sqlite` internally |
| `jackc/pgx/v5` | v5 | High-performance Postgres driver; required by `riverqueue/river` (pgxpool) when `DB_DRIVER=postgres`; stdlib adapter used by GORM Postgres driver |
| `golang-migrate/migrate/v4` | v4 | Production Postgres schema migrations — SQL files, version-controlled, zero-downtime compatible |
| `riverqueue/river` | latest | Postgres-backed job queue; strongly typed jobs; active when `DB_DRIVER=postgres` only |
| `patrickmn/go-cache` | v2 | In-process TTL cache; replaces any need for an external cache at PoC/MVP scale |
| `go.opentelemetry.io/otel` | v1.x | Official OTel Go SDK; traces + metrics |
| `testcontainers/testcontainers-go` | latest | Spin up real Postgres in integration tests; SQLite unit tests need no container |
| `stretchr/testify` | v1 | `assert`/`require` for cleaner test failures |
| *(hand-written)* | — | JSON:API 1.1 envelope types — ~200 lines of `encoding/json`-based serialisers in `internal/api/jsonapi/`. No external JSON:API library dependency. |

---

## Phase Delivery Summary

| Phase | Key Deliverables | Spec Stories |
|---|---|---|
| **Foundation** | Repo scaffold, CI/CD (all 6 workflows), `.golangci.yml`, Dockerfile, `compose.yaml` (Postgres only), Helm chart skeleton, `/health` + `/ready` (Postgres-only readiness), JWT auth middleware + seed admin, `organization_id` column in all baseline migrations, embedded PatternFly shell (all nav routes rendered as `EmptyState` placeholders, Vite proxy for hot-reload), all ADRs, README/CONTRIBUTING/docs | US0 |
| **PoC** | Webhook ingestion (Grafana), provider-agnostic AI triage worker, alert list UI, incident declaration, incident timeline | US1, US2 |
| **MVP** | On-call scheduling, escalation policy, status page, SLO/SLI evaluation, postmortem, action items, RBAC, multi-tenancy (org isolation enforced), email notifications | US3–US7 |
| **Full Product** | Multi-source webhooks, OIDC SSO, custom roles, full audit log, advanced SLO analytics | US8 + FR-036–038 |
