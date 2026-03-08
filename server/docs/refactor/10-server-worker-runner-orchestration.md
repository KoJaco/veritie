# 10 Server Worker Runner Orchestration

## Objective
Implement worker/runner orchestration that executes the staged batch pipeline deterministically, persists lifecycle changes, and emits events for downstream SSE consumers.

## Why This Branch Exists
The jobs domain model (08) and provider adapters (09) are inert until a runner coordinates stage execution, failure handling, retries, and cancellation in one controlled flow.

## In Scope
- `server/internal/app/jobs/runner.go`
- Worker process bootstrapping in `server/cmd/worker/main.go`
- Stage orchestration policy across ingest, stt, classify, extract, validate, persist
- Lifecycle event emission and persistence at stage boundaries
- Cancellation and graceful shutdown behavior for in-flight jobs

## Out of Scope
- HTTP endpoint request/response contracts (branch 11)
- SSE transport endpoint behavior and replay protocol (branch 12)
- Multi-queue/distributed scheduler architecture beyond single-worker baseline

## Split Decision
No split required. Runner and worker boot/runtime controls must land together to avoid partially wired execution paths and unstable operational behavior.

## Implementation Plan
1. Implement runner orchestration core in `runner.go`:
   - stage-by-stage execution pipeline
   - shared stage context carrying principal snapshot, config, and correlation metadata
   - explicit transition calls through jobs service (no direct state mutation)
2. Implement stage execution contract:
   - each stage returns structured result + error classification
   - deterministic mapping of failures to job terminal states/events
   - strict ordering of stage events (start/completed/failed)
3. Integrate provider usage:
   - STT via default Deepgram adapter through interface abstraction
   - classification/extraction via LLM provider interfaces
   - no provider-specific branching outside adapter layer
4. Implement retry and cancellation rules:
   - bounded retries only for retryable/transient stage failures
   - cancellation checks between stages and before expensive provider calls
   - cancellation emits terminal cancellation event path
5. Implement worker process runtime:
   - polling/dispatch loop or queue-consumer loop (single worker baseline)
   - controlled concurrency settings
   - graceful shutdown with in-flight drain deadline
6. Add observability:
   - stage duration metrics
   - per-job correlation logs
   - failure-class metrics by stage and provider
7. Add test coverage:
   - happy-path end-to-end runner tests (with fakes)
   - transient failure + retry tests
   - permanent failure tests
   - cancellation and shutdown behavior tests

## Deliverables
- Production-ready runner orchestration implementation
- Worker boot loop with graceful shutdown controls
- Stage event emission tied to persisted job lifecycle
- Retry/cancellation behavior documented and test-covered

## Dependencies
- 08 Server Jobs Domain State Machine
- 09 Server Providers STT LLM

## Risks and Mitigations
- Risk: stage execution ordering drifts from domain transition rules.
- Mitigation: enforce all state transitions through jobs service APIs only.
- Risk: retries create duplicate side effects/events.
- Mitigation: idempotency guards and explicit retry-safe stage boundaries.
- Risk: shutdown interrupts jobs in inconsistent state.
- Mitigation: drain strategy with timeout and deterministic cancellation fallback.

## Verification
- Runner unit tests for success/failure/retry/cancel paths.
- Worker lifecycle tests for startup and graceful shutdown.
- Integration test validates persisted events align with stage order.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates
- Runner executes full happy path with persisted state and ordered events.
- Failures emit typed error events and result in correct terminal states.
- Cancellation behavior is deterministic and test-covered.
- Worker process handles shutdown safely with in-flight awareness and bounded drain policy.
