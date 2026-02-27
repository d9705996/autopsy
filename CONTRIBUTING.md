# Contributing to Autopsy

Thanks for your interest in contributing!

## Getting started

1. Fork the repository and clone your fork.
2. Follow the [development guide](docs/development.md) to set up your local environment.
3. Create a branch: `git checkout -b feat/my-feature`

## Development workflow

```bash
cp .env.example .env
make dev       # starts Postgres, Go backend, Vite dev server
```

## Commit conventions

Autopsy uses [Conventional Commits](https://www.conventionalcommits.org/).  
PR titles are validated automatically.

```
feat: add SCIM provisioning
fix: prevent duplicate alert deduplication
docs: update README prerequisites
```

## Pull request process

1. Make sure `make test` and `make lint` pass locally.
2. Open a PR against `main`.  The PR title must follow Conventional Commits.
3. A project maintainer will review and merge.

## Release process

Releases are triggered by closing a GitHub Milestone.  The milestone title must
be a semver number (e.g. `1.2.0`).  The release workflow will:

1. Create and push the corresponding git tag (`v1.2.0`).
2. Build release binaries via GoReleaser.
3. Push the Docker image to GHCR.

## Code style

- **Go**: `gofmt` + `golangci-lint v2` (see `.golangci.yml`).
- **TypeScript**: `eslint` with the config in `web/eslint.config.js`.
- Write tests for new behaviour; aim for table-driven tests in Go.

## Architecture decisions

Significant design choices are documented in [docs/adr/](docs/adr/).
