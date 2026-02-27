# ADR 0007 — Distroless runtime image

**Status:** Accepted  
**Date:** 2025-02

## Context

Container security best practice is to minimise the attack surface by shipping
only the application binary and its runtime dependencies.

## Decision

Use `gcr.io/distroless/static-debian12:nonroot` as the final stage of the
multi-stage Dockerfile.  The Go binary is statically linked
(`CGO_ENABLED=0`), so no libc is required.

## Consequences

- No shell, no package manager in the final image.
- `kubectl exec` into a running container produces "exec format error" — use
  ephemeral debug containers (`kubectl debug`) instead.
- Image size is typically 5–10 MB (binary + distroless base).
