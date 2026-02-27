# Changelog

All notable changes to Autopsy are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Phase 0 foundation:
  - Go module scaffold with `cmd/autopsy` entrypoint
  - PostgreSQL connection pool with `pgx/v5`
  - Database migrations: organisations, users, refresh tokens
  - Stateless JWT auth with refresh token rotation
  - River job queue bootstrap
  - OpenTelemetry SDK (OTLP traces + Prometheus metrics)
  - `GET /api/v1/health` and `GET /api/v1/ready` endpoints
  - Seed admin user on first boot
  - React 19 + PatternFly 6 frontend scaffold (8 route shells)
  - Multi-stage Dockerfile (Node builder → Go builder → distroless)
  - Docker Compose for local Postgres
  - GitHub Actions: lint, test, security, semantic-pr, build, release
  - GoReleaser with milestone-triggered releases
  - Helm chart skeleton
  - Architecture Decision Records (ADR 0001–0011)
