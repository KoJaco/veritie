# 09 Server Providers STT LLM

## Objective
Implement batch-first STT and LLM provider adapters behind stable interfaces, with strict error/timeout semantics and no realtime/WebSocket assumptions.

## Why This Branch Exists
Provider integrations are a major source of coupling and runtime instability. This branch creates clean boundaries so job orchestration remains deterministic even when provider behavior varies.

## In Scope
- `server/internal/infra/providers/stt`
- `server/internal/infra/providers/llm`
- Provider interface contracts consumed by `internal/app/jobs`
- Config-driven provider selection and initialization
- Timeout, retry, and error-classification policy for provider calls
- Canonical internal transcript types/structs used across STT adapters

## Out of Scope
- Runner stage orchestration sequencing (branch 10)
- HTTP/SSE endpoint contracts (branches 11 and 12)
- Rich fallback orchestration across multiple providers in one request (future optimization)

## Split Decision
No further branch split required. STT and LLM adapters should ship together because their contracts are co-consumed by the same job pipeline stages and share config, observability, and error-handling patterns.

## Implementation Plan
1. Define provider interfaces in app-facing terms:
   - STT interface for short-audio transcription with timestamps/segments
   - LLM interface for classification/extraction outputs and structured validation feedback
2. Define canonical transcript models (provider-agnostic):
   - shared transcript, segment, token/word, confidence, and timing structs
   - no provider-specific fields in domain-facing types
3. Implement STT adapter set:
   - primary Deepgram adapter as default in `internal/infra/providers/stt`
   - Speechmatics adapter scaffold/stub wired behind same interface and ready to connect
   - request/response mapping from each adapter into canonical transcript models
4. Implement LLM adapter(s):
   - initial configured adapter(s) in `internal/infra/providers/llm`
   - mapping for classification and extraction outputs into app-domain structures
5. Add provider initialization and selection:
   - default STT provider: Deepgram
   - STT provider is swappable via config without app-layer code changes
   - fail fast on invalid provider configuration at startup
   - expose explicit “provider unavailable” errors
6. Define reliability policy:
   - per-provider timeout budgets
   - bounded retry policy for transient failures
   - no retry on deterministic validation/auth failures
7. Add observability hooks:
   - per-call timing metrics
   - error-class tagging
   - correlation fields for job ID/stage/provider
8. Add test coverage:
   - transcript model conformance tests (both STT adapters)
   - adapter mapping unit tests
   - error classification tests
   - timeout/retry behavior tests with fakes/mocks
   - interface contract tests against app jobs interfaces

## Deliverables
- STT provider interface + canonical transcript models
- Deepgram STT adapter as default implementation
- Speechmatics STT adapter stub/scaffold ready for full wiring
- LLM provider interface + configured adapter implementations
- Config-based provider bootstrap and selection logic
- Standard timeout/retry/error handling contract
- Provider-level test suite

## Dependencies
- 05 Server Foundation Config Obs Runtime
- 08 Server Jobs Domain State Machine

## Risks and Mitigations
- Risk: provider response shape drift breaks mappings.
- Mitigation: isolate mapping logic and cover with fixture-based tests.
- Risk: retries increase latency without improving success.
- Mitigation: bounded retries with stage-level timeout caps and metrics review.
- Risk: hidden realtime assumptions leak from legacy code.
- Mitigation: explicitly ban websocket/streaming interfaces in provider contracts and tests.
- Risk: provider-specific transcript fields leak into domain types and break adapter swapability.
- Mitigation: enforce canonical transcript structs and adapter conformance tests.

## Verification
- Unit tests for request/response mapping and error paths.
- Deepgram default-selection test and explicit provider override tests.
- Speechmatics stub wiring test (construction and interface conformance).
- Contract tests confirming providers satisfy app interfaces.
- Timeout/retry tests validate bounded behavior.
- `go test ./...` and `go vet ./...` pass for touched packages.

## Acceptance Gates
- Deepgram is the default STT adapter in provider initialization.
- STT adapters are swappable via shared interfaces and canonical transcript models.
- Speechmatics stub is present and interface-compliant for later full integration.
- Providers compile against app-defined interfaces and are config-initializable.
- Fallback/error behavior is explicit, bounded, and test-covered.
- Provider APIs are batch-first with no websocket/realtime assumptions.
- Provider call metrics and error tags are emitted for observability.
