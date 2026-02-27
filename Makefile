##
## Autopsy Makefile
## ─────────────────────────────────────────────────────────────────────────────
## Usage:
##   make dev        — run backend (SQLite by default) + Vite dev server
##   make build      — compile the Go binary (requires ui/dist to exist)
##   make test       — run all Go tests with race detector
##   make lint       — run golangci-lint and ESLint
##   make docker-build — build the multi-stage Docker image
##   make clean      — remove build artefacts
## ─────────────────────────────────────────────────────────────────────────────

BINARY        := autopsy
BUILD_DIR     := bin
CMD_PATH      := ./cmd/$(BINARY)
MODULE        := github.com/d9705996/autopsy
IMAGE         := ghcr.io/d9705996/autopsy

VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE          ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(MODULE)/internal/version.Version=$(VERSION) \
  -X $(MODULE)/internal/version.Commit=$(COMMIT) \
  -X $(MODULE)/internal/version.Date=$(DATE)

.DEFAULT_GOAL := help

# ─── Core targets ─────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ui-build ## Build the Go binary (includes embedded SPA)
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY)"

.PHONY: run
run: ## Run the server (SQLite by default; set DB_DRIVER=postgres for Postgres)
	@[ -f .env ] || cp .env.example .env
	@set -a && . ./.env && set +a && go run -ldflags "$(LDFLAGS)" $(CMD_PATH)
.PHONY: dev
dev: ## Start backend (hot-reload via air) + Vite dev server — SQLite by default, no Docker needed
	@[ -f .env ] || (cp .env.example .env && echo "Copied .env.example → .env")
	@grep -q 'DB_DRIVER=postgres' .env 2>/dev/null && { \
		if pg_isready -h localhost -p 5432 -q 2>/dev/null; then \
			echo "Postgres already up on :5432"; \
		elif command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then \
			echo "Starting Postgres via Docker Compose..."; \
			docker compose up -d --wait; \
		else \
			echo "Starting native Postgres..."; \
			sudo pg_ctlcluster 17 main start 2>/dev/null || true; \
			pg_isready -h localhost -p 5432 -q || (echo "ERROR: Postgres not reachable" && exit 1); \
		fi; \
	} || true

.PHONY: dev-api
dev-api: ## Run backend with air hot-reload (used by 'make dev')
	@command -v air >/dev/null 2>&1 || go install github.com/air-verse/air@latest
	@set -a && . ./.env && set +a && air -c .air.toml

.PHONY: dev-ui
dev-ui: ## Run Vite dev server (used by 'make dev')
	npm run dev --prefix web

.PHONY: stop
stop: ## Stop background services (Docker Compose or native Postgres if running)
	@if grep -q 'DB_DRIVER=postgres' .env 2>/dev/null; then \
		if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then \
			docker compose down; \
		else \
			sudo pg_ctlcluster 17 main stop 2>/dev/null || true; \
		fi; \

# ─── Test ─────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all Go tests with race detector
	go test -race -count=1 ./...

.PHONY: test-cover
test-cover: ## Run tests and open HTML coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: test-integration
test-integration: ## Run integration tests (starts a real Postgres container)
	INTEGRATION=1 go test -race -count=1 -timeout 2m ./...

# ─── Lint / Format ────────────────────────────────────────────────────────────

.PHONY: lint
lint: lint-go lint-ui ## Run all linters

.PHONY: lint-go
lint-go: ## Run golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run --timeout=5m

.PHONY: lint-ui
lint-ui: ## Run ESLint on the React SPA
	npm run lint --prefix web

.PHONY: fmt
fmt: ## Auto-format Go code
	gofmt -w ./...

# ─── Database ─────────────────────────────────────────────────────────────────

.PHONY: migrate-up
migrate-up: ## Apply all pending Postgres migrations (requires DB_DRIVER=postgres in .env)
	@[ -f .env ] || cp .env.example .env
	@grep -q 'DB_DRIVER=postgres' .env || (echo "migrate-up is only needed for DB_DRIVER=postgres (SQLite uses AutoMigrate on startup)" && exit 0)
	@set -a && . ./.env && set +a && go run -ldflags "$(LDFLAGS)" $(CMD_PATH) -migrate-only

.PHONY: migrate-down
migrate-down: ## Roll back the last Postgres migration (requires DB_DRIVER=postgres)
	@command -v migrate >/dev/null 2>&1 || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@[ -f .env ] && set -a && . ./.env && set +a; migrate -database "$$DB_DSN" -path internal/db/migrations down 1

# ─── Docker ───────────────────────────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build the multi-stage Docker image
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg DATE=$(DATE) \
	  -t $(IMAGE):$(VERSION) \
	  -t $(IMAGE):latest \
	  .

# ─── UI ───────────────────────────────────────────────────────────────────────

.PHONY: ui-build
ui-build: ## Build the React SPA into ui/dist/
	npm ci --prefix web
	npm run build --prefix web

.PHONY: ui-install
ui-install: ## Install frontend npm dependencies
	npm install --prefix web

# ─── Code generation ──────────────────────────────────────────────────────────

.PHONY: generate
generate: ## Run go generate across all packages
	go generate ./...

# ─── Release ──────────────────────────────────────────────────────────────────

.PHONY: release-snapshot
release-snapshot: ui-build ## Build a local goreleaser snapshot (dry-run)
	@command -v goreleaser >/dev/null 2>&1 || go install github.com/goreleaser/goreleaser/v2@latest
	goreleaser release --snapshot --clean

# ─── Utilities ────────────────────────────────────────────────────────────────

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR) coverage.out tmp/
	rm -rf ui/dist/**/* ui/dist/*.html
