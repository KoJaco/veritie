# Schma MVP Sprint Board (v2)

---

## 0. Infrastructure Scaffolding

-   **Concrete tasks**

    -   Add latest DDL file and run `atlas schema apply`.
    -   Generate `sqlc` models with `make db`.

-   **Key choices / libs**
    Atlas, sqlc

-   **Definition of Done**
    All `go test ./internal/db` tests pass.

---

## 1. Auth + Settings Cache

-   **Concrete tasks**

    1. Implement **`KeyAuth` middleware**: bcrypt lookup → context puts `AccountID`, `AppID`.
    2. Add `lru.Cache[uuid.UUID]*AppSettings` with 30 s TTL.
    3. Add in-memory **rate limiter** keyed on `AppID` (`ulule/limiter`).

-   **Key choices / libs**
    `hashicorp/golang-lru/v2`, `ulule/limiter/v3`

-   **Definition of Done**
    Median auth-path latency ≤ 250 µs (bench); invalid key returns **401**.

---

## 2. WebSocket Core

-   **Concrete tasks**

    -   **Silence handler**: when `audio_idle > 3 s`, send STT keep-alive ping.
    -   10-second **ring buffer** for STT fallback replay.
    -   Build and test `shouldTriggerLLM` delta-trigger helper.
    -   Add new channels: `outStruct`, `outSummarize` for structured LLM output.

-   **Key choices / libs**
    Gorilla WS

-   **Definition of Done**
    30-minute soak shows no silent dropouts; fallback gap ≤ 250 ms.

---

## 3. Batch Lane `/api/v1/batch`

-   **Concrete tasks**

    1. HTTP **202** handler: save upload, insert `batch_jobs`.
    2. Non-blocking in-proc worker consumes queue.
    3. Re-use pipeline; write `usage_events` & `function_rollups`.

-   **Key choices / libs**
    Re-use existing accumulator structs

-   **Definition of Done**
    90-minute WAV returns final transcript + cost.

---

## 4. Session Usage Accumulator

-   **Concrete tasks**

    -   Flush `Acc` struct to `usage_events` every 5 s (include CPU ms).
    -   Implement `DraftAgg` roll-up (**highest stab**).
    -   Flush on final LLM batch and on session close.

-   **Definition of Done**
    Draft streak of 20 produces exactly **one** roll-up row.

---

## 5. Dynamic Schema Watcher

-   **Concrete tasks**

    1. Client sends `schema_version` header.
    2. Goroutine polls `apps.schema_hash` every 5 s.
    3. On change: re-init Gemini tools **after** draining pending queue.

-   **Key choices / libs**
    `atomic.Value` to hot-swap tool list

-   **Definition of Done**
    Schema swap adds ≤ 1 LLM round-trip delay; no lost functions.

---

## 6. Persistent WebSocket Sessions (Hybrid Architecture) ✅

-   **Concrete tasks**

    -   Refactor WebSocket handler to support multiple audio sessions per connection.
    -   Move LLM session to connection-level (created once, reused, updatable by schema watcher).
    -   Spin up pipeline and all session-level components (STT client, DB session, usage accumulator, pipeline channels) on `audio_start`.
    -   Spin down session-level components and close pipeline on `audio_stop` (by closing upstream audio channel).
    -   Ensure dynamic schema watcher can update LLM session tools within or between sessions.
    -   Update state management and lifecycle logic for hybrid architecture.

-   **Key choices / libs**
    Gorilla WS, Gemini LLM, custom pipeline orchestration

-   **Definition of Done**
    Multiple audio sessions per connection work reliably; LLM session is updatable and reused; session-level resources are created/destroyed per session; pipeline closes naturally; schema/tool updates are reflected live.

---

## 7. LLM Tools & System Instruction Caching (Google Gemini)

-   **Concrete tasks**

    -   Implement a cache middleware in `infra/llmgemini` to intercept LLM session initialization and tool updates.
    -   On session start or tool/schema update, upload tools and system instructions to Google's cache endpoint and store the cache key.
    -   For subsequent LLM requests, send only the cache reference if available; otherwise, send the full tools/instructions.
    -   Integrate with the dynamic schema watcher to invalidate and refresh the cache on tool/schema changes.
    -   Add fallback logic if the cache endpoint is unavailable.
    -   Track cache key per session or schema version in session state.
    -   Document the caching strategy and update the persistent WebSocket session docs as needed.

-   **Key choices / libs**
    Gemini LLM, custom cache middleware, session state management

-   **Definition of Done**
    LLM tools and system instructions are cached on Google's servers; repeated requests use cache reference; prompt size is reduced; cache is invalidated and refreshed on schema/tool changes; fallback works if cache is unavailable.

---

## 8. Health & Readiness Endpoints

-   **Concrete tasks**

    -   `/healthz` (liveness).
    -   `/readyz` (readiness): DB, Stripe & STT checks.
    -   Add Fly `[[checks]]` config.

-   **Key choices / libs**
    `internal/health` package

-   **Definition of Done**
    Fly restarts unhealthy instance; manual `curl` shows JSON status.

---

## 9. Prometheus Monitoring

-   **Concrete tasks**

    -   Expose counters/gauges: `SCHMA_ws_active`, `SCHMA_llm_latency_ms`, `redactions_total`, etc.
    -   Commit Grafana dashboard JSON.

