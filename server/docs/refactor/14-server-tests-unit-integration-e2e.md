# 14 Server Tests Unit Integration E2E

## Objective
Establish a complete server testing strategy across unit, integration, and contract/e2e scenarios to enforce correctness of job lifecycle, auth, HTTP/SSE behavior, and provider orchestration.

## Why This Branch Exists
By this stage, most core behavior is implemented. Without a unified test harness and CI gates, regressions in state transitions, auth boundaries, and stream semantics will slip through.

## In Scope
- Unit tests for domain/app logic (`jobs`, `auth`, config/validation, provider mappings)
- Integration tests for DB repositories and migrations
- API contract tests for jobs endpoints
- SSE stream behavior tests (ordering, replay, reconnect, auth)
- Test fixtures and helpers for providers/repositories
- CI quality thresholds (coverage/race/flakiness constraints)

## Out of Scope
- Frontend tests
- Load/performance testing as a full benchmark suite (only targeted smoke checks)
- External provider live-environment certification tests (can be separate non-blocking suite)

## Split Decision
No split required. Test strategy and CI quality gates should land together; splitting would delay signal quality and create temporary blind spots.

## Implementation Plan
1. Define server test taxonomy and directory conventions:
   - unit tests co-located with packages
   - integration tests grouped by subsystem with explicit tags/build constraints
   - contract tests for HTTP/SSE payload and status guarantees
2. Build reusable test infrastructure:
   - fixtures for job/event records
   - provider fakes for STT/LLM paths (Deepgram default behavior + adapter abstraction)
   - auth principal/context helpers
   - deterministic clock/id helpers for state-machine assertions
3. Expand unit coverage:
   - jobs transition policy and service invariants
   - auth parsing/resolution and fail-closed behavior
   - provider mapping/error classification
   - config validation and startup guardrails
4. Implement integration suites:
   - Postgres repo CRUD + event ordering
   - Atlas migration replay checks in test workflow
   - API lifecycle path (`POST /v1/jobs`, `GET /v1/jobs/{job_id}`, rerun)
   - SSE replay/live-tail and ownership checks
5. Define CI quality gates:
   - required unit + integration suites
   - race checks for touched/critical packages
   - coverage threshold policy and artifact publishing
   - flaky-test policy (quarantine process + fail criteria)
6. Add regression scenario set:
   - invalid state transition attempts
   - cross-tenant access attempts
   - duplicate idempotency keys
   - stream reconnect at boundary cursor

## Deliverables
- Expanded `_test.go` coverage across server modules
- Integration and contract test suites for DB/auth/jobs/SSE
- Test utilities/fakes for provider and repository boundaries
- CI gate definitions for required checks and thresholds
- Regression test matrix for critical failure modes

## Dependencies
- 06 through 13

## Risks and Mitigations
- Risk: test flakiness from timing-sensitive SSE/worker flows.
- Mitigation: deterministic fakes, bounded waits, and controlled clocks where possible.
- Risk: overreliance on mocks hides integration failures.
- Mitigation: include DB-backed integration tests for core lifecycle paths.
- Risk: test runtime bloat slows developer iteration.
- Mitigation: split fast unit path vs heavier integration path with clear CI stages.

## Verification
- Local run for unit suite + targeted integration suite.
- CI run validates race, coverage, and contract test outputs.
- Regression suite demonstrates protection for known risk scenarios.
- `go test ./...` and `go vet ./...` pass with expected thresholds.

## Acceptance Gates
- Core lifecycle scenarios pass in both unit and integration environments.
- Auth, tenant boundaries, idempotency, and SSE replay behaviors are test-covered.
- Race checks and coverage thresholds are enforced and stable in CI.
- Regression tests exist for identified high-risk failure modes.
