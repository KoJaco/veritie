# 13 Server Observability and Usage

## Objective
Implement production-grade observability and usage instrumentation across API, worker, and provider paths to make job execution auditable and operationally actionable.

## Why This Branch Exists
Refactor velocity increases failure risk. Without structured telemetry and usage tracking, diagnosing regressions, enforcing SLOs, and understanding cost/performance becomes unreliable.

## In Scope
- HTTP middleware instrumentation for requests and auth path timing
- Runner stage metrics, logs, and tracing spans
- Provider call metrics and error classification tags
- Usage event collection and persistence hooks
- Correlation IDs across request/job/stage/provider boundaries
- Runtime-vs-persistence telemetry separation (stream emissions vs durable checkpoints)

## Out of Scope
- External dashboard implementation details
- Billing product logic beyond usage event generation and persistence
- Frontend observability

## Split Decision
No split required. Metrics, logs, tracing, and usage events should land as one coherent instrumentation layer to avoid partial visibility gaps.

## Implementation Plan
1. Define observability taxonomy:
   - required dimensions: `job_id`, `account_id`, `app_id`, `stage`, `provider`, `status`
   - metric naming conventions and cardinality guardrails
   - log fields required for incident triage
2. Implement HTTP instrumentation:
   - request duration/status metrics
   - auth-path timing metrics (parse/lookup/config-resolve)
   - request correlation ID generation/propagation
3. Implement runner instrumentation:
   - stage start/end logs with durations
   - stage result counters (success/failure/cancelled)
   - retry count metrics and terminal-state counters
4. Implement provider instrumentation:
   - per-call latency histograms
   - provider-specific error-class counters
   - timeout/retry counters by provider and stage
5. Implement tracing spans:
   - root span per request/job execution
   - child spans for stage execution and provider calls
   - trace propagation through service/repository/provider boundaries
6. Implement usage collection plumbing:
   - emit usage events at key points (audio duration, provider calls, tokens/compute proxies)
   - persist usage events through repository layer
   - ensure usage writes are non-blocking or bounded to avoid hot-path latency spikes
7. Add test coverage and validation:
   - instrumentation unit tests for metric/log emission
   - usage event persistence tests
   - smoke verification that critical job path emits required telemetry fields

## Deliverables
- Structured logs with request/job correlation IDs
- Metrics for request/auth, job states, stage timings, retries, and provider calls
- Trace spans across API -> jobs service -> provider/repo paths
- Usage event collection and persistence integration
- Observability/usage test coverage

## Dependencies
- 05 Server Foundation Config Obs Runtime
- 10 Server Worker Runner Orchestration
- 11 Server HTTP Jobs Contract

## Risks and Mitigations
- Risk: high-cardinality labels degrade metrics backends.
- Mitigation: define and enforce label allow-list and avoid unbounded IDs in metric labels.
- Risk: instrumentation overhead increases latency.
- Mitigation: keep synchronous work minimal and benchmark critical paths.
- Risk: inconsistent correlation fields reduce debugging value.
- Mitigation: enforce shared logging/context helpers and contract tests for required fields.

## Verification
- Unit tests validating metric/log/tracing hooks on core paths.
- Usage event tests confirming persisted records and required fields.
- Load/smoke checks to ensure instrumentation does not materially regress p95 latency.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates
- Job execution path is traceable end-to-end via correlation IDs and spans.
- Stage-level metrics and provider metrics are emitted with stable naming/dimensions.
- Usage events are persisted, queryable, and scoped to app/account/job context.
- Auth and request timing telemetry is available for latency/SLO monitoring.
