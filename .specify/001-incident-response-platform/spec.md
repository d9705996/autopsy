# Feature Specification: Incident Response Management Platform

**Feature Branch**: `001-incident-response-platform`  
**Created**: 2026-02-25  
**Status**: Draft  
**Stage**: PoC → MVP → Full Product (iterative)

---

## Clarifications

### Session 2026-02-27

- Q: Should the spec treat the database as PostgreSQL-only or DB-driver-agnostic (matching ADR 0012)? → A: DB-driver-agnostic. FR-000f, FR-015a, and SC-000 updated to remove PostgreSQL specificity. FR-000k added to capture the `DB_DRIVER` interchangeability requirement (SQLite default for dev, Postgres for production).
- Q: What should the system do when webhook ingest exceeds the sustained throughput limit? → A: Per-source rate limiting at the webhook middleware layer. Requests over the limit receive `429 Too Many Requests` with a `Retry-After` header; limit is configurable per `WebhookSource` (default 100 req/s). Captured in FR-002a.
- Q: What is the expected behaviour of async features (AI triage, notifications, SLO evaluation) when running with `DB_DRIVER=sqlite`? → A: Graceful degradation. Jobs enqueued to `noopQueue` are logged at WARN with `"job dropped: async features require DB_DRIVER=postgres"`. A startup banner lists degraded features. CI always uses `DB_DRIVER=postgres` for all integration and acceptance tests. Captured in FR-000l.
- Q: How should completed postmortems be matched against incoming alerts to populate the `related_postmortems` field in AI triage context? → A: DB-native full-text search. On Postgres: `tsvector`/`tsquery` GIN index over `postmortems.title` and `root_cause`, ranked by `ts_rank`, top-5 above configured threshold. On SQLite: FTS5 virtual table over same fields. No extra infrastructure required. Captured in FR-033a.
- Q: How should the spec handle API version compatibility within the `/api/v1/` surface? → A: Additive-only changes within `/api/v1/` (new response fields, new optional query parameters, new endpoints) are allowed without a version bump. Any breaking change (field removal, type change, required field addition, endpoint removal) MUST be introduced under a new `/api/v2/` prefix and MUST include a deprecation entry in `CHANGELOG.md`. No `/api/v2/` is planned before the Full Product stage. Captured in FR-034a.

### Session 2026-02-27 (2)

- Q: What should happen when a webhook payload exceeds the configured maximum body size? → A: Global configurable cap via `WEBHOOK_MAX_BODY_BYTES` env var (default 1 MiB), overrideable per `WebhookSource`. Requests exceeding the cap return `413 Request Entity Too Large` with a JSON:API error body; event counted in `webhook_body_too_large_total` metric. Captured in FR-002b.
- Q: How detailed should the RBAC permission matrix be in the spec? → A: Define a minimal PoC/MVP permission table covering only the permissions cited in user stories (`incident:*`, `alert:*`, `postmortem:*`, `user:*`, `admin:*`). Custom-role granularity deferred to Full Product. Captured in FR-036a.
- Q: How should the system handle potential duplicate incidents (two declarations for the same root cause)? → A: Fingerprint-based auto-link. When an AI-triggered incident declaration occurs, if any open incident already has a `linked_alert` sharing the same alert `fingerprint`, the new alert is merged into the existing incident instead of creating a second one, and a `TimelineEntry` of type `dedup_merge` is appended. Manual declarations against a fingerprint already covered by an open incident return `409 Conflict` with a JSON:API pointer to the existing incident. Captured in FR-012a.
- Q: How are time zones handled for on-call schedules spanning DST transitions? → A: Rotation boundaries are stored and evaluated as wall-clock times in the schedule's configured IANA timezone (e.g., `"09:00 America/New_York"`). On-call handoffs always occur at the same clock time; shift durations may be 23 or 25 hours during DST changeovers — documented behaviour, not an error. Captured in FR-017a; `OnCallSchedule` entity updated with `timezone` attribute.

### Session 2026-02-27 (3)

- Q: How does the status page behave during a database outage? → A: In-process snapshot cache (TTL configurable, default 30 s). On DB outage the last cached snapshot is served with `Cache-Control: public, max-age=60, stale-while-revalidate=300` headers so upstream CDN/proxy can continue serving it. No public degraded banner (avoid false alarm). `GET /api/v1/ready` returns `503` to signal the operator. Captured in FR-025a.
- Q: How should the SLO engine handle gaps in SLI data (missing Prometheus scrapes)? → A: Configurable per SLO. Gaps ≤ `max_gap_seconds` (default 300 s) are treated as good (1.0 availability) to prevent false burn-rate alerts from brief scrape failures. Gaps exceeding the threshold are classified as `unknown` and excluded from the burn-rate calculation window entirely. Captured in FR-027a.
- Q: What happens if a postmortem action item's owner is deactivated before the item is closed? → A: Soft-deactivation (`active=false` on the User record). Open action items remain assigned to the deactivated user but are discoverable via `GET /api/v1/action-items?owner_active=false`. The deactivation API response MUST include a `meta.warnings` field stating the count of open action items needing reassignment. Admins are responsible for manual reassignment. Captured in FR-032a; `User` entity gains `active` attribute.
- Q: What cursor format should the pagination system use (FR-039)? → A: Opaque base64url-encoded keyset cursor encoding `last_seen_id` + `last_seen_created_at`. Stable under concurrent inserts. Expires after 24 h. Callers must treat cursors as opaque; constructing cursors manually is forbidden. Captured as an update to FR-039.
- Q: What JWT signing key rotation model should the auth system implement? → A: Two env vars: `JWT_SECRET` (current, used to sign new tokens) and `JWT_SECRET_PREV` (previous, accepted for verification only). During rotation, set the old secret as `JWT_SECRET_PREV` and the new secret as `JWT_SECRET`. Tokens issued under the previous key remain valid until natural expiry; no forced logouts. Captured in FR-015e.

### Session 2026-02-27 (4)

