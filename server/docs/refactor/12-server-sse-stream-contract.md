# 12 Server SSE Stream Contract

## Objective
Implement and lock the SSE contract for job lifecycle streaming with authenticated access, deterministic ordering, and replay support from persisted checkpoints.

## Why This Branch Exists
SSE is the real-time visibility layer for batch jobs. Without a strict stream contract, clients cannot reliably render progress, recover from disconnects, or trust event ordering.

## In Scope
- `GET /v1/jobs/{job_id}/stream`
- SSE event envelope/schema and serialization rules
- Keepalive heartbeat behavior
- Replay/resume strategy backed by persisted `job_events`
- Auth + principal ownership enforcement on stream access

## Out of Scope
- Job creation/retrieval/rerun HTTP endpoint contracts (branch 11)
- Worker pipeline stage execution logic (branch 10)
- WebSocket transport compatibility paths

## Split Decision
No split required. Stream auth, event envelope, keepalive, and replay behavior must be implemented together to avoid incompatible client/server semantics.

## Implementation Plan
1. Define SSE event contract:
   - event envelope fields (`event`, `id`, `data`, `timestamp`, `job_id`, `stage`, `level`)
   - stable JSON payload schema aligned to `job_events` persistence model
   - event naming conventions for lifecycle stages and terminal states
2. Implement stream handler in HTTP transport:
   - route registration for `GET /v1/jobs/{job_id}/stream`
   - shared auth middleware usage (principal + config snapshot)
   - principal/job ownership validation before stream starts
3. Implement replay/resume behavior:
   - support resume from last event cursor (`Last-Event-ID` or explicit cursor strategy)
   - load persisted events in deterministic order before live tailing
   - transition seamlessly from replay batch to live event feed
   - keep runtime SSE granularity independent from persistence write granularity
4. Implement live fanout behavior:
   - subscribe stream to in-process event broadcaster source
   - emit ordered events as runner persists stage updates
   - ensure write errors/disconnects cleanly unsubscribe resources
5. Implement keepalive and connection policy:
   - periodic heartbeat frames to keep intermediaries alive
   - bounded idle and write timeout handling
   - explicit close behavior when job reaches terminal state (or configurable linger)
6. Add test coverage:
   - stream ordering tests
   - replay/resume tests
   - auth/ownership tests
   - disconnect/reconnect and resource cleanup tests

## Deliverables
- SSE stream handler and routing integration
- Versioned/locked event serialization contract
- Replay + live-tail stream behavior
- Keepalive/timeout policy implementation
- SSE contract and handler tests

## Dependencies
- 10 Server Worker Runner Orchestration
- 11 Server HTTP Jobs Contract

## Risks and Mitigations
- Risk: event ordering divergence between DB replay and live stream.
- Mitigation: one canonical sort key and shared serializer for both paths.
- Risk: stream access leaks cross-tenant job data.
- Mitigation: enforce principal ownership check before replay and live subscribe.
- Risk: resource leaks on disconnected clients.
- Mitigation: explicit unsubscribe/cleanup hooks and disconnect tests.

## Verification
- Contract tests for SSE envelope and field stability.
- Replay tests validating deterministic order with persisted events.
- Live stream tests validating new events after replay boundary.
- Auth tests for unauthorized and cross-account access denial.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates
- Clients receive ordered lifecycle events over SSE.
- Reconnect/resume behavior is deterministic and replay-safe.
- Stream endpoint enforces auth and principal ownership boundaries.
- Keepalive/timeout behavior prevents silent dead streams and resource leakage.
