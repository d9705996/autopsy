# ADR 0010 — SPA served via `go:embed`

**Status:** Accepted  
**Date:** 2025-02

## Context

A single deployable binary is simpler than a separate static-file server or
CDN for self-hosted installations.

## Decision

Compile the React SPA into `ui/dist/` via `npm run build --prefix web`.  The
`ui/` package embeds `dist/` using `//go:embed dist` and exposes an
`embed.FS`.  The Go HTTP server serves the SPA from this embedded FS.

## Consequences

- `ui/dist/` must be populated before `go build`.  CI does this in a dedicated
  step.
- `make build` calls `ui-build` as a prerequisite.
- The SPA is always in sync with the binary version.
- Embedding ~500 KB of JS/CSS is negligible for a server binary.
