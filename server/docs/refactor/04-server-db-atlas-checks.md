# 04 Server DB Atlas Checks

## Objective

Add database-focused CI checks that validate Atlas schema integrity and migration replay safety for server DB changes.

## Why This Branch Exists

DB drift is a high-cost failure mode in refactors. This branch enforces schema and migration sanity before runtime branches depend on them.

## In Scope

- GitHub Actions workflow for Atlas checks
- Ephemeral Postgres service in CI
- Schema apply dry-run against current schema file
- Conditional migration replay dry-run

## Out of Scope

- SQL migration authoring
- Repository implementation in Go (`jobs_repo`, `events_repo`)

## Implementation Plan

1. Create/update `.github/workflows/server-db-atlas-checks.yml`.
2. Configure triggers for changes under:
    - `server/atlas.hcl`
    - `server/internal/infra/db/postgres/**`
3. Add Postgres service container.
4. Install Atlas via `ariga/setup-atlas`.
5. Add checks:
    - assert schema file exists
    - `atlas schema apply --dry-run`
    - if migration SQL files exist: `atlas migrate apply --dry-run`
6. Ensure missing migration directory/files are handled gracefully (skip with explicit message).

## Deliverables

- `.github/workflows/server-db-atlas-checks.yml`

## Dependencies

- 01 Project Setup
- 03 Server Go Baseline CI

## Risks and Mitigations

- Risk: migrations directory not present in early phase.
- Mitigation: conditional replay step instead of hard failure.
- Risk: Atlas CLI version changes affect flags.
- Mitigation: pin setup action and review during docs/CI refresh branch.

## Verification

- Validate workflow file syntax.
- Confirm trigger paths include both schema and migrations.

## Acceptance Gates

- Workflow runs for Atlas/db path changes.
- Schema file presence is enforced.
- Schema apply dry-run passes in CI.
- Migration replay dry-run executes when SQL migrations exist, otherwise skips explicitly.

## Completion Status (2026-03-11)

Branch 04 is complete for scoped deliverables.

Implemented evidence:
- Atlas workflow with Postgres service container: `.github/workflows/server-db-atlas-checks.yml`
- Trigger coverage includes:
  - `server/atlas.hcl`
  - `server/internal/infra/db/postgres/**`
  - workflow file itself
- Checks implemented:
  - schema file existence assertion
  - `atlas schema apply --dry-run`
  - conditional `atlas migrate apply --dry-run` when SQL migrations exist
- Atlas environment config present: `server/atlas.hcl`

Verification snapshot:
- Workflow implements explicit skip messaging when migration SQL files are absent.
- Trigger paths and dry-run checks align to branch objective for schema/migration drift detection.

Related documentation:
- `server/docs/architecture/server-foundation-runtime-and-ci-guardrails.md`
- `server/docs/contracts/server-foundation-runtime-and-ci-contract.md`
- `server/docs/adr/ADR-0003-server-foundation-runtime-and-ci-guardrails.md`
- `server/docs/decisions/refactor-02-05-foundation-completion-summary.md`
