# ADR-0004: Auth Principal Snapshot and Credential Transport

## Status

Accepted

## Date

2026-03-11

## Context

Branch 07 introduces server-side auth as a prerequisite for jobs and SSE flows. The current data model already supports app-bound active runtime refs (`active_schema_version_id`, `active_toolset_version_id`, runtime JSON config fields) and API key records (`api_keys`) with revocation/expiration metadata.

The branch needs deterministic auth semantics with shared behavior across HTTP and SSE while preserving fail-closed security posture and reasonable hot-path latency.

## Decision

1. Use app active refs runtime model for principal snapshot resolution (not `config_id` schema indirection).
2. Accept credentials from either `Authorization: Bearer` or `X-API-Key` (exactly one source per request).
3. Use one shared middleware/auth service path for both HTTP and SSE route protection.
4. Return deterministic auth status mapping (`401` unauthenticated/malformed/invalid/revoked/expired, `403` forbidden policy checks).
5. Add bounded in-memory TTL cache for auth resolution optimization; keep cache correctness non-authoritative.

## Alternatives Considered

- **Introduce `config_id` schema indirection in branch 07** — rejected to avoid unnecessary schema churn against existing active-refs model.
- **Require only one credential header format** — rejected; dual-format support improves migration ergonomics while keeping parser strict.
- **Separate SSE auth flow from HTTP middleware** — rejected due to behavior drift risk.
- **No auth cache** — rejected for this branch because targeted bounded cache reduces repeated hot-path lookups with low complexity.

## Consequences

- **Pros**
- Stable server-authoritative principal snapshot for downstream execution.
- Reduced divergence risk by reusing one auth path across transports.
- Clear and testable auth error semantics.
- Better hot-path performance under repeated credential reuse.
- **Cons**
- In-memory cache invalidation is time-based and process-local.
- Dual-header support requires strict ambiguity handling logic.
- Additional auth-path instrumentation and testing overhead.
- **Follow-ups / TODOs** (optional)
- Evaluate cache tuning from real latency telemetry.
- Reassess whether to standardize on one credential header format post-MVP.
- Keep branch docs aligned if auth model evolves beyond active refs.

## References

- Related docs/contracts: `server/docs/contracts/auth-principal-runtime-contract.md`, `server/docs/architecture/auth-principal-snapshot-runtime-boundary.md`
- Related decision note: `server/docs/decisions/auth-branch-07-completion-and-operational-defaults.md`
- Related refactor doc: `server/docs/refactor/07-server-auth-principal-config-snapshot.md`
- Issue: #
- PR: #
