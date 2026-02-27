# Autopsy Constitution

## Core Principles

### I. Code Quality (NON-NEGOTIABLE)

Write idiomatic Go, not textbook-pattern Go. Prefer the constructs and conventions the Go community actually uses — `errors.As`/`errors.Is` over type assertions, table-driven tests, `context.Context` propagation, small focused interfaces, and explicit error returns over panics. Avoid ceremonial abstractions (factories, managers, registries) unless the problem genuinely demands them. Code must pass `go vet`, `staticcheck`, and `golangci-lint` with no suppressions before merge. All exported symbols require doc comments. Cyclomatic complexity per function must stay ≤ 15; refactor before adding complexity.

### II. Testing Standards (NON-NEGOTIABLE)

TDD is mandatory: write tests first, get approval, watch them fail, then implement. Red-Green-Refactor is the only accepted cycle. Coverage gate: ≥ 80% statement coverage on all packages; critical paths (auth, data integrity, error handling) must reach 100%. Use the standard `testing` package as the foundation; add `testify/assert` only where it meaningfully reduces noise. Every public function must have a table-driven unit test. Integration tests live in `_test` packages and must be runnable via `go test ./... -tags integration`. Fuzz targets are expected for any function that parses external input. No test may depend on execution order, wall-clock time, or external network calls without a clearly labelled build tag.

### III. Observability First

Structured, leveled logging is mandatory from day one — use `log/slog` (stdlib) with JSON output in production and human-readable text in development mode. Every service operation must emit a trace span (OpenTelemetry SDK); every significant business event must emit a metric counter or histogram. Log, metric, and trace field names follow the OpenTelemetry semantic conventions. `DEBUG` logs must be safe to enable in production without leaking PII or secrets. Dashboards and alert definitions live in the repository alongside the code they observe.

### IV. Borrow Before Build

Before writing a new package, check whether the problem is solved by the Go standard library, a well-maintained OSS library, or an AI-assisted code generation approach. The evaluation order is: **stdlib → established OSS → AI-generated utility code → build from scratch**. Every third-party dependency must be justified in the PR description. Dependencies must not be vendored without a documented reason; use Go modules with a checked-in `go.sum`. Prefer libraries with > 1 k GitHub stars, active maintenance, and permissive licenses (MIT, Apache-2.0, BSD). Pin dependency versions; never use `latest` in production paths.

### V. AI-Augmented Development

AI assistance (GitHub Copilot, Claude, GPT-class models) is a first-class development tool across the entire lifecycle — spec, plan, code, test, review, and documentation. AI-generated code carries the same quality bar as human-written code: it must pass all linting, test, and observability requirements before merge. Prompts used to generate non-trivial components should be preserved in `/docs/ai-prompts/` so the intent is reproducible. AI is especially encouraged for: boilerplate reduction, fuzz corpus generation, documentation drafting, and exploratory spike code.

### VI. User Experience Consistency

All CLI interfaces emit machine-readable JSON when `--output json` is specified, plain text by default, and structured errors to `stderr` with a non-zero exit code on failure. HTTP APIs follow RESTful conventions with OpenAPI 3.x specifications checked into the repository. Error messages are human-actionable: they state what went wrong, why, and what the user can do next. No breaking changes to public interfaces without a deprecation notice in at least one minor version and an entry in `CHANGELOG.md`.

### VII. Performance Requirements

Establish baseline benchmarks (`go test -bench`) for every hot path before optimisation. P99 latency targets must be defined per endpoint/command in the spec before implementation — performance is a feature, not an afterthought. Profile before optimising; instrument with pprof endpoints in all long-running processes. Allocations in critical loops must be justified via benchmarks. Binary size and startup time are metrics for CLI tools: measure them in CI and fail the build if a PR increases either by > 10% without documented justification.

## Engineering Standards

### Open Source Posture

OSS is the default. All dependencies must be compatible with the project's license. Contributions back to upstream projects are encouraged when autopsy fixes or improves library behaviour. New OSS dependencies require a brief evaluation note in the PR (license, maintenance health, security posture). The project itself is MIT-licensed; all contributors must sign the CLA.

### Documentation Policy

Documentation is not optional — it is a deliverable. Every feature ships with:

- **API/interface docs**: godoc-style doc comments for all exported symbols, including examples where non-obvious.
- **Architecture decision records (ADRs)**: stored in `/docs/adr/` using the MADR format for any decision that has cross-component impact or is not immediately obvious from the code.
- **Runbooks**: operational runbooks in `/docs/runbooks/` covering startup, shutdown, incident triage, and common failure modes.
- **CHANGELOG**: every user-facing change logged under [Semantic Versioning](https://semver.org/) categories (Added / Changed / Deprecated / Removed / Fixed / Security).

Documentation must be accurate — stale docs are treated as bugs. When code changes, the accompanying doc change is part of the same PR, not a follow-up.

### Security and Compliance

No secrets in source code, ever. Use environment variables or a secrets manager. All inputs from external sources are validated and sanitised before use. Dependencies are scanned for known CVEs in CI via `govulncheck`; critical or high severity findings block merge. Authentication and authorisation logic must be reviewed by a second engineer regardless of PR size.

## Development Workflow

- **Branch strategy**: trunk-based development; feature branches are short-lived (< 3 days) and merged via PR.
- **Quality gates**: CI must pass (`build`, `lint`, `test -race`, `govulncheck`) before any merge.
- **PR size**: aim for < 400 lines of non-generated diff; large PRs must include a decomposition plan.
- **Code review**: every PR requires at least one approving review; security-sensitive changes require two.
- **Release cadence**: semantic versioning; patch releases as needed, minor on a rough biweekly cadence.

## Governance

This constitution supersedes all other development practices. Any amendment requires: (1) a written proposal with rationale, (2) team consensus, (3) an updated version entry below. Complexity must always be justified against this constitution. All PR reviews must verify compliance.

**Version**: 1.0.0 | **Ratified**: 2026-02-25 | **Last Amended**: 2026-02-25
