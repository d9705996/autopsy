---
name: autopsy-pr-guard
description: Use when making code changes in the autopsy repo to prevent PR failures by enforcing domain modeling checks, migration checks, UI/API parity checks, and a deterministic pre-PR validation sequence.
---

# Autopsy PR Guard

Use this skill for any feature/fix PR in this repository.

## Goal

Reduce avoidable PR failures (schema drift, API/UI mismatch, missing tests, accidental artifacts).

## Workflow

1. **Model-first check**
   - If behavior is tied to a business object (e.g., service availability), add/verify explicit domain entities in `internal/app/models.go`.
   - Ensure JSON fields needed by UI/API are present and named consistently.

2. **Persistence parity check**
   - Update both memory and SQL stores when models change.
   - For SQL: update `CREATE TABLE` and add backward-compatible migration helpers (`ensure*Column`) for existing DBs.
   - Never rely on only fresh schema for new fields.

3. **API/UI contract check**
   - Confirm API response shape changes are reflected in `web/app.js` and relevant HTML.
   - If adding query params, document defaults and validation behavior in handler code.

4. **Test coverage gate**
   - Add or extend API tests for new response fields and core calculation logic.
   - Include at least one deterministic test that validates numeric behavior where relevant.

5. **Pre-PR validation (required)**
   - Run `scripts/pre_pr_check.sh` from this skill.
   - If a command fails, fix code and rerun until green.

## Commands

Run from repo root:

```bash
bash .codex/skills/autopsy-pr-guard/scripts/pre_pr_check.sh
```

## Output checklist before commit

- [ ] Domain model updated for new business concept
- [ ] Memory + SQL store parity complete
- [ ] Backward-compatible SQL migration included
- [ ] API and UI contract aligned
- [ ] Tests added/updated and passing
- [ ] No accidental local artifacts committed
