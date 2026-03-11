# ADR-0001: Server Pipeline Runtime Boundary

## Status

Accepted

## Date

2026-03-11

## Context

Veritie server flow requires a stable execution contract across job creation, upload, worker processing, persistence checkpoints, and SSE progress streaming. Without explicit runtime boundaries, implementations drift toward repeated mutable config reads, SSE/persistence coupling, and inconsistent stage ordering.

The refactor plan and ground-truth docs establish:
- app-centric runtime config
- direct single-part upload for short audio MVP
- SSE for progress
- checkpointed persistence
- sequential extraction then tool suggestion

This ADR records the irreversible runtime-boundary choices so jobs, transport, and SDK consumers align on one lifecycle model.

## Decision

Veritie adopts the following runtime boundary decisions:

1. App is the workflow boundary in MVP; jobs do not accept client runtime config negotiation.
2. Job creation snapshots active app runtime config for deterministic downstream execution.
3. SSE is a progress transport; canonical truth is persisted job/artifact state.
4. Persistence occurs at stable checkpoints, not per SSE event.
5. Tool suggestion runs after extraction in MVP.
6. Optional stages may resolve terminal state as `partial_success` when policy allows.

## Alternatives Considered

- **Client runtime overrides per job** — rejected due to reproducibility and security risk, plus increased validation complexity.
- **SSE as source of truth** — rejected due to reconnect fragility and inability to guarantee durable reconstruction.
- **Per-event durable persistence** — rejected due to write amplification and coupling runtime noise to storage cost/performance.
- **Parallel extraction + tool suggestion in MVP** — rejected due to orchestration complexity, ordering risk, and grounding consistency cost.

## Consequences

- **Pros**
  - Deterministic replayability from job snapshot + durable checkpoints.
  - Clear separation between progress UX (SSE) and durable truth (reads/checkpoints).
  - Lower orchestration complexity for MVP stage sequencing.
  - Better control of DB write volume via bounded checkpoint persistence.
- **Cons**
  - Reduced per-job runtime flexibility for callers.
  - Sequential extraction-to-suggestion increases total wall time in some scenarios.
  - Requires explicit reconnect flow (`job.snapshot`/canonical fetch) in clients.
- **Follow-ups / TODOs** (optional)
  - Define evolution path for multipart upload when short-audio constraints are exceeded.
  - Define evolution path for optional indexing as non-blocking background work.
  - Revisit guidance-level `SHOULD` language and promote critical invariants to stricter gates once implementation stabilizes.

## References

- Related docs/contracts: `server/docs/architecture/server-pipeline-core-flow.md`, `server/docs/contracts/server-pipeline-runtime-contract.md`, `server/docs/decisions/server-pipeline-latency-prioritization.md`
- Related architecture boundary doc: `server/docs/architecture/persistence-and-sse-runtime-boundary.md`
- Related refactor docs: `server/docs/refactor/ground-truth.md`, `server/docs/refactor/07-server-auth-principal-config-snapshot.md`, `server/docs/refactor/08-server-jobs-domain-state-machine.md`, `server/docs/refactor/10-server-worker-runner-orchestration.md`, `server/docs/refactor/12-server-sse-stream-contract.md`
- Issue: #
- PR: #
