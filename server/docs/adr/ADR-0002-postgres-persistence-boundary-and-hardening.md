# ADR-0002: Postgres Persistence Boundary and Hardening

## Status

Accepted

## Date

2026-03-11

## Context

Veritie branch 06 established foundational Postgres persistence for jobs and events. The branch introduced runtime snapshot writes, ordered event replay reads, migration conventions, and schema hardening using enum-backed finite domains.

This ADR captures the irreversible persistence decisions so later branches do not regress into free-text state fields, ambiguous runtime sourcing, or non-deterministic event replay behavior.

## Decision

1. Use Postgres as canonical durable store for jobs/events in the refactor baseline.
2. Resolve runtime execution references from app-owned active schema/toolset on job create/rerun and persist a job snapshot.
3. Enforce finite domains with strict Postgres enums for bounded state/type columns.
4. Preserve deterministic event replay ordering by `(created_at, id)`.
5. Standardize schema evolution through SQL migrations under `internal/infra/db/postgres/migrations` and Atlas checks.

## Alternatives Considered

- **Free-text state/type columns** — rejected due to invalid-state drift risk and weaker fail-fast guarantees.
- **Client-provided runtime config as job authority** — rejected due to reproducibility and security concerns.
- **Unordered or timestamp-only event replay** — rejected due to instability under tie conditions.
- **Migrationless schema drift/manual DB edits** — rejected due to CI reproducibility and auditability loss.

## Consequences

- **Pros**
- Stronger data integrity and deterministic replay semantics.
- Clear server-side authority for runtime snapshot sourcing.
- Better CI reliability for schema/migration consistency.
- Shared, testable transaction boundary patterns for repositories.
- **Cons**
- Enum changes require explicit migrations and coordination.
- Early status vocabulary differs from later pipeline naming (`succeeded` vs `completed`).
- Additional upfront discipline for migration authoring and replay checks.
- **Follow-ups / TODOs** (optional)
- Track cross-branch status vocabulary alignment (`succeeded/cancelled` vs `completed/partial_success`) as an explicit schema evolution step.
- Reassess enum sets when optional stage semantics are finalized in later branches.

## References

- Related docs/contracts: `server/docs/contracts/postgres-persistence-runtime-contract.md`, `server/docs/architecture/postgres-persistence-boundary.md`
- Related runtime ADR: `server/docs/adr/ADR-0001-server-pipeline-runtime-boundary.md`
- Related decision note: `server/docs/decisions/postgres-branch-06-completion-and-latency-defaults.md`
- Related refactor docs: `server/docs/refactor/06-server-db-postgres-core.md`, `server/docs/refactor/ground-truth.md`
- Issue: #
- PR: #
