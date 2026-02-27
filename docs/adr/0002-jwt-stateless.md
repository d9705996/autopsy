# ADR 0002 — Stateless JWT with refresh token blocklist

**Status:** Accepted  
**Date:** 2025-02

## Context

A session system is required.  Options considered:

1. Opaque session tokens stored entirely in Postgres (stateful).
2. Stateless JWT access tokens + Postgres refresh token blocklist.
3. Pure stateless JWT with no revocation mechanism.

## Decision

Short-lived (15 min) HS256 JWT access tokens + long-lived (30 day) refresh
tokens stored as SHA-256 hashes in the `refresh_tokens` table.

## Consequences

- Access tokens cannot be revoked individually; the 15-minute window is the
  blast radius.
- Refresh token rotation is enforced: rotating a refresh token revokes the old
  one.
- No Redis or external cache required.
- The `refresh_tokens` table requires occasional cleanup of expired rows.
