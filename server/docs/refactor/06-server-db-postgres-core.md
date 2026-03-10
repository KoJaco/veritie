# 06 Server DB Postgres Core

## Objective

Implement production-ready Postgres adapters for jobs/events persistence and establish migration conventions that are safe for local dev and CI.

## Why This Branch Exists

All downstream server branches rely on stable persistence primitives. Defining pool, transaction boundaries, and repository contracts early prevents data-access fragmentation.

## In Scope

- `server/internal/infra/db/postgres/pool.go`
- `server/internal/infra/db/postgres/tx.go`
- `server/internal/infra/db/postgres/jobs_repo.go`
- `server/internal/infra/db/postgres/events_repo.go`
- Migration directory bootstrap: `server/internal/infra/db/postgres/migrations/`
- Atlas alignment with `server/atlas.hcl` and schema file
- Canonical schema baseline with `jobs` (renamed from `batch_jobs`) and consolidated ownership/extraction/tool/evidence/runtime tables
- App-as-execution-bundle enforcement (`apps` pinned to one active schema version + one active toolset version)
- DB hardening policy: finite domain fields use strict enums (fail-fast), not free-text strings

## Out of Scope

- Auth middleware behavior
- Job orchestration/state-machine logic
- Provider integrations (STT/LLM)

## Split Decision

No further branch split required. Keeping DB core in one branch preserves velocity and keeps related persistence decisions atomic. Splitting pool/tx from repos would add handoff friction without reducing meaningful risk.

## Implementation Plan

1. Implement `pool.go`:
    - config-backed pgx pool construction
    - connection health check utility
    - shutdown/close semantics
2. Implement `tx.go`:
    - transaction helper with context propagation
    - rollback-on-error and panic safety
    - optional retry policy hook for serialization errors (interface-ready, minimal initial behavior)
3. Implement `jobs_repo.go`:
    - create job record
    - resolve app runtime bundle (active schema/toolset + defaults) before create/rerun writes
    - fetch by ID
    - update status and lifecycle timestamps
    - list/query primitives required by upcoming HTTP contract branch
4. Implement `events_repo.go`:
    - append immutable job events
    - list events by job ID in deterministic order
    - optional cursor/time filtering stub for SSE replay compatibility
5. Add migration directory bootstrap:
    - ensure `migrations/` exists
    - add initial migration workflow notes in refactor docs or inline package README
6. Verify Atlas consistency:
    - schema path and migration dir align with `.github/workflows/server-db-atlas-checks.yml`

## Deliverables

- Working Postgres pool and transaction helpers
- Concrete jobs/events repository implementations
- Migration directory present and wired to Atlas conventions
- Basic DB usage docs for local and CI checks

## Dependencies

- 04 Server DB Atlas Checks
- 05 Server Foundation Config Obs Runtime

## Risks and Mitigations

- Risk: schema and repository fields diverge during iterative refactor.
- Mitigation: add integration tests that exercise insert/select/update against actual schema.
- Risk: hidden transaction boundary bugs.
- Mitigation: centralize transaction helper and enforce repository methods use explicit tx/non-tx variants.
- Risk: migration conventions unclear before additional tables are introduced.
- Mitigation: codify naming/order conventions now and enforce via PR checklist.

## Verification

- Unit tests for transaction helper behavior.
- Integration tests for job CRUD and event append/list paths.
- Local run of Atlas dry-run checks mirroring CI workflow.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates

- Repositories compile and satisfy app-layer interfaces used by branch 08+.
- DB integration tests pass for job CRUD and ordered event retrieval.
- Atlas checks pass with current schema and migration directory layout.
- Transaction helper guarantees rollback on error and safe cleanup.
