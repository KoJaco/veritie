# 08 Server Jobs Domain State Machine

## Objective
Define and implement a deterministic jobs domain state machine that governs lifecycle transitions, stage events, and rerun behavior independent of transport concerns.

## Why This Branch Exists
Without a strict state model, runner behavior and API responses drift, leading to invalid transitions, ambiguous retries, and inconsistent SSE/event semantics.

## In Scope
- `server/internal/app/jobs/model.go`
- `server/internal/app/jobs/policy.go`
- `server/internal/app/jobs/interfaces.go`
- `server/internal/app/jobs/service.go`
- Job status model, stage-event model, and rerun linkage
- Validation logic for legal transitions and terminal-state behavior

## Out of Scope
- Worker stage execution internals (branch 10)
- HTTP request/response handler contracts (branch 11)
- SSE transport formatting and replay mechanics (branch 12)

## Split Decision
No further branch split required. Model/policy/interfaces/service should land atomically to avoid transition-rule mismatch between files and reduce integration churn in branch 10.

## Implementation Plan
1. Define core models in `model.go`:
   - job states: `queued`, `running`, `succeeded`, `failed`, `cancelled`
   - stage identifiers aligned to `ground-truth.md`
   - event metadata fields (timestamp, level, message, data payload, trace ids)
   - rerun linkage (`rerun_of_job_id`)
2. Implement transition policy in `policy.go`:
   - explicit allowed state transitions map
   - terminal-state lockout rules
   - idempotent handling for duplicate stage notifications
   - cancellation semantics and priority rules
3. Define orchestration interfaces in `interfaces.go`:
   - repository contracts (jobs + events)
   - provider-facing contracts required by runner
   - clock/id generation abstractions where determinism is needed for testing
4. Implement jobs service in `service.go`:
   - create/start/advance/fail/cancel/complete operations
   - write-through behavior for job state + event append
   - rerun creation path that snapshots references to prior job
5. Add invariants and guardrails:
   - prevent regressions from terminal to non-terminal states
   - prevent out-of-order stage completion updates
   - preserve event ordering guarantees at service boundary
6. Add test coverage:
   - table-driven transition tests
   - rerun semantics tests
   - duplicate event/idempotency tests
   - cancellation race tests at service-level boundaries

## Deliverables
- Formal job state/event models
- Enforced transition policy and invariants
- Stable app-layer interfaces for runner, repos, and providers
- Jobs service methods ready for worker and HTTP integration
- Comprehensive domain/service tests

## Dependencies
- 06 Server DB Postgres Core
- 07 Server Auth Principal Config Snapshot

## Risks and Mitigations
- Risk: transition rules become too permissive under failure paths.
- Mitigation: deny-by-default transition map with explicit allow-list and exhaustive tests.
- Risk: event ordering differs across code paths.
- Mitigation: centralize event append behavior in service and assert ordering in tests.
- Risk: rerun semantics become ambiguous across schema/config changes.
- Mitigation: model rerun as new immutable job linked to source job, with explicit snapshot reference fields.

## Verification
- Unit tests for all legal and illegal transitions.
- Service tests validating event append behavior per transition.
- Rerun tests verifying new job identity with correct linkage metadata.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates
- State transitions are deterministic and enforced by policy.
- Terminal-state guarantees prevent invalid regressions.
- Event ordering and lifecycle invariants are test-covered.
- Rerun semantics create a distinct linked job with preserved provenance.
