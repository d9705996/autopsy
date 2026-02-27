# ADR 0004 — River for background jobs

**Status:** Accepted  
**Date:** 2025-02

## Context

Autopsy needs background processing: AI analysis, notification dispatch,
scheduled status-page snapshots.  Options considered:

1. Ad-hoc `goroutine` + in-memory channel (no persistence).
2. A separate message broker (RabbitMQ, Kafka, SQS).
3. Postgres-backed queue library (River, pgqueue, etc.).

## Decision

Use [River](https://riverqueue.com) (`riverqueue/river`) backed by the existing
Postgres pool via `riverpgxv5`.

## Consequences

- Jobs are durable: they survive process restarts.
- No additional infrastructure required.
- River's worker model mirrors Go's concurrency idioms.
- Job queue tables are created via River's own migration utility.
