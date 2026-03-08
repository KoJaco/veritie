# 11 Server HTTP Jobs Contract

## Objective
Implement and lock the authenticated HTTP contract for job creation, retrieval, and rerun using principal-resolved config and deterministic validation rules.

## Why This Branch Exists
The platform contract is exposed primarily through HTTP. This branch formalizes the public API shape and request validation boundaries before SSE and client SDK integration expand.

## In Scope
- `POST /v1/jobs`
- `GET /v1/jobs/{job_id}`
- `POST /v1/jobs/{job_id}/rerun`
- Request/response schema validation and error model
- Idempotency key handling on job creation
- Principal-scoped authorization and tenant isolation checks

## Out of Scope
- SSE endpoint protocol and event streaming details (branch 12)
- Export endpoints
- SDK implementation details (branches 15-17)

## Split Decision
No split required. These three endpoints share auth, validation, and response schema concerns and should be delivered together as one coherent contract branch.

## Implementation Plan
1. Define request/response contracts:
   - create-job payload (audio source, metadata, optional overrides as allowed by policy)
   - job summary/detail response model
   - rerun request model and response linkage fields
   - standardized error envelope
2. Implement routing/handlers under `server/internal/transport/http`:
   - register all three routes
   - apply shared auth middleware (principal + config snapshot)
   - map handler calls to jobs service APIs
3. Implement validation rules:
   - required field and type validation
   - source constraints for audio input modes
   - strict rejection of forbidden client-provided runtime config objects
   - idempotency-key format and dedup semantics for create-job
4. Implement authorization checks:
   - principal/account scoping for job read and rerun operations
   - deny access to cross-account job IDs
5. Implement response/status semantics:
   - deterministic status codes for validation, auth, and domain errors
   - include `rerun_of_job_id` linkage in rerun responses
6. Add contract docs updates:
   - reflect endpoint shapes in `server/docs/contracts/*`
   - include examples for success and failure responses
7. Add test coverage:
   - handler tests for happy and failure cases
   - contract tests for payload shape/status code guarantees
   - idempotency behavior tests for duplicate create requests

## Deliverables
- HTTP handlers and route wiring for jobs contract
- Stable request/response models and validation logic
- Idempotency support for `POST /v1/jobs`
- Contract documentation updates for API consumers
- Contract and handler test suite

## Dependencies
- 07 Server Auth Principal Config Snapshot
- 10 Server Worker Runner Orchestration

## Risks and Mitigations
- Risk: payload flexibility reintroduces insecure client-side config injection.
- Mitigation: explicitly reject runtime-config payload fields not permitted by policy.
- Risk: inconsistent status code mapping across handlers.
- Mitigation: centralize error-to-status translation.
- Risk: idempotency collisions or incorrect dedup behavior.
- Mitigation: deterministic idempotency key scope and explicit collision tests.

## Verification
- Endpoint tests for auth, validation, and domain-error mapping.
- Contract tests for JSON shape and status codes.
- Idempotency tests for repeated create requests.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates
- Endpoints enforce auth, tenant scoping, and input validation deterministically.
- `POST /v1/jobs` supports idempotency with tested dedup behavior.
- `GET /v1/jobs/{job_id}` and rerun endpoints enforce principal ownership constraints.
- Rerun creates linked new job and returns `rerun_of_job_id` provenance.
- Contract tests verify status codes and response payload shapes.
