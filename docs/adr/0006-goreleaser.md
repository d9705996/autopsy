# ADR 0006 — GoReleaser triggered by Milestone close

**Status:** Accepted  
**Date:** 2025-02

## Context

Releases should be easy to cut without manual git tag operations.  The
conventional approach (tag → CI trigger) requires a human to remember to
push the tag correctly.

## Decision

Closing a GitHub Milestone triggers the release workflow.  The milestone title
is the semver version (e.g. `1.2.0`).  The workflow:

1. Creates and pushes `v1.2.0` tag.
2. Runs GoReleaser to build multi-arch binaries and Docker images.
3. Pushes the image to GHCR only.

## Consequences

- Milestones become the release planning artifact (group issues/PRs).
- No NPM publish or PyPI — only GitHub Releases + GHCR.
- `goreleaser.yml` uses `version: 2` format.
