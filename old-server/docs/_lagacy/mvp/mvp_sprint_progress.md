MVP Sprint Progress Review 0. Infrastructure Scaffolding
Status: ✅ Complete
Database schema, migrations, SQLC, and local/remote workflows are streamlined and automated.

1. Auth + Settings Cache
   Status: ✅ Complete
   KeyAuth middleware, LRU cache, and rate limiter are implemented and tested.
   Auth is integrated into all relevant HTTP and WebSocket endpoints.
2. WebSocket Core
   Status: ✅ Complete
   WebSocket handler, pipeline, and real-time session logic are in place.
   Auth is enforced.
   Silence handler implemented as dedicated service with ring buffer for STT fallback.
   10-second audio ring buffer prevents audio loss during connection issues.
3. Batch Lane
   Status: ✅ Complete
   Batch jobs table, queries, and models are in place.
   HTTP 202 handler for uploads, job status, and job listing endpoints are implemented.
   Background workers process jobs and update status.
   End-to-end flow from upload to result is present.
4. Session Usage Accumulator + Draft Function Analytics
   Status: ✅ Complete
   Usage metering, cost tracking, and draft function aggregation system implemented.
   Periodic flush to DB, rollup logic, and session close handling all in place.
   Real-time analytics track draft function detections vs final function calls.
   Comprehensive database schema with smart upserts and session-level statistics.
5. Silence Handler
   Status: ✅ Complete
   Silence detection service implemented with clean architecture separation. Keep-alive pings are now suppressed while real audio is streaming (2025-07-31 refactor).
   Configurable thresholds (3s silence detection, 2s keep-alive intervals).
   Event-driven notifications and STT connection keep-alive pings.
   Ring buffer integration for audio fallback replay.
6. Dynamic Config Watcher
   Status: ✅ Complete
   Real-time function config updates during active database sessions (audio start → stop cycles).
   Tracks all configs used per database session for end-of-session storage.
   Hot-swap capability for adaptive function calling without reconnection.
   WebSocket handler orchestrates config watcher per database session.

7. Persistent WebSocket Sessions (Simplified Architecture)
   Status: ✅ Complete
   Implements a simplified architecture for real-time sessions:

    - WebSocket handler manages connection state and tracks all database sessions
    - Pipeline handles individual database session lifecycle (audio start → stop)
    - Config watcher tracks config changes within each database session
    - LLM session remains connection-level (reusable across audio sessions)
    - On each `audio_start`, pipeline and config watcher are created for that database session
    - On `audio_stop`, pipeline closes and config watcher flushes configs to database
    - Supports multiple audio sessions per connection with clear separation of concerns
    - No separate session manager - WebSocket handler orchestrates everything

8. LLM Tools & System Instruction Caching (Google Gemini)
   Status: ✅ Complete (Phase 1)
   Implemented a caching strategy for LLM tools (function schemas) and system instructions on Google's servers (Gemini), integrated with the GenAI v2 SDK and dynamic config watcher.

    - On session start and on dynamic schema updates, proactively build static context (tools + system) and create provider cache via Caches.Create (with ToolConfig ANY); hard-skip caching if estimated tokens < 1024.
    - Subsequent LLM requests use `GenerateContent` with `CachedContent` set (no tools/system/tool_config in the request) and low temperature for determinism.
    - LLM session reconfigures tools/system when config fingerprint changes; dynamic watcher stages pending config and applies on `audio_start` if no active session.
    - Accurate usage accounting: prompt/completion tokens plus `saved_prompt_tokens` recorded via adapter v2 and accumulated into DB.
    - Fallback: if cache is unavailable/invalid or creation fails, calls proceed uncached; repeated cache attempts for known failing versions are skipped.
    - Observability: span-based debug logs (cache prep, cache used), detailed STT and LLM timings.

    Remaining follow-ups (post-Phase 1):

    - Safe swap protocol when configs change mid-session (flush interim → final call under old config → pause → swap draft index + cache → resume).
    - Expose Prometheus `/metrics` and add cache-effectiveness metrics (hit rate, savings).

9. Health & Readiness Endpoints
   Status: ✅ Complete
   Comprehensive health system with /healthz (liveness) and /readyz (readiness).
   Checks database connectivity, STT providers, filesystem, and environment.
   Fly.io health check configuration with automatic restarts.
   Detailed component status with latency tracking and rich diagnostics.

10. Structured-output Feature
    Status: ⏳ Not started
    TODO: Implement Structured Output Implementation.

11. PII Redaction Middleware
    Status: ⏳ Not started
    TODO: Regex detector, redaction logic, and Prometheus counters.

12. Config-Ping Endpoint
    Status: ⏳ Not started
    TODO: Implement /api/v1/cost-estimate endpoint.

13. Pre-MVP Finalise (Concurrency Bugs)
    Status: ⏳ Not started
    A concurrency issue occurs when a `dynamic_config_update` arrives while the transcript has non-final interim text:

    - Interim transcript must be flushed into finals before switching config.
    - Perform a last LLM call with the old config to capture any pending functions.
    - Temporarily pause the pipeline during this flush to avoid race conditions.
    - Resume streaming with the new config and ensure behaviour is covered by an integration test.

## MVP Cleanup Tasks

