# ADR 0001 — Use Go stdlib `net/http` `ServeMux`

**Status:** Accepted  
**Date:** 2025-02

## Context

Go 1.22 added method-and-pattern routing (`"GET /api/v1/health"`) to the stdlib
`net/http.ServeMux`, eliminating the main reason to reach for a third-party
router for REST APIs of moderate complexity.

## Decision

Use `net/http.ServeMux` with Go 1.22+ patterns for all routing.

## Consequences

- Zero external router dependencies; smaller binary.
- Path parameter extraction uses `r.PathValue("name")` — no helper library.
- Complex wildcard routing (e.g. `{id...}`) is supported natively.
- If routing complexity grows significantly, migrating to chi or similar is
  straightforward because all handlers implement `http.Handler`.
