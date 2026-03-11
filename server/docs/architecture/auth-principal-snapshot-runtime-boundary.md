# Architecture — Auth Principal Snapshot Runtime Boundary

## Purpose

Define the server auth boundary for branch 07: credential parsing, API-key-backed principal resolution, immutable request principal snapshot injection, and shared behavior across HTTP and SSE routes.

## Scope

Covers:
- Credential extraction from `Authorization: Bearer` and `X-API-Key`.
- DB-backed resolution from `api_keys` + `apps` active refs.
- Request context principal snapshot model.
- Shared auth middleware behavior for jobs and SSE stream routes.
- Auth-path latency instrumentation and bounded TTL cache behavior.

Does not cover:
- Token issuance flows or external identity provider provisioning.
- WebSocket session auth paths.
- Jobs payload contracts or worker stage orchestration.

## Components

- Auth parser and service (`internal/app/auth`).
- Postgres-backed API key resolver (`api_keys` joined to app active refs/runtime JSON).
- In-memory auth cache (short TTL, credential-hash keyed).
- Transport middleware that enforces auth and injects principal into context.
- Protected HTTP handlers for jobs and SSE stream endpoints.

## Boundaries

- Runtime snapshot authority is server-side app active refs, not client config payloads.
- Credential parsing and principal resolution path is shared across HTTP and SSE.
- Auth failures are deterministic (`401` unauthenticated/malformed/invalid/revoked/expired; `403` forbidden policy checks).
- Cache is an optimization only; correctness must not depend on cache hits.

## Invariants

- Protected endpoints should fail closed on missing/malformed credential input.
- Principal snapshot should be immutable per request after auth resolution.
- Sensitive credential material should never be written to logs.
- API key validation should use constant-time hash comparison where applicable.
- SSE auth behavior should remain parity-aligned with HTTP route auth behavior.

## Non-Goals

- Introducing a `config_id` schema model in branch 07.
- Performing deep authorization policy modeling beyond principal resolution and simple transport-level forbidden mapping.
- Persisting auth cache state across process restarts.

