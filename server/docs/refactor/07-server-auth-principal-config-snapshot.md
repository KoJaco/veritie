# 07 Server Auth Principal Config Snapshot

## Objective
Implement request authentication and principal resolution that returns a deterministic app-config snapshot (resolved server-side via config ID) for downstream pipeline execution.

## Why This Branch Exists
Jobs, provider routing, and policy decisions depend on a stable authenticated principal context. This branch formalizes that contract before API and runner branches expand.

## In Scope
- `server/internal/app/auth`
- HTTP auth middleware wiring under `server/internal/transport/http`
- DB-backed app/principal lookup and config snapshot materialization via `config_id` indirection
- Unauthorized/forbidden error mapping for HTTP responses
- SSE endpoint auth behavior (same credential parsing and principal resolution model as HTTP API routes)

## Out of Scope
- Jobs endpoint request/response contracts (branch 11)
- Client-provided config payload ingestion
- Token issuance or external identity-provider provisioning
- Any websocket upgrade/session-auth path

## Split Decision
No further branch split required. Auth service and middleware should land together to avoid partial security posture and repeated integration churn.

## Implementation Plan
1. Define principal/auth domain model in `internal/app/auth`:
   - principal identity fields (app/account IDs)
   - resolved policy/config snapshot payload loaded by server-side `config_id`
   - auth error taxonomy (unauthenticated, unauthorized, disabled app, malformed key)
   - explicit model boundary that excludes client-passed config objects
2. Implement auth service:
   - parse credential input from headers
   - lookup app/principal via repository
   - resolve `config_id` -> config snapshot in the same auth resolution flow
   - validate active status and required config shape
   - return immutable request-scoped principal snapshot
3. Implement HTTP middleware integration:
   - extract auth header/api key
   - call auth service
   - inject principal into request context
   - short-circuit with typed error responses on failure
4. Implement SSE middleware/guard reuse:
   - use the same credential parsing and principal resolution on SSE endpoints
   - no websocket-specific upgrade/session logic
5. Add auth-path latency controls:
   - define auth resolution SLO budget (for example, p95 target)
   - add request-scoped timing metrics for auth parse, principal lookup, and config resolution steps
   - optionally introduce short-lived server-side cache for app/config snapshots keyed by app ID/config ID with strict TTL and invalidation strategy
6. Add security best-practice controls:
   - constant-time comparison where applicable for API key verification
   - fail-closed behavior on malformed/missing credentials
   - no credential or secret material in logs
   - strict header parsing and normalization rules
7. Add context access helpers for handlers/services to consume principal safely.
8. Add test coverage:
   - unit tests for auth service decision paths
   - middleware tests for header parsing, context injection, and status-code mapping
   - SSE auth-path tests to confirm parity with HTTP route behavior
   - latency-focused tests/benchmarks for auth hot path

## Deliverables
- Auth service implementation in `internal/app/auth`
- HTTP middleware enforcing auth on protected routes
- SSE auth guard integration using shared middleware/service logic
- Principal config snapshot contract available via request context
- Comprehensive auth unit/middleware tests
- Auth-path timing metrics and optional bounded cache design docs/implementation

## Dependencies
- 05 Server Foundation Config Obs Runtime
- 06 Server DB Postgres Core

## Risks and Mitigations
- Risk: auth context shape changes later, causing handler churn.
- Mitigation: define narrow stable principal interface and keep raw DB models private.
- Risk: accidental permissive behavior on malformed credentials.
- Mitigation: fail closed by default and test malformed/empty header cases explicitly.
- Risk: hidden coupling to legacy schma auth assumptions.
- Mitigation: document intentional behavior differences and enforce with tests.
- Risk: config resolution introduces latency regressions.
- Mitigation: instrument auth stages, set p95 budget, and use bounded TTL caching only if needed.
- Risk: mixed HTTP/SSE auth behavior divergence.
- Mitigation: enforce one shared auth service and shared credential parser across both transports.

## Verification
- Unit tests across success/failure branches in auth service.
- Middleware tests confirming:
  - missing credentials -> 401
  - invalid credentials -> 401
  - valid credentials + inactive/unauthorized principal -> 403
  - valid principal -> request context populated
- Client-supplied config payload on protected routes is ignored/rejected by contract.
- SSE endpoint smoke test uses same auth resolution path and status behavior.
- Benchmark or timing assertions validate auth path remains within defined latency target.

## Acceptance Gates
- Protected routes reject missing/invalid credentials with deterministic status codes.
- Valid credentials resolve principal and app-config snapshot deterministically via server-side `config_id`.
- No endpoint accepts client-supplied config object as authoritative auth/runtime config.
- Auth context is accessible from handlers/services without type assertions leaking transport details.
- HTTP and SSE use shared auth parsing/resolution behavior; no websocket upgrade logic remains.
- Auth-path latency is instrumented and meets agreed budget in test/staging validation.
- Unit and middleware auth tests pass in CI.
