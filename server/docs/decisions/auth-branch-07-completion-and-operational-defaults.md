# Decision Note: Auth Branch 07 Completion and Operational Defaults

## Date

2026-03-11

## Summary

Branch 07 auth/principal snapshot scope is complete using the current app active-refs runtime model. Auth now has typed error semantics, shared HTTP/SSE middleware behavior, request-context principal snapshots, bounded TTL caching, and branch-level test coverage.

## Decision

Treat branch 07 as complete and adopt the following defaults:

- Runtime snapshot source is app active refs/runtime JSON fields.
- Accepted credential headers are `Authorization: Bearer` and `X-API-Key` (one source only).
- Auth path is shared across HTTP and SSE routes.
- Auth failures are deterministic (`401`/`403` mapping).
- Cache is short-lived, in-memory, and non-authoritative for correctness.

## Rationale

- Aligns auth model with existing DB schema and avoids unnecessary migration churn.
- Prevents HTTP/SSE auth divergence by using one middleware/service path.
- Enables predictable client behavior from stable status-code semantics.
- Improves repeated auth hot-path latency with bounded complexity.

## Impact

- Downstream handlers/services can rely on typed principal context instead of ad hoc auth parsing.
- Branch 11/12 API + SSE work inherits stable auth guard behavior.
- Documentation now explicitly supersedes previous `config_id` wording for branch 07.

## Follow-ups

- [ ] Tune auth cache TTL based on observed latency and key-rotation patterns.
- [ ] Add staging auth-path p95 dashboards and alert thresholds if not already present.
- [ ] Revisit credential header standardization post-MVP if one form is preferred.

## References

- Related ADR/Contracts: `server/docs/adr/ADR-0004-auth-principal-snapshot-and-credential-transport.md`, `server/docs/contracts/auth-principal-runtime-contract.md`
- Related architecture: `server/docs/architecture/auth-principal-snapshot-runtime-boundary.md`
- Related refactor doc: `server/docs/refactor/07-server-auth-principal-config-snapshot.md`
- Issue: #
- PR: #
