# ADR 0003 — Postgres as the only data store

**Status:** Accepted  
**Date:** 2025-02

## Context

Incident management data is relational (organisations → users → incidents →
alerts → timeline events).  A document store would add operational complexity
without benefit for this data model.

## Decision

PostgreSQL 17 is the sole data store.  No Redis, no Elasticsearch, no S3 for
core functionality.

## Consequences

- Single-service `compose.yaml` simplifies local development.
- Full-text search uses Postgres `tsvector`/`tsquery` — sufficient for MVP;
  Elasticsearch can be added later.
- The River job queue uses the same Postgres instance, so no separate message
  broker is required.