-   **Key choices / libs**
    `prometheus/client_golang`

-   **Definition of Done**
    Grafana shows live metrics locally.

---

## 10. PII Redaction Middleware

-   **Concrete tasks**

    1. Regex detector (email, AU phone, TFN); toggle via `AppSettings.AutoPIIRedact`.
    2. Redact transcripts & roll-ups; set `pii_redacted=true`.
    3. Increment Prom counter per redaction type.

-   **Key choices / libs**
    `regexp2`; future plug-in: spaCy / NER microservice

-   **Definition of Done**
    Regex unit tests pass; ≤ 10 µs / 100 chars in bench.

---

## 11. Config-Ping Endpoint `/api/v1/cost-estimate`

-   **Concrete tasks**

    -   Accept JSON config (e.g., `{stt:"dg", llm:"g2.5", audio_sec:120}`).
    -   Calculate & return estimated cost + latency (stub latency constants for now).

-   **Key choices / libs**
    `usage.CalcCost()` helper

-   **Definition of Done**
    CLI estimate within ± 5 % of actual bill.

---

## 12. CLI & Cron Jobs

-   **Concrete tasks**

    -   `memoctl partitions rotate` (daily).
    -   `memoctl usage rollup` + `memoctl stripe export`.
    -   `memoctl batch list`, `memoctl sessions list`, `memoctl key disable`.

-   **Key choices / libs**
    Cobra

-   **Definition of Done**
    Disabling a key terminates live WS within 10 s; daily roll-up completes.

---

## 13. LLM Trigger Services (Function + Structured Output)

-   **Concrete tasks**

    -   Implement `FunctionTriggerService`:

        -   Watches draft detection and transcript delta size.
        -   Calls Gemini using tools config and rolling prior function list.
        -   Outputs to `outFn`.

    -   Implement `StructuredTriggerService`:

        -   Triggers on pause, session-end, or manual override.
        -   Uses user-defined structured schema.
        -   Outputs to `outStruct`.

    -   Add runtime support for:

        -   `FunctionParsingConfig` (tools)
        -   `StructuredOutputSchema` (1-level-deep, type guard this in SDK and use a helper function `defineStructuredOutputSchema`)
        -   Trigger modes (`pause`, `manual`)

-   **Key choices / libs**
    `context.WithCancel`, `atomic.Value`, Gemini Flash 1.5

-   **Definition of Done**
    Real-time function triggering ≤ 1.5 s avg; structured output populated on manual/timeout trigger.

---

## 14. Pre-MVP Finalise (Concurrency Bugs)

-   **Concrete tasks**

    1. On `dynamic_config_update`, immediately _flush_ the current interim transcript: append the in-flight interim text to the session’s final transcript slice.
    2. Perform one **final LLM function-call pass** using the _previous_ configuration to ensure any functions triggered by that transcript chunk are captured.
    3. Pause or synchronise the pipeline (e.g., via a barrier channel or mutex) while the final LLM call completes to avoid race conditions.
    4. After the flush + LLM call succeed, proceed with the new config update and resume normal streaming.
    5. Add integration tests that stream audio, issue a mid-speech config change, and verify transcripts & function calls are split correctly across configs.

-   **Key choices / libs**
    `context.WithCancel`, channel barriers, `sync.WaitGroup`, pipeline `UpdateDraftIndex` helper.

-   **Definition of Done**
    No transcript text is lost during a config change; function calls reference the correct pre-change transcript; no data races or panics under load-test with rapid config updates.

---

## Five-Day Sequence (suggested)

| Day       | Focus                         | Expected completions                |
| --------- | ----------------------------- | ----------------------------------- |
| **Day 0** | DB + Auth                     | Epics 0 & 1 finished.               |
| **Day 1** | WS engine                     | Epics 2 & 4 complete.               |
| **Day 2** | Batch lane                    | Epic 3 delivered.                   |
| **Day 3** | Health, Prom, PII             | Epics 6, 7, 8 live.                 |
| **Day 4** | Schema, Config-Ping, CLI, LLM | Epics 5, 9, 10, 11; tag **v0.1.0**. |

---

### Tiny Gotchas

-   **Keep-alive**: send provider-specific ping every 10 s when silence > 3 s.
-   **Rate-limit burst**: set burst = 20 to absorb reconnects.
-   **Schema hot-swap**: compare SHA-256, not deep structs, before re-init.
-   **Batch RAM**: stream file in 32 kB chunks; never load entire WAV.

### Tiny MVP Polish items

1. Env / secrets template
   Ship a .env.sample listing DEEPGRAM_KEY, GOOGLE_STT_KEY, JWT_SECRET, STRIPE_SECRET, etc.

2. One-command local bootstrap
   make dev → spins Postgres with partitions, runs migrations, starts server, Prom & Grafana.

3. Smoke test script
   A Go or Bash script that:
   creates a temp app/key → streams 30 s of wav → checks transcript length → queries usage_events.
   You’ll burn this every time you change the pipeline.

4. Runbook
   A markdown page: “If /readyz red == DB → restart Fly Postgres; if STT down → env-switch provider.”

### Get to quick post MVP

1. Caching!

-   Must cut LLM costs, effective caching strategy for Tools sent to gemini. Need to maintain cache on my own end to determine if we must update Google's Cache.

2. Wiring up stripe webhook events - 'stripe-go'

---
