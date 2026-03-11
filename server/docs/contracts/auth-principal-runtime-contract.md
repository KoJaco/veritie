# Contract: Auth Principal Runtime Contract

## Purpose

Define the internal runtime contract for request authentication and principal snapshot resolution used by protected HTTP and SSE routes.

## Scope

Included:
- Accepted credential header formats.
- Auth error/status mapping contract.
- Principal snapshot context shape and availability.
- API-key-backed app runtime snapshot resolution model.
- Cache and latency instrumentation expectations.

Out of scope:
- Public jobs request/response payload schema.
- Provider authorization models.
- Token minting, refresh, or OAuth flows.

## Versioning

- **Current version:** v1
- **Compatibility:** Backward compatible for branch-07 auth consumers
- **Change policy:** Major bump for credential/header semantics or status-code mapping changes; minor bump for additive metadata fields.

## Definitions

- **Principal snapshot:** Immutable request-context identity and runtime reference bundle (app/account IDs + active refs + runtime JSON).
- **Credential source:** Either Bearer token in `Authorization` or raw value in `X-API-Key`.
- **Auth resolver:** Server-side lookup of API key metadata joined with app active refs/runtime configuration.

## Contract Shape (Conceptual)

### Required fields

- Credential input from exactly one header source:
  - `Authorization: Bearer <credential>`
  - or `X-API-Key: <credential>`
- Principal snapshot context fields:
  - `app_id`
  - `account_id`
  - `schema_id`
  - `active_schema_version_id`
  - `active_toolset_version_id`
- Auth result state:
  - authenticated principal in context, or deterministic auth error response.

### Optional fields

- Runtime JSON snapshot payloads:
  - `processing_config`
  - `runtime_behavior`
  - `llm_config`
- Auth key metadata:
  - `key_id`
  - `key_prefix`
- Auth cache hit/miss telemetry and stage latency metrics.

## Invariants (Must Always Hold)

- Missing/malformed/invalid/revoked/expired credentials should map to `401`.
- Forbidden policy decisions should map to `403`.
- Principal snapshot should be injected once and consumed from context by downstream handlers.
- HTTP and SSE should use the same credential parsing and auth resolution behavior.
- Client-supplied runtime config payloads should not override server-resolved principal snapshot values.

## Error Handling

- Multiple credential sources in one request are rejected as malformed (`401`).
- Non-Bearer `Authorization` schemes are rejected as malformed (`401`).
- Resolver miss or hash mismatch is rejected as unauthenticated (`401`).
- Revoked or expired API keys are rejected as unauthenticated (`401`).
- Explicit authorization rejection from policy hooks returns forbidden (`403`).
- Unexpected resolver/internal failures return internal auth error (`500`).

## Examples

### Minimal valid example

```json
{
  "request_headers": {
    "Authorization": "Bearer vt_live_abc123"
  },
  "principal_snapshot": {
    "app_id": "7fa4f5f2-c302-42a2-b727-687fbf1cf2b9",
    "account_id": "9f220ccb-0e4f-44f9-b7a3-c402f56d5607",
    "schema_id": "8a6caeb6-9f12-4e74-b153-a2795f353c07",
    "active_schema_version_id": "11adce71-c98d-4dd8-9fc8-bdc8b4aecc6b",
    "active_toolset_version_id": "3f9109be-a46e-446d-a46d-4bd51d6b0539"
  },
  "response_status": 200
}
```

### Invalid example

```json
{
  "request_headers": {
    "Authorization": "Bearer one",
    "X-API-Key": "two"
  }
}
```

Expected handling:
- Reject as malformed credential source ambiguity.
- Return `401` with auth error code `malformed_credential`.

### Operational notes

- Branch 07 uses app active refs runtime model, not `config_id` indirection.
- Cache TTL should remain short-lived (for example, 30-120s) and bounded in-memory.
- Cache misses must still produce correct behavior; cache is non-authoritative.

### References

- Related ADRs: `server/docs/adr/ADR-0004-auth-principal-snapshot-and-credential-transport.md`, `server/docs/adr/ADR-0001-server-pipeline-runtime-boundary.md`
- Related architecture: `server/docs/architecture/auth-principal-snapshot-runtime-boundary.md`
- Related decision notes: `server/docs/decisions/auth-branch-07-completion-and-operational-defaults.md`
- Related refactor doc: `server/docs/refactor/07-server-auth-principal-config-snapshot.md`
- Issue/PR: #
