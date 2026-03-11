# Decision Note: Postgres Branch 06 Completion and Latency Defaults

## Date

2026-03-11

## Summary

Branch 06 DB core deliverables are complete and provide the baseline persistence layer for the refactor. The implementation includes pool/tx primitives, jobs/events repositories, migration + Atlas checks, and enum/FK hardening with deterministic event ordering.

## Decision

Treat branch 06 as complete for scoped deliverables and adopt the following DB operational defaults:

- Keep runtime snapshot resolution server-side at job create/rerun.
- Keep jobs/events ordering deterministic by `(created_at, id)`.
- Keep finite domains enum-backed and fail-fast.
- Keep migration replay checks in CI via Atlas workflow.
- Keep status+event transitions atomic when they represent the same boundary.

## Rationale

- Enables stable foundations for branch 08+ lifecycle/orchestration work.
- Reduces invalid-state drift and cross-tenant integrity risk.
- Improves replay predictability for SSE and reconnect semantics.
- Keeps schema evolution auditable and reproducible in CI.

## Impact

- Documentation now canonizes DB repository and schema constraints as part of Veritie server architecture.
- Later branches should build on these DB invariants rather than re-deriving persistence behavior.
- Temporary vocabulary mismatch (`succeeded/cancelled` vs `completed/partial_success`) is tracked as explicit follow-up, not a blocker to branch-06 completeness.

## Follow-ups

- [ ] Add a dedicated schema/contract alignment task for status vocabulary convergence across DB and pipeline docs.
- [ ] Add branch-level note when status enum evolution is implemented.
- [ ] Keep integration tests exercising runtime snapshot create/rerun and ordered cursor replay paths.

## References

- Related ADR/Contracts: `server/docs/adr/ADR-0002-postgres-persistence-boundary-and-hardening.md`, `server/docs/contracts/postgres-persistence-runtime-contract.md`
- Related architecture: `server/docs/architecture/postgres-persistence-boundary.md`
- Related runtime docs: `server/docs/architecture/server-pipeline-core-flow.md`, `server/docs/contracts/server-pipeline-runtime-contract.md`
- Related refactor docs: `server/docs/refactor/06-server-db-postgres-core.md`
- Issue: #
- PR: #