### Naming Consistency

-   **draft_agg → function_agg**: Rename all "draft_agg" references to "function_agg" for better clarity
    -   Database tables: `draft_function_aggs` → `function_aggs`, `draft_function_stats` → `function_stats`
    -   Go types: `DraftAgg` → `FunctionAgg`, `DraftAggregator` → `FunctionAggregator`
    -   File names: `draft_agg_*.go` → `function_agg_*.go`
    -   Documentation: Update all references in docs and comments
    -   SQL queries: Update query files and repository methods

### Prompt/Checksum Consistency

-   **Prompt deduplication bug**: Currently, prompts are being saved with dynamic, time-based, or transcript-specific content, which breaks checksum deduplication.
    -   TODO: Refactor so that only the actual parsing guide (as sent by the user/client) is saved and checksummed.
    -   Move any dynamic or time-based prompt construction to the server, and ensure only the static parsing guide is persisted and deduplicated.
    -   Ensure this applies to all prompt storage and linking logic.

### Ring Buffer Documentation

-   **Undocumented, untested**: Currently, ring buffer is untested and undocumented... must do this.

### Usage Accumulator

-   **Proper error handling**: Add errors the usage accumulator method, allowing for proper error handling is web scoket handler.
-   Adjust app file
-   Adjust websocket handler to log printf failures if we get an error from the method.

### Depricated Genai SDK

-   Adjust our google gemini genai SDK to https://ai.google.dev/gemini-api/docs/quickstart?lang=go#go

### Testing

-   **Unit testing**: Must unit test for each service we have
-   **Integration testing**: Provide integration testing for the ws handler, \_\_\_\_ .

## Post MVP-Frontend Build

1. CLI & Cron Jobs
   Status: ⏳ Not started
   TODO: Implement a SCHAFFOLD for cron jobs and CLI tools to be built

2. Prometheus Monitoring
   Status: ⏳ Not started
   TODO: Expose metrics and commit Grafana dashboard.

    - /metrics endpoint, prometheus scrapes.
    - Use a package or make myself?

3. Grammar and Syntax Feature
   Status: ⏳ Not started
   TODO: allow users to receive back their transcript in real-time with grammar and syntax corrected (according to prompt)

4. Diarization support
   Status: ⏳ Not started
   TODO: support multiple speakers. If checked, will need to add speaker flag and speaker confidence (only for batch) to the transcript channel. Will likely need to update the function parsing logic. Are we identifying functions based on the entire speaker output? yeah probably... How are we identify perhaps the primary speaker and other speakers and weighing whether to use one output over the other? Or exclude speakers? This is gunna be difficult, left till the end to sort out. Will start with a naive strategy and document it.

### Caching

Static context cache per mode:

-   Functions: tools + system instruction.
-   Structured: JSON schema + system instruction.

Behavior:

-   Pre-warm cache on `audio_start`.
-   Keyed by fingerprint of static content.
-   Skip creation for tiny contexts (token threshold).
-   Track hit/miss and saved tokens.

---

### Dynamic Updates and Safe Swap

Functions:

-   `dynamic_config_update` hot-swaps draft index and function config.
-   Safe swap protocol (planned): flush under old config if interim text exists, pause, swap, resume.

Structured:

-   `dynamic_structured_update` validates schema.
-   If no active audio session: stage for next session.
-   If active audio session: pause structured slot, optionally flush, invalidate/prepare new cache, attempt field-preserving migration if schemas are compatible; otherwise reset structured state; resume.

Mode mismatches:

-   Reject dynamic updates that don’t match current mode (or stage for next session) with a clear `config_update_ack` message.

---

### Usage Metering and Metrics

Usage:

-   Record tokens and cost for each LLM call (both modes).
-   Include cache savings when available.
-   Emit lightweight events for observability (e.g., `structured_update` with `rev` and changed fields).

Metrics (per provider, cached label):

-   `llm_request_total{mode=functions|structured}`
-   `llm_tokens_prompt_total{mode=...}`
-   `llm_tokens_completion_total{mode=...}`
-   `llm_request_latency_ms{mode=...}`
-   Cache effectiveness metrics (hit rate, saved tokens/cost).

---

### Error Handling

-   Mixed initial config → error; require exclusivity.
-   Invalid structured schema → error; structured slot disabled.
-   Provider-side schema errors → fallback to unconstrained JSON with local validate/prune.
-   If neither functions nor structured provided → STT-only session.

---

### Acceptance Criteria

-   Server enforces mutual exclusivity for `function_config` and `structured_config`.
-   On `audio_start`, only the active mode makes LLM calls; the other is inert.
-   Functions mode streams merged `functions` updates; structured mode streams `structured` deltas and a final object on stop.
-   Dynamic updates are handled per-mode with staging or safe swap; no race conditions or panics.
-   Usage metering and metrics recorded for both modes.
-   Backward compatible with existing clients that only send `function_config`.

---

### Future (Post-MVP)

Optional concurrent dual-mode:

-   Run two independent LLM sessions (functions + structured) per connection with separate caches, throttles, and outputs.
-   Add rate limiting and per-connection token budgets to control cost/latency.
-   Maintain strict isolation between the two modes’ conversations and static contexts.
