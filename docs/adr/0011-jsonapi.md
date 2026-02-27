# ADR 0011 — JSON:API 1.1 response envelope

**Status:** Accepted  
**Date:** 2025-02

## Context

A consistent API response format reduces client-side boilerplate.  Options:

1. Ad-hoc `{"data": ..., "error": ...}` shapes per endpoint.
2. JSON:API 1.1 (standardised envelope with `data`, `errors`, `meta`, `links`).
3. gRPC + protobuf.

## Decision

Use [JSON:API 1.1](https://jsonapi.org/) with a hand-written ~200-line
`internal/api/jsonapi/` package (no external library).

## Consequences

- Consistent `data` / `errors` / `meta` / `links` structure across all
  endpoints.
- Clients can use any JSON:API-aware library (or the raw envelope).
- No code generation required.
- gRPC is deferred; the JSON:API design does not preclude adding gRPC later.