- Q: Is an HMAC secret required when creating a WebhookSource, or can endpoints be unauthenticated? → A: HMAC secret is mandatory for all `WebhookSource` records. Creating a source without `hmac_secret` MUST return `422 Unprocessable Entity`. Requests arriving without a valid `X-Hub-Signature-256` header MUST return `401 Unauthorized` before the payload is parsed. No unauthenticated webhook endpoints are permitted. Captured as an update to FR-002.
- Q: Is `Alert.severity` the raw webhook hint or the AI-assigned value? → A: Two separate fields. `severity_hint` holds the raw value from the webhook payload (normalised string, immutable after insert). `severity` holds the AI-assigned SEV1–SEV4 value (null until triage completes, never null after). The UI displays `severity` when populated, `severity_hint` as a fallback. Captured as an update to `Alert` entity and FR-006a.
- Q: Who can approve a postmortem for publication (`in_review` → `published`) and what does approval require? → A: Single-approver model at PoC/MVP. Any user with `postmortem:publish` permission (Incident Commander or Admin) may transition a postmortem to `published` via `PATCH /api/v1/postmortems/:id` with `{status: "published"}`. No separate approval record or endpoint is required; the transition is recorded as a standard audit log event. Multi-reviewer sign-off deferred to Full Product. Captured in FR-030a.
- Q: FR-024 mandates email + RSS subscriptions for the status page but FR-015c defers all email infrastructure to MVP. How should the stages be aligned? → A: RSS feed (`/status/feed.rss`) ships at PoC — no email infra required. Status page email subscriptions are deferred to MVP to align with the email infrastructure decision. FR-024 updated to stage-label each.
- Q: What happens when an on-call notification channel delivery fails (e.g., Slack/email returns 5xx)? → A: Each channel delivery is retried with exponential backoff (max 3 attempts). If all channels for an escalation tier fail after all retries, the failure is recorded as a `TimelineEntry` of type `page` with `status: failed` and escalation immediately proceeds to the next tier without waiting for the normal timeout. Captured in FR-019a.

---

## Background & Intent

