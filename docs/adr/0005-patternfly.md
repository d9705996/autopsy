# ADR 0005 — PatternFly 6 for the React UI

**Status:** Accepted  
**Date:** 2025-02

## Context

An enterprise incident management UI needs:
- Accessible, keyboard-navigable components.
- Design system coherence for data-heavy dashboards.
- Active maintenance.

## Decision

Use [PatternFly 6](https://www.patternfly.org/) with React 19 and Vite 6.

## Consequences

- PF v6 is a major redesign over v5; APIs have changed (`Text`/`TextContent`
  replaced by `Content`, etc.).
- Bundle size is larger than a minimal component library; acceptable for a
  dashboard-style application.
- The PF design language aligns with Red Hat / OpenShift tools, which is
  appropriate for an ops/SRE-focused product.