Autopsy is an open-source incident response management platform aimed at reducing the frequency, duration, and organisational cost of production incidents. The product is strongly influenced by [Grafana IRM](https://grafana.com/products/cloud/irm), [Cachet](https://cachethq.io/), and the [Google SRE Book](https://sre.google/sre-book/table-of-contents/).

The core value proposition is: **less incidents, shorter outages, faster deployments**. It achieves this by combining intelligent alert triage (AI agent), structured incident lifecycle management, SLO/SLI tracking, on-call scheduling, post-incident learning, and a public status page — all through a consistent JSON:API-conformant REST API and a responsive frontend UI.

This spec covers the full product vision. Work will be delivered in three stages:

| Stage | Goal | User-Visible Outcome |
|---|---|---|
| **Foundation** | Repository scaffold, CI/CD, linting, bare-bones app, local + Docker + Kubernetes runnable, `organization_id` schema hook in first migration | Any contributor can clone, build, and run in < 5 minutes |
| **PoC** | Validate core alert ingestion → AI triage → incident declaration loop | Alerts arrive, AI triages them, incidents appear in the UI |
| **MVP** | On-call, status page, basic SLO tracking, postmortems, RBAC, JWT auth, multi-tenancy (org isolation enforced) | Self-hostable, useable by a multi-team engineering organisation |
| **Full Product** | Advanced SLO, integrations, OIDC SSO, custom roles, enterprise docs | Feature parity with commercial IRM tools |

---

## User Scenarios & Testing *(mandatory)*

### User Story 0 — Foundation & Scaffolding (Priority: P0) `[Foundation]`

A new contributor clones the repository and, by following the README, has a fully functional local development environment running within 5 minutes. The repository ships with CI/CD pipelines, enforced linting and formatting, a minimal but deployable application skeleton (API server + frontend shell), a `docker compose up` path for local work, and a Helm chart / Kubernetes manifest set for production-style deployments. Developer documentation covers: local setup, contribution guidelines, the branching strategy, how to run tests, and how to cut a release.

**Why this priority**: Every subsequent user story builds on a healthy, consistent foundation. Without CI gates, linting enforcement, and a runnable skeleton, parallel feature development will produce unintegrateable code and on-boarding will be a blocker.

**Independent Test**: A fresh clone on a machine with only Go and Node.js installed (no Docker required) should reach a running, health-check-passing application using `make dev` (SQLite mode). Alternatively, a machine with Docker can use `docker compose up`. CI must pass on a new branch with zero code changes from `main`.

**Acceptance Scenarios**:

1. **Given** a developer clones the repository and runs the documented single bootstrap command (e.g., `make dev` or `docker compose up`), **When** the command completes successfully, **Then** the API server responds `200 OK` on `GET /api/v1/health` and the frontend shell is reachable in a browser.
2. **Given** a pull request is opened against `main`, **When** the CI pipeline runs, **Then** the following gates must all pass: build (Go + frontend), `go vet`, `golangci-lint`, `staticcheck`, unit tests with `go test -race ./...`, and a Docker image build.
3. **Given** a developer pushes a Go file that violates `golangci-lint` rules or a frontend file that fails the configured formatter, **When** the lint CI job runs, **Then** the pipeline fails with a clear, actionable error indicating the file and rule violated.
4. **Given** a tagged release commit (e.g., `v0.1.0`) is pushed to `main`, **When** the release CI pipeline runs, **Then** a multi-arch Docker image is published to the configured container registry and a GitHub Release with changelog and binary artefacts is created.
5. **Given** a developer runs `helm install autopsy ./charts/autopsy` against a local Kubernetes cluster (e.g., kind or k3d) with default values, **When** all pods reach `Running` state, **Then** `GET /api/v1/health` returns `200 OK` from within the cluster, and the ingress (if enabled) routes external traffic correctly.
6. **Given** a developer reads the `CONTRIBUTING.md` and `docs/development.md`, **When** they follow the documented steps to run tests, add a new linting rule, or cut a local Docker image, **Then** each documented command succeeds without requiring undocumented pre-requisites.

---

### User Story 1 — Alert Ingestion & AI Triage (Priority: P1) `[PoC]`

An SRE has Grafana Alerting configured. When a production alert fires, it is delivered to Autopsy via webhook. An AI agent examines the alert payload, assigns a severity (SEV1–SEV4), proposes a summary and probable cause, and records the triage result. The SRE sees the triage in the UI within seconds.

**Why this priority**: This is the foundational capability — every other feature depends on alerts arriving and being understood. Without it, no value is delivered.

**Independent Test**: Configure a Grafana webhook to POST a synthetic alert to `POST /api/v1/webhooks/grafana`. Verify triage record appears in `GET /api/v1/alerts` with AI-assigned severity and summary fields populated.

**Acceptance Scenarios**:

1. **Given** a valid Grafana Alerting webhook payload is POSTed to `/api/v1/webhooks/grafana`, **When** the request is authenticated and the payload is valid, **Then** an `Alert` resource is created, an AI triage job is enqueued, and the response is `202 Accepted` with the alert ID.
2. **Given** the AI triage job runs, **When** it completes successfully, **Then** the `Alert` record is updated with `severity`, `ai_summary`, `probable_cause`, and `suggested_actions` fields within 30 seconds.
3. **Given** an alert arrives with an invalid or missing required field, **When** the webhook handler validates the payload, **Then** a `422 Unprocessable Entity` response is returned with a JSON:API error body describing the missing field.
4. **Given** the AI provider is unavailable, **When** the triage job fails after retries, **Then** the alert is marked `triage_status: failed`, an error is logged with trace ID, and an operator notification is emitted.
5. **Given** a duplicate alert (same `fingerprint`) arrives within the deduplication window, **When** the webhook handler processes it, **Then** the existing alert is updated (not duplicated) and a `deduplicated: true` field is set.

---

### User Story 2 — Incident Declaration & Lifecycle Management (Priority: P1) `[PoC]`

When an AI triage determines an alert warrants an incident (or an SRE manually escalates), an Incident is declared. The incident has a structured lifecycle: `declared → investigating → identified → monitoring → resolved`. The AI agent participates as a responder, posting timeline updates and suggesting RCA hypotheses.

**Why this priority**: Incident lifecycle management is the core product — it converts alert noise into managed, auditable incidents.

**Independent Test**: Trigger incident declaration via `POST /api/v1/incidents` (manual or AI-automated). Walk the incident through all lifecycle states via PATCH calls. Verify timeline entries are created at each transition and AI posts at least one RCA hypothesis.

**Acceptance Scenarios**:

1. **Given** an alert with severity SEV1 or SEV2 is triaged, **When** the AI agent applies the escalation policy, **Then** an `Incident` resource is automatically created, linked to the triggering alert, and the declaring user (or AI agent identity) is recorded.
2. **Given** an open incident, **When** a responder transitions it to `investigating` via `PATCH /api/v1/incidents/:id`, **Then** the previous state, new state, actor, and timestamp are appended to the incident timeline.
3. **Given** an open incident, **When** the AI agent analyses linked alerts and logs, **Then** it posts at least one `TimelineEntry` of type `ai_hypothesis` containing a root cause hypothesis and confidence score.
4. **Given** an incident is resolved, **When** `PATCH /api/v1/incidents/:id` sets `status: resolved`, **Then** the incident is closed with a `resolved_at` timestamp, linked alerts are closed, and a postmortem stub is automatically created.
5. **Given** a resolved incident, **When** a user attempts to transition it back to `investigating`, **Then** a `409 Conflict` response is returned unless the requester has the `incident:reopen` permission.

---

### User Story 3 — On-Call Scheduling & Notifications (Priority: P2) `[MVP]`

Engineering managers define on-call schedules (rotations, overrides). When a SEV1/SEV2 incident is declared and not acknowledged within the escalation timeout, Autopsy pages the on-call engineer via their configured notification channels (email, SMS, Slack, PagerDuty webhook). The engineer can acknowledge, escalate, or resolve from the notification.

**Why this priority**: Without human escalation, AI triage alone cannot close the loop on critical incidents.

**Independent Test**: Create a schedule with a rotation containing one user. Trigger a SEV1 incident and leave it unacknowledged past the escalation timeout. Verify a notification record is created and the configured channel receives the page.

**Acceptance Scenarios**:

1. **Given** an on-call schedule is active, **When** a SEV1 incident is declared and not acknowledged within the configured `escalation_timeout_seconds`, **Then** the current on-call user is paged via all their enabled notification channels.
2. **Given** multiple escalation tiers are configured, **When** tier-1 on-call does not acknowledge within their timeout, **Then** the incident is escalated to tier-2 and a timeline entry records the escalation.
3. **Given** a schedule override is active, **When** an incident triggers a page, **Then** the override user is paged instead of the rotation user.
4. **Given** an on-call user acknowledges the incident, **When** they respond via any notification channel action, **Then** the incident `acknowledged_at` and `acknowledging_user` fields are populated and further pages for the current tier are suppressed.
5. **Given** all escalation tiers are exhausted without acknowledgement, **When** the final timeout expires, **Then** the incident is escalated to a `CRITICAL_UNACKNOWLEDGED` state and a notification is sent to the configured fallback (e.g., team Slack channel).

---

### User Story 4 — Public Status Page (Priority: P2) `[MVP]`

The organisation publishes a Cachet-inspired public status page at a configurable URL. The page shows overall system health, per-component status, and a history of past incidents. When an incident is declared or resolved, the status page updates automatically. External users (customers, stakeholders) can subscribe to email/RSS updates.

**Why this priority**: Proactive external communication reduces inbound support load during incidents and builds customer trust.

**Independent Test**: Create a component and two status page entries. Declare an incident mapped to the component. Verify component status on `GET /status` changes to `degraded` and an incident banner appears. Resolve the incident and verify status returns to `operational`.

**Acceptance Scenarios**:

1. **Given** a component is mapped to an incident, **When** the incident transitions to `investigating`, **Then** the component status on the public status page changes to `degraded` (or `partial_outage` / `major_outage` based on severity).
2. **Given** a user visits the public status page, **When** there are no active incidents, **Then** all components show `operational` and a "All Systems Operational" banner is displayed; the page loads in < 500 ms from CDN/cache.
3. **Given** a subscriber has registered their email, **When** an incident affecting their subscribed components is created or resolved, **Then** an email notification is dispatched within 60 seconds.
4. **Given** an incident is resolved, **When** all component-incident mappings are cleared, **Then** affected component statuses revert to `operational` and a resolution notice appears in the incident history.
5. **Given** the status page RSS feed URL is fetched, **When** incidents exist, **Then** a valid RSS 2.0 document is returned listing the ten most recent incidents with titles, links, and published dates.

---

### User Story 5 — SLO/SLI Management (Priority: P2) `[MVP]`

SREs define Service Level Objectives (SLOs) with associated Service Level Indicators (SLIs) and error budgets. The platform continuously evaluates SLI data (ingested from Prometheus/Grafana), calculates burn rate, and alerts when the error budget is at risk. Incident impact is automatically attributed to SLO error budget consumption.

**Why this priority**: SLO management is the quantitative foundation for balancing reliability and deployment velocity — the product's core value differentiation.

**Independent Test**: Define an SLO with a 99.9% target and a rolling 30-day window. Import a synthetic SLI time series with two hours of errors. Verify error budget consumption is calculated correctly and a burn-rate alert fires when the fast-burn threshold is crossed.

**Acceptance Scenarios**:

1. **Given** an SLO is defined with a target and evaluation window, **When** SLI data is ingested, **Then** the error budget remaining is recalculated and the current value is available on `GET /api/v1/slos/:id`.
2. **Given** the 1-hour burn rate exceeds the configured fast-burn threshold (e.g., 14× nominal), **When** the evaluation job runs, **Then** an alert of type `slo_burn_rate` is created and the standard AI triage pipeline is triggered.
3. **Given** an incident is resolved, **When** the incident duration and affected SLOs are known, **Then** the error budget consumed by the incident is attributed to the relevant SLOs and recorded in the incident record.
4. **Given** an SLO's error budget drops below 10% of the period's allocation, **When** the evaluation runs, **Then** a `BudgetExhaustion` warning event is emitted and the SLO status changes to `at_risk`.
5. **Given** an SLO is queried for its history, **When** `GET /api/v1/slos/:id/history?window=30d` is called, **Then** a time-series of daily error budget consumption is returned as a JSON array.

---

### User Story 6 — Post-Incident Postmortem Management (Priority: P3) `[MVP]`

After an incident is resolved, a postmortem is automatically stubbed with the incident timeline, contributing alerts, AI hypotheses, and impacted SLOs pre-populated. An incident commander completes the postmortem (root cause, contributing factors, action items). Action items are tracked to closure. Completed postmortems are searchable and inform future AI triage context.

**Why this priority**: Postmortems are the primary mechanism for reducing future incident frequency — closing the improvement loop.

**Independent Test**: Resolve an incident and verify a postmortem stub is auto-created with timeline and alert data. Complete the postmortem with root cause text and two action items. Verify action items appear in `GET /api/v1/action-items` and are linkable to their source postmortem.

**Acceptance Scenarios**:

1. **Given** an incident is resolved, **When** the resolution transition occurs, **Then** a `Postmortem` resource is automatically created with `status: draft`, linked to the incident, and pre-populated with the incident timeline, contributing alerts, AI hypotheses, and impacted SLOs.
2. **Given** a postmortem is in `draft`, **When** an incident commander submits it with `root_cause`, `contributing_factors`, and at least one `action_item`, **Then** the postmortem status transitions to `in_review`.
3. **Given** a postmortem is approved, **When** status transitions to `published`, **Then** it becomes readable by all authenticated users and its content is indexed for future AI triage context retrieval.
4. **Given** an action item is created from a postmortem, **When** `GET /api/v1/action-items?postmortem_id=:id` is called, **Then** all action items with their `status`, `owner`, `due_date`, and `postmortem_id` are returned.
5. **Given** a new alert arrives whose fingerprint or description semantically matches a past postmortem root cause, **When** the AI triage runs, **Then** a `related_postmortems` field is populated with links to relevant postmortems in the triage output.

---

### User Story 7 — RBAC & User Management (Priority: P3) `[MVP]`

Operators manage users, teams, and roles. Built-in roles: `Viewer`, `Responder`, `Incident Commander`, `Admin`. Custom roles with granular permissions are supported. All API actions are gated by RBAC. SSO via OIDC is supported for enterprise deployments.

**Why this priority**: Required for any multi-user, enterprise-grade deployment; blocks on-call and incident commander workflows.

**Independent Test**: Create a user with the `Viewer` role. Attempt to create an incident via `POST /api/v1/incidents` as that user. Verify a `403 Forbidden` response is returned. Elevate to `Responder` role and verify the same call succeeds.

**Acceptance Scenarios**:

1. **Given** a user with the `Viewer` role is authenticated, **When** they attempt any mutating API call (`POST`, `PATCH`, `DELETE`), **Then** a `403 Forbidden` response with a JSON:API error body is returned.
2. **Given** an Admin creates a new user, **When** `POST /api/v1/users` is called with valid email and role, **Then** the user is created and receives an onboarding email with a one-time setup link.
3. **Given** the OIDC provider is configured, **When** a user authenticates via the SSO flow, **Then** their identity is mapped to a local user record (auto-provisioned on first login) and a session token is issued.
4. **Given** a custom role is defined with `incident:read` and `incident:update` but not `incident:create`, **When** a user with that role calls the incidents API, **Then** GET and PATCH succeed (200/200) and POST returns 403.
5. **Given** an admin revokes a user's session, **When** the user's existing token is used, **Then** a `401 Unauthorized` response is returned on all subsequent requests.

---

### User Story 8 — Multi-Source Webhook Ingestion (Priority: P3) `[Full Product]`

Beyond Grafana, the platform accepts webhooks from Prometheus Alertmanager, PagerDuty (bi-directional sync), Datadog, and a generic JSON webhook with a configurable field-mapping DSL. Each source is configured per-organisation with a unique HMAC-signed endpoint.

**Why this priority**: Broader integrations expand addressable market but are not required for the core loop.

**Independent Test**: Configure a generic webhook source with a field mapping. POST a synthetic payload. Verify the alert is created with fields mapped according to the configuration.

**Acceptance Scenarios**:

1. **Given** a webhook source is configured with an HMAC secret, **When** a request arrives with a valid `X-Hub-Signature-256` header, **Then** the payload is accepted; if the signature is invalid, `401 Unauthorized` is returned.
2. **Given** a generic webhook source with a field mapping config, **When** a payload with mapped fields is received, **Then** an `Alert` is created with the correct `title`, `severity_hint`, `source`, and `labels` fields.
3. **Given** a PagerDuty integration is configured, **When** an incident is created in Autopsy, **Then** a corresponding PagerDuty incident is opened via the PagerDuty Events API v2.

---

### Edge Cases

- What happens when the AI provider (e.g., OpenAI/Anthropic) returns a rate-limit error during triage? *(Handled by FR-009: exponential backoff, max 3 retries, then `triage_status: failed`.)*
- How does the system handle an alert storm (> 1000 alerts/minute from a single source)? Per-source rate limiting (FR-002a) enforces a configurable req/s ceiling per `WebhookSource`; requests over the limit receive `429 Too Many Requests` with a `Retry-After` header. Queue backpressure (worker concurrency + River queue depth) provides additional protection at the processing layer.
- What happens when async features are invoked with `DB_DRIVER=sqlite`? Jobs are dropped with a WARN log and metric increment (FR-000l); a startup banner lists degraded features. Async acceptance tests always run against `DB_DRIVER=postgres`.
- What happens when a webhook payload exceeds the configured maximum body size? *(Handled by FR-002b: `413 Request Entity Too Large` with JSON:API error body; global `WEBHOOK_MAX_BODY_BYTES` cap, default 1 MiB, overrideable per `WebhookSource`, counted in `webhook_body_too_large_total`.)*
- How are time zones handled for on-call schedules spanning DST transitions? *(Handled by FR-017a: rotation boundaries stored as wall-clock times + IANA timezone; handoffs always at the same clock time; shift durations may be 23/25 h during DST changeovers — documented behaviour.)*
- What happens if a postmortem action item's owner is deactivated before the item is closed? *(Handled by FR-032a: soft-deactivate user (`active=false`); open action items remain assigned; discoverable via `?owner_active=false` filter; deactivation response includes `meta.warnings` count for Admin reassignment.)*
- How does the status page behave during a database outage (should serve from cache/CDN)? *(Handled by FR-025a: in-process snapshot cache, TTL 30 s default; stale snapshot served on DB outage with `stale-while-revalidate` headers; no public degraded banner; `GET /api/v1/ready` returns `503` for operator alerting.)*
- What happens when two incidents are declared for the same root cause (duplicate detection)? *(Handled by FR-012a: fingerprint-based auto-link merges alerts into open incidents; manual duplicate declarations return `409 Conflict` with pointer to existing open incident.)*
- How does SLO calculation handle gaps in SLI data (missing scrapes)? *(Handled by FR-027a: gaps ≤ `max_gap_seconds` (default 300 s) treated as good; gaps exceeding the threshold are classified `unknown` and excluded from burn-rate windows.)*

---

## Requirements *(mandatory)*

### Functional Requirements

**Foundation & Scaffolding**

- **FR-000**: The repository MUST contain a `Makefile` (or equivalent task runner) with targets: `build`, `test`, `lint`, `dev`, `docker-build`, and `release`.
- **FR-000a**: CI MUST run on every pull request and include: Go build, `go vet`, `golangci-lint`, `staticcheck`, `go test -race ./...`, and a Docker image build.
- **FR-000b**: On push of a semver tag to `main`, CI MUST publish a multi-arch (`linux/amd64`, `linux/arm64`) Docker image and create a GitHub Release with changelog notes and binary artefacts.
- **FR-000c**: The repository MUST include a `docker-compose.yml` (or `compose.yaml`) that starts the full local development infrastructure (database only) with a single command; the Go API server runs natively via `go run ./cmd/autopsy` for fast hot-reload iteration.
- **FR-000d**: The repository MUST include a Helm chart under `charts/autopsy/` that targets Kubernetes 1.28+ and provides configurable values for image, replicas, ingress, resource limits, and external secret references.
- **FR-000e**: `GET /api/v1/health` MUST return `200 OK` with a JSON body containing at minimum: `status`, `version`, and `uptime_seconds`.
- **FR-000f**: `GET /api/v1/ready` MUST return `200 OK` only when the active database is reachable (SQLite file accessible or Postgres connection pool responding to `SELECT 1`); otherwise `503 Service Unavailable`.
- **FR-000g**: The repository MUST include: `README.md` (overview, quick-start, badges), `CONTRIBUTING.md` (setup, branching strategy, PR process), `docs/development.md` (detailed local setup, test execution, linting, releasing), `docs/adr/0001-record-architecture-decisions.md` (foundation ADR), and `CHANGELOG.md`.
- **FR-000h**: All Go code MUST be formatted with `gofmt`; frontend code MUST be formatted with the project-configured formatter (e.g., Prettier); formatting violations MUST fail CI.
- **FR-000i**: `.devcontainer/devcontainer.json` MUST be present so the repository opens into a fully configured development environment in VS Code / GitHub Codespaces without additional manual setup.
- **FR-000j**: All core database tables MUST include a nullable `organization_id UUID` column (no foreign key, no enforcement) from the first migration as a schema hook for future multi-tenancy. This incurs zero runtime cost and avoids a costly backfill later.
- **FR-000k**: The platform MUST support interchangeable database drivers selected via a `DB_DRIVER` environment variable (`sqlite` | `postgres`). The default MUST be `sqlite`, enabling a zero-infrastructure local development experience (no Docker, no Postgres required). `postgres` MUST be used for CI, staging, and production. All application code MUST interact only with the ORM abstraction layer (`*gorm.DB`); no package outside `internal/db` and `internal/worker` may import a driver-specific package.
- **FR-000l**: When `DB_DRIVER=sqlite`, the platform MUST degrade async features (AI triage, on-call notifications, SLO evaluation, postmortem indexing) gracefully. Jobs submitted to the no-op queue MUST be logged at `WARN` level with the message `"job dropped: async features require DB_DRIVER=postgres"` and counted in a `worker_jobs_dropped_total` metric. The server MUST emit a startup log line explicitly listing which async features are non-functional in SQLite mode. All CI pipelines, integration tests, and PoC/MVP acceptance tests MUST run with `DB_DRIVER=postgres`.

**Alert Ingestion**

- **FR-001**: System MUST expose a webhook endpoint per configured source (`/api/v1/webhooks/:source`) that accepts HTTP POST requests.
- **FR-002**: System MUST validate webhook signatures (HMAC-SHA256) before processing payloads. Every `WebhookSource` MUST have a non-null `hmac_secret`; attempting to create a `WebhookSource` without one MUST return `422 Unprocessable Entity`. Webhook requests that arrive without a valid `X-Hub-Signature-256` header (or whose computed HMAC does not match) MUST be rejected with `401 Unauthorized` before any payload parsing occurs. There are no unauthenticated webhook endpoints.
- **FR-002a**: System MUST enforce per-source rate limiting on webhook ingest. Requests from a single `WebhookSource` that exceed the configured limit MUST receive `429 Too Many Requests` with a `Retry-After` header indicating the next acceptable request time. The rate limit is configurable per `WebhookSource` (default: 100 requests/second). Rate-limit events MUST be logged and counted in a `webhook_rate_limited_total` metric counter labelled by `source_id`.
- **FR-002b**: The webhook ingest middleware MUST enforce a maximum request body size. The global default is `1 MiB` (1 048 576 bytes), configurable via the `WEBHOOK_MAX_BODY_BYTES` environment variable. Individual `WebhookSource` records MAY override this limit. Requests whose body exceeds the effective limit MUST be rejected with `413 Request Entity Too Large` and a JSON:API-conformant error body before the payload is parsed. Every rejection MUST be counted in a `webhook_body_too_large_total` metric counter labelled by `source_id`.
- **FR-003**: System MUST deduplicate alerts using a configurable fingerprint field within a configurable time window (default: 5 minutes).
- **FR-004**: System MUST enqueue AI triage asynchronously (non-blocking response to webhook sender).
- **FR-005**: System MUST support at minimum: Grafana Alerting, Prometheus Alertmanager, and a generic JSON source in PoC/MVP.

**AI Triage**

- **FR-006**: System MUST assign an AI-derived severity (SEV1–SEV4) to each ingested alert.
- **FR-006a**: Every `Alert` record MUST store two distinct severity attributes: `severity_hint` (the raw severity value extracted from the incoming webhook payload, e.g. `"critical"` / `"warning"` / `"info"`; normalised to a lowercase string; immutable after initial insert; may be `null` if the source payload contains no severity field) and `severity` (the AI-assigned `SEV1` | `SEV2` | `SEV3` | `SEV4` value; `null` until the AI triage job completes; MUST be non-null on `triage_status: completed`). UI rendering MUST display `severity` when non-null; `severity_hint` when `severity` is still null (pre-triage). Invalid AI-returned severity values MUST default to `SEV4` and log a WARN.
- **FR-007**: System MUST produce: `ai_summary`, `probable_cause`, `suggested_actions`, and `confidence_score` for each triaged alert.
- **FR-008**: AI triage MUST complete within 30 seconds of alert creation under normal provider conditions.
- **FR-009**: System MUST retry failed AI triage jobs with exponential backoff (max 3 attempts).
- **FR-010**: AI triage MUST be configurable per alert source (model, prompt template, escalation policy).
- **FR-011**: System MUST make AI triage explainable — the prompt sent and response received MUST be stored with the triage record.

**Incident Management**

- **FR-012**: System MUST support incident lifecycle states: `declared → investigating → identified → monitoring → resolved`.
- **FR-012a**: The system MUST prevent duplicate incident creation using alert fingerprint matching. When an AI-triggered incident declaration is attempted and an open incident already exists whose `linked_alerts` contain any alert with the same `fingerprint`, the new alert MUST be auto-linked to the existing incident instead of creating a new one; a `TimelineEntry` of type `dedup_merge` MUST be appended to the existing incident's timeline. When a user manually declares an incident and the triggering alert's `fingerprint` already appears in an open incident, the API MUST return `409 Conflict` with a JSON:API error body that includes a `related` pointer to the existing incident's URL. Manual declarations that explicitly supply a unique title and no triggering alert are not subject to this check.
- **FR-013**: System MUST maintain an immutable, append-only timeline for every incident.
- **FR-014**: System MUST support manual incident declaration by any user with `incident:create` permission.
- **FR-015**: System MUST link incidents to one or more triggering alerts, impacted components, and impacted SLOs.
- **FR-016**: The AI agent MUST automatically post RCA hypotheses as timeline entries during the `investigating` state.

**Authentication** *(decision: Q1, Q3, G4)*

- **FR-015a**: The platform MUST issue stateless JWTs for authentication: a short-lived access token (15-minute expiry) and a long-lived refresh token (30-day expiry) stored in a refresh-token blocklist table for revocation support. The blocklist table is managed by GORM and exists in both SQLite (dev) and Postgres (production) via the DB driver abstraction (FR-000k).
- **FR-015e**: The JWT signing key MUST be configurable via two environment variables: `JWT_SECRET` (required; used to sign all newly issued tokens) and `JWT_SECRET_PREV` (optional; accepted for token verification only, never used to sign new tokens). During token verification, the system MUST first attempt verification with `JWT_SECRET`; if that fails with a signature error, it MUST retry with `JWT_SECRET_PREV` if set. This enables zero-downtime key rotation: set the old `JWT_SECRET` as `JWT_SECRET_PREV` and the new secret as `JWT_SECRET` without forcing users to re-authenticate. Tokens signed under the previous key remain valid until their natural expiry. If neither key succeeds, `401 Unauthorized` is returned. `JWT_SECRET_PREV` MUST be unset (or rotated out) once all tokens issued under the previous key have expired.
- **FR-015b**: A default seed admin user (`admin@autopsy.local` / generated secret printed to stdout on first boot) MUST be created when the database has no users. The secret MUST NOT be hard-coded; it MUST be a cryptographically random string.
- **FR-015c**: Email notification dispatch is deferred to MVP. No email infrastructure (SMTP, SES, etc.) is required in Foundation or PoC.

**AI Provider** *(decision: Q2)*

- **FR-015d**: AI triage MUST be routed through a provider-agnostic Go interface (`AIProvider`). The concrete implementation is injected via configuration (`ai.provider` env var). At PoC, a single implementation (e.g., OpenAI-compatible) is sufficient; additional providers are added without changing the core triage logic.

**On-Call**

- **FR-017**: System MUST support rotation-based on-call schedules with override capability.
- **FR-017a**: `OnCallSchedule` records MUST include a `timezone` field containing a valid IANA timezone string (e.g., `"America/New_York"`, `"Europe/London"`, `"UTC"`). Rotation boundary times (shift start/end) MUST be stored and evaluated as wall-clock times in the schedule's configured timezone using Go's `time.LoadLocation` / `time.In`. An invalid or unknown IANA timezone string MUST be rejected at create/update time with `422 Unprocessable Entity`. During DST transitions, shift durations may be 23 or 25 hours; this is expected behaviour and MUST be noted in the operator documentation. The effective on-call engineer at any moment is determined by evaluating `now.In(schedule.timezone)` against the rotation layer boundaries.
- **FR-018**: System MUST support multi-tier escalation policies per team.
- **FR-019**: System MUST notify on-call via at minimum: email and a configurable webhook (for Slack/Teams), with SMS and push notification as future channels.
- **FR-019a**: Each on-call notification channel delivery MUST be retried with truncated exponential backoff on transient failure (HTTP 5xx, connection timeout). The maximum number of delivery attempts per channel per page event is 3. If all configured channels for an escalation tier have exhausted their retry attempts without successful delivery, the system MUST record a `TimelineEntry` of type `page` with `delivery_status: all_channels_failed` and MUST immediately escalate to the next tier in the escalation policy without waiting for the normal `escalation_timeout_seconds`. If no further tier exists, the incident transitions to `CRITICAL_UNACKNOWLEDGED` state (per US3 AC5). All delivery attempts (successful and failed) MUST be recorded in the incident timeline.
- **FR-020**: System MUST record every page attempt and acknowledgement in the incident timeline.

**Status Page**

- **FR-021**: System MUST serve a publicly accessible status page without authentication.
- **FR-022**: System MUST support per-component status with at least: `operational`, `degraded_performance`, `partial_outage`, `major_outage`, `maintenance`.
- **FR-023**: System MUST automatically update component status from incident lifecycle transitions.
- **FR-024**: System MUST support status page update subscriptions. `[PoC]` RSS feed (`/status/feed.rss`, valid RSS 2.0) MUST be available without authentication and MUST list the ten most recent incidents with titles, links, and published dates; no infrastructure beyond the Go server is required. `[MVP]` Email subscription (subscribe/unsubscribe via API or status page UI) is introduced at MVP alongside the email notification infrastructure (see FR-015c stage alignment). Email subscription endpoints are stubs in PoC (return `501 Not Implemented`).
- **FR-025**: The status page MUST be renderable from a CDN-cacheable static endpoint (< 500 ms TTFB globally).
- **FR-025a**: The status page endpoint (`GET /status`) MUST be backed by an in-process snapshot cache. The cache MUST be refreshed from the database on a configurable interval (`STATUS_CACHE_TTL_SECONDS`, default `30`). When a cache refresh fails because the database is unreachable, the previously cached snapshot MUST be served with HTTP response headers `Cache-Control: public, max-age=60, stale-while-revalidate=300` to allow upstream CDN and reverse proxies to continue serving it. No degraded-mode banner or warning MUST be displayed to public visitors. Operators are notified of the database outage exclusively via the `GET /api/v1/ready` probe returning `503 Service Unavailable` (FR-000f).

**SLO/SLI**

- **FR-026**: System MUST support SLO definition with: indicator type, target percentage, evaluation window, and error budget policy.
- **FR-027**: System MUST ingest SLI data from Prometheus-compatible query APIs.
- **FR-027a**: The SLO evaluation engine MUST handle gaps in SLI time-series data using a configurable per-SLO gap policy. Gaps between consecutive data points that are shorter than or equal to `slo.max_gap_seconds` (default: `300`, configurable per SLO record) MUST be treated as good (availability = 1.0) to prevent false burn-rate alerts from transient Prometheus scrape failures. Gaps that exceed `max_gap_seconds` MUST be classified as `unknown` and excluded from all burn-rate calculation windows (i.e., the window denominator is reduced by the unknown interval duration rather than counting it as either good or bad). The `unknown` classification and its duration MUST be surfaced in the SLO history API response (`GET /api/v1/slos/:id/history`).
- **FR-028**: System MUST calculate fast-burn (1h × 14) and slow-burn (6h × 6) alert thresholds per the Google SRE burn-rate model.
- **FR-029**: System MUST attribute incident duration to error budget consumption for affected SLOs.

**Postmortem**

- **FR-030**: System MUST auto-create a postmortem stub on incident resolution.
- **FR-030a**: The postmortem state machine transitions are: `draft` → `in_review` (by any user with `postmortem:update`) → `published` (by any user with `postmortem:publish`). All transitions occur via `PATCH /api/v1/postmortems/:id` with the appropriate `status` value. Reverse transitions (`published` → `in_review`, `in_review` → `draft`) are forbidden and MUST return `422 Unprocessable Entity`. The `published` state is terminal except by Admin override (deferred to Full Product). Each transition is recorded in the `AuditLog` (FR-038a). No separate approval endpoint or approval record is required at PoC/MVP; multi-reviewer sign-off is deferred to the Full Product stage.
- **FR-031**: Postmortem MUST be pre-populated with: timeline, contributing alerts, AI hypotheses, impacted SLOs, and incident metadata.
- **FR-032**: System MUST track action items to closure, with owner, due date, and status.
- **FR-032a**: User records MUST support soft-deactivation via a boolean `active` attribute (default `true`). Deactivating a user (`PATCH /api/v1/users/:id` with `active: false`) MUST NOT be blocked by the presence of open action items. However, if the user owns any open `ActionItem` records at deactivation time, the API response MUST include a `meta.warnings` array with an entry stating the count of open action items requiring reassignment. Open action items belonging to a deactivated user MUST remain visible and filterable via `GET /api/v1/action-items?owner_active=false`. Deactivated users MUST be prevented from authenticating (JWT issuance rejected at login; existing tokens invalidated via the refresh-token blocklist).
- **FR-033**: Completed postmortems MUST be indexed and retrievable for AI triage context.
- **FR-033a**: Postmortem retrieval for AI triage context MUST use DB-native full-text search. On Postgres, the `postmortems` table MUST maintain a `tsvector` GIN index over `title` and `root_cause`; queries use `tsquery` ranked by `ts_rank`. On SQLite, an FTS5 virtual table (`postmortems_fts`) mirroring `title` and `root_cause` MUST be created via GORM `AutoMigrate` hook. At AI triage time, the alert `title` and `labels` summary are used as the search query; the top-5 results whose score meets the configured threshold are included in the `related_postmortems` field. The score threshold and result limit MUST be configurable (`ai.postmortem_fts_threshold`, default `0.05` for Postgres `ts_rank`; `ai.postmortem_fts_limit`, default `5`). This feature is unavailable in SQLite mode (degraded per FR-000l; job dropped with WARN log).

**API & Platform**

- **FR-034**: All API responses MUST conform to the [JSON:API 1.1](https://jsonapi.org/) specification.
- **FR-034a**: The `/api/v1/` surface MUST follow an additive-only stability policy within the v1 prefix. Permitted non-breaking changes (no version bump required): adding new response fields, adding new optional query parameters, adding new endpoints, relaxing validation constraints. Breaking changes (field removal, type change, required field addition, endpoint removal, pagination contract change) MUST be introduced under a new `/api/v2/` URL prefix. Every breaking change MUST have a corresponding deprecation entry in `CHANGELOG.md` before the old endpoint is removed. No `/api/v2/` surface is planned before the Full Product stage.
- **FR-035**: All API endpoints MUST be documented in an OpenAPI 3.x specification committed to the repository.
- **FR-036**: System MUST implement RBAC with at minimum the built-in roles: `Viewer`, `Responder`, `Incident Commander`, `Admin`.
- **FR-036a**: The four built-in roles MUST be granted the following minimum permissions at PoC/MVP. All permissions are additive; custom roles (Full Product) are defined as combinations of these primitives.

  | Permission | Viewer | Responder | Incident Commander | Admin |
  |---|:---:|:---:|:---:|:---:|
  | `alert:read` | ✓ | ✓ | ✓ | ✓ |
  | `alert:update` | | ✓ | ✓ | ✓ |
  | `incident:read` | ✓ | ✓ | ✓ | ✓ |
  | `incident:create` | | ✓ | ✓ | ✓ |
  | `incident:update` | | ✓ | ✓ | ✓ |
  | `incident:reopen` | | | ✓ | ✓ |
  | `incident:close` | | ✓ | ✓ | ✓ |
  | `postmortem:read` | ✓ | ✓ | ✓ | ✓ |
  | `postmortem:create` | | | ✓ | ✓ |
  | `postmortem:update` | | | ✓ | ✓ |
  | `postmortem:publish` | | | ✓ | ✓ |
  | `user:read` | | | | ✓ |
  | `user:create` | | | | ✓ |
  | `user:update` | | | | ✓ |
  | `user:delete` | | | | ✓ |
  | `admin:*` | | | | ✓ |

  A `403 Forbidden` JSON:API error MUST be returned whenever a caller lacks the required permission. OIDC custom roles and finer-grained permissions are deferred to the Full Product stage.

- **FR-037**: System MUST support OIDC-based SSO for enterprise deployments.
- **FR-038**: All API mutations MUST be audit-logged with actor identity, timestamp, and before/after state.
- **FR-038a**: Audit events MUST be stored as an append-only `AuditLog` GORM model in the same database as application data (both SQLite and Postgres drivers via FR-000k). The `AuditLog` table MUST NEVER be updated or deleted by application code. Before/after state MUST be stored as JSON columns (GORM `serializer:"json"` tag). Each record MUST capture: `id`, `organization_id`, `actor_user_id`, `action` (HTTP method + route pattern, e.g. `PATCH /api/v1/incidents/:id`), `resource_type`, `resource_id`, `before_state` (JSON), `after_state` (JSON), `ip_address`, `created_at`. Read-only query endpoints (`GET /api/v1/audit-logs`) require `admin:*` permission.
- **FR-039**: All list endpoints (`GET /api/v1/:resource`) MUST support cursor-based pagination via `page[cursor]` and `page[size]` query parameters (JSON:API pagination object in response). Default page size: 50; maximum: 200. The cursor MUST be an opaque base64url-encoded string internally encoding the keyset values `(last_seen_id, last_seen_created_at)` of the final record in the previous page. Callers MUST treat cursor values as opaque; the spec explicitly forbids constructing or parsing cursors. Cursors are valid for 24 hours from issuance; an expired or malformed cursor MUST return `400 Bad Request` with a JSON:API error indicating the cursor is invalid. The response `links.next` and `links.prev` fields MUST be populated with pre-constructed URLs containing the appropriate cursor values.

### Key Entities

All entities carry an `organization_id UUID NULL` column from the first migration as a multi-tenancy schema hook. Enforcement is added in the MVP stage when org isolation is implemented.

- **Alert**: An event ingested from an external source. Attributes: `id`, `organization_id`, `source`, `fingerprint`, `title`, `labels`, `severity_hint` (raw webhook value, immutable), `severity` (AI-assigned SEV1–SEV4, null until triage), `triage_status`, `ai_summary`, `probable_cause`, `suggested_actions`, `confidence_score`, `deduplicated`, `received_at`, `resolved_at`.
- **Incident**: A declared production issue. Attributes: `id`, `organization_id`, `status`, `severity`, `title`, `summary`, `commander_user_id`, `declared_at`, `acknowledged_at`, `resolved_at`, `impacted_components[]`, `impacted_slos[]`, `linked_alerts[]`.
- **TimelineEntry**: An immutable event in an incident's history. Attributes: `id`, `organization_id`, `incident_id`, `entry_type` (state_change | ai_hypothesis | comment | page | action | dedup_merge), `actor`, `content`, `created_at`.
- **OnCallSchedule**: A rotation-based schedule for a team. Attributes: `id`, `organization_id`, `team_id`, `timezone` (IANA timezone string, e.g. `"UTC"`), `rotation_type`, `layers[]`, `overrides[]`.
- **EscalationPolicy**: Ordered tiers mapping timeout + schedule to notification targets. Attributes: `id`, `organization_id`, `team_id`, `tiers[]`.
- **Component**: A logical service or system for the status page. Attributes: `id`, `organization_id`, `name`, `status`, `group_id`, `slo_id`.
- **StatusPage**: Top-level status page configuration. Attributes: `id`, `organization_id`, `title`, `domain`, `visibility`, `components[]`, `incidents[]`.
- **SLO**: A service level objective. Attributes: `id`, `organization_id`, `name`, `target_percentage`, `window_days`, `sli_query`, `error_budget_remaining`, `status`.
- **Postmortem**: A post-incident review. Attributes: `id`, `organization_id`, `incident_id`, `status` (draft | in_review | published), `root_cause`, `contributing_factors`, `action_items[]`, `published_at`.
- **ActionItem**: A follow-up task from a postmortem. Attributes: `id`, `organization_id`, `postmortem_id`, `description`, `owner_user_id`, `due_date`, `status`.
- **User**: A platform user. Attributes: `id`, `email`, `name`, `active` (boolean, default `true`), `roles[]`, `notification_channels[]`, `oidc_sub`. *(Authentication: stateless JWT — short-lived access token (15 min) + long-lived refresh token with blocklist for revocation. A default seed admin user is created on first boot. Deactivated users (`active=false`) cannot authenticate and have existing tokens invalidated.)*
- **WebhookSource**: A configured inbound webhook. Attributes: `id`, `organization_id`, `name`, `source_type`, `endpoint_path`, `hmac_secret`, `field_mapping`, `rate_limit_rps`, `max_body_bytes`.
- **AuditLog**: An append-only record of all API mutation events. Attributes: `id`, `organization_id`, `actor_user_id`, `action`, `resource_type`, `resource_id`, `before_state` (JSON), `after_state` (JSON), `ip_address`, `created_at`. Never updated or deleted by application code.
- **Organization**: A tenant entity (used from MVP onwards). Attributes: `id`, `name`, `slug`, `created_at`. *(Schema present from Foundation; enforcement active from MVP.)*

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-000**: A developer with only **Go and Node.js** installed (no Docker, no Postgres) can clone the repo and reach a passing `GET /api/v1/health` within **5 minutes** by running `make dev` — the server uses SQLite by default. A developer who additionally has Docker can reach the same result using `docker compose up`.
- **SC-001**: `POST /api/v1/webhooks/grafana` responds within **200 ms** at P99 under 500 concurrent webhook deliveries.
- **SC-002**: AI triage completes and updates the alert record within **30 seconds** of webhook receipt under normal AI provider conditions.
- **SC-003**: Incident declaration (manual or AI-triggered) reflects in the UI within **2 seconds** of the API call.
- **SC-004**: The public status page achieves < **500 ms TTFB** when no active incidents are being written (served from application-level cache; CDN is an optional operator-layer concern).
- **SC-005**: End-to-end flow (alert ingested → triaged → incident declared → on-call paged) completes within **2 minutes** under normal conditions.
- **SC-006**: The platform sustains **1 000 alerts/minute** ingest throughput without dropping messages (queue depth ≤ worker capacity).
- **SC-007**: API test suite achieves ≥ **80% statement coverage** across all packages; auth/RBAC paths reach **100%**.
- **SC-008**: All core spec-driven user stories (US0–US7) are independently deployable and demonstrable by the end of the MVP stage.
- **SC-009**: A new engineer can run the full local development stack with a **single command** (`make dev` for zero-infrastructure SQLite mode, or `docker compose up` for Postgres mode).
- **SC-010**: Mean time to detect (MTTD) for a synthetic SEV1 test alert (webhook → incident → page) is measurably shorter than a comparable manual process — target ≤ 90 seconds end-to-end.
- **SC-011**: Zero P0 security findings (secrets in code, unauthenticated mutations, HMAC bypass) at any stage of delivery.
- **SC-012**: Every public API endpoint has a corresponding OpenAPI 3.x operation document before the endpoint ships.
