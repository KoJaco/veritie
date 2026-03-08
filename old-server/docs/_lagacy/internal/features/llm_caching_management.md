## LLM Caching Management

This document specifies the feature for caching static LLM context (function tools and system instructions) with Google Gemini to reduce prompt size, latency, and cost during real-time sessions. It also records the current implementation state and the work remaining to complete the feature.

### Goals

-   Minimize repeated transmission of static context: function tool schemas and system (parsing) instructions.
-   Reduce prompt tokens and end-to-end latency for repeated LLM calls within a session.
-   Integrate seamlessly with dynamic configuration updates without requiring clients to reconnect.
-   Provide safe fallbacks when the cache is unavailable or invalid.
-   Expose observability for cache effectiveness and token savings.

## Architecture (Hexagonal)

-   **Domain (ports & models)**

    -   `internal/domain/speech/llm_cache_port.go`
        -   `type LLMCache interface { ... }`: Abstract cache operations (Store, Get, Invalidate, etc.).
        -   `type CachedLLM interface { LLM; EnrichWithCache(...) }`: Extends LLM with a cached-call path.
        -   `type StaticContext`: Domain representation of static context to cache (tools + parsing guide + version/checksum).
        -   `type CacheKey`: Opaque identifier returned by provider.

-   **Infrastructure (adapters)**

    -   `internal/infra/llmgemini/cache.go`
        -   `GeminiCache` implements `speech.LLMCache` using Google’s Cached Content APIs (`CreateCachedContent`, `GetCachedContent`, `DeleteCachedContent`).
    -   `internal/infra/llmgemini/session.go`
        -   `GeminiSession.CallFunctions(...)`: normal generation path.
        -   `GeminiSession.CallFunctionsWithCache(...)`: cached-content path (to pass the cache key to generation).
    -   `internal/infra/llmgemini/adapter.go`
        -   Adapter implements `speech.LLM` (normal path). Will be extended to implement `speech.CachedLLM` for cached path.

-   **Application (orchestration)**

    -   `internal/app/llmcache/llm_cache_service.go`
        -   `LLMCacheService` coordinates: compute version checksum → store/refresh cache → choose cached vs normal enrich path.
        -   Exposes `EnrichWithOptimalStrategy(...)` that tries cached path first, then falls back.

-   **Transport**
    -   WebSocket handler (`internal/transport/ws/handler.go`) orchestrates live sessions and dynamic config updates. It will call into the app service (directly or via pipeline deps) to ensure cache is prepared on session start and on `dynamic_config_update`.

## Lifecycle & Flows

### 1) Session start (audio_start)

-   Compute context version checksum from current function config (schemas + parsing guide).
-   If version differs from in-memory state, upload static context to provider cache:
    -   Build system message via `prompts.BuildFunctionsSystemInstructionPrompt`.
    -   Convert function definitions to provider tool schema.
    -   Call `LLMCache.Store(...)` → returns `CacheKey`.
-   Store `CacheKey` in session-scoped state (app service) and in the config watcher’s tracked state for visibility.

### 2) Per-final-transcript LLM call

-   Build user prompt (transcript-derived).
-   Try cached call first: `CachedLLM.EnrichWithCache(ctx, cacheKey, prompt, partial)`.
-   If cache is unavailable/invalid or the call fails, gracefully fall back to `LLM.Enrich(...)`.
-   Record token usage and compute token savings (if available from provider usage metadata).

### 3) Dynamic config updates (hot-swap)

-   On `dynamic_config_update` during an active session:
    -   Compute the new checksum.
    -   Concurrency protocol: flush interim transcript into finals; perform final LLM call under old config; pause pipeline; swap to new draft index and new cache key; resume pipeline.
    -   Invalidate outdated cache key (best effort) after the swap to avoid use-after-invalidate races.

### 4) Session end (audio_stop)

-   Optionally invalidate cache key (policy-driven) or let provider TTL expire naturally.
-   Persist which config versions (and optionally cache keys) were used in the session for audit/analytics.

## Error Handling Strategy

-   Domain error taxonomy indicates categories: unavailable, invalid, expired, corrupt, miss. These are mapped from provider errors.
-   Miss/Hit are results, not errors, in runtime paths. Use boolean results or counters to track cache effectiveness.
-   Fallback behavior: If cache store/validate fails, or cached call fails, use normal `Enrich(...)` path.

## Observability

-   Metrics to emit:
    -   Cache store attempts/success/fail and latency.
    -   Cache invalidations and failures.
    -   Cache hit rate on LLM calls (attempted cached vs actual cached success).
    -   Token usage: prompt/completion tokens with and without cache; estimated savings per call and per session.
    -   Errors by `CacheErrorType`.

## Configuration

-   Provider model name and temperature/top-p/top-k.
-   Cache TTL policy (e.g., 24h), feature flag to enable/disable caching.
-   Reconfiguration policy: when checksum changes, rebuild session tools and cache reference.
-   Multi-provider support is handled outside the raw cache key (do not mutate provider key protocol strings).

## Current Status (as implemented today)

-   Domain:

    -   `LLMCache` and `CachedLLM` ports exist; `StaticContext`, `CacheKey`, and typed `CacheError` defined.
    -   Some error constructors include "hit" as an error (should be modeled as a result, not an error).

-   Infrastructure:

    -   `GeminiCache` implements `Store`, `Get` (validate semantics), `Invalidate/Delete`, `IsValid`, `IsAvailable`, etc.
    -   `GeminiSession.CallFunctionsWithCache(...)` exists but currently does not pass the cache reference to the generation call; it effectively performs an uncached call.
    -   `Adapter` implements standard `LLM.Enrich(...)` but does not implement `CachedLLM.EnrichWithCache(...)` yet.
    -   `ConfigureOnce` prevents reconfiguration of tools/system messages, so dynamic config updates are not reflected post-first call.

-   Application:

    -   `LLMCacheService` computes checksum, (re)stores cache, and selects cached vs normal path, but:
        -   It prefixes the cache key with `provider:` which will break provider calls that expect the raw cache key.
        -   It holds mutable state (`currentKey`, `lastVersion`) without synchronization.

-   Transport/Pipeline Integration:
    -   Pipeline still calls `LLM.Enrich(...)` directly; not yet wired to the caching service or the cached path.
    -   `dynamic_config_update` flow does not yet prepare/refresh cache nor coordinate the interim transcript flush + safe swap.

## TODO (to complete the feature)

### Critical path

-   Implement proper cached calls:
    -   [ ] Update `GeminiSession.CallFunctionsWithCache(...)` to pass the provider’s cached content reference (e.g., `genai.WithCachedContent(cacheKey)` or the current SDK’s equivalent) instead of ignoring the key.
    -   [ ] Extend `infra/llmgemini/adapter.go` to implement `speech.CachedLLM` with `EnrichWithCache(...)` delegating to session cached call, including usage extraction.
-   Wire caching into the pipeline:
    -   [ ] Provide a shim so `pipeline` can depend on a type that routes `Enrich(...)` to `LLMCacheService.EnrichWithOptimalStrategy(...)` without invasive changes; or update pipeline to use `speech.CachedLLM` directly.
    -   [ ] Inject `LLMCacheService` and replace current `deps.LLM` usage accordingly.
-   Fix cache key handling & thread-safety:
    -   [ ] Remove provider prefixing from `LLMCacheService`. Keep raw cache key for provider calls; track provider separately if needed.
    -   [ ] Add a `sync.RWMutex` to protect `currentKey` and `lastVersion` in `LLMCacheService`.
-   Support dynamic reconfiguration:
    -   [ ] Replace `ConfigureOnce` with reconfiguration keyed by checksum/tool-hash. When checksum changes, rebuild tools/system instructions for the session.
    -   [ ] On `dynamic_config_update`, compute new checksum, prepare new cache key, and atomically swap the LLM session configuration and cache reference.

### Concurrency and correctness

-   [ ] Implement the safe swap protocol when configs change mid-session: flush interim to finals, final LLM call under old config, pause pipeline, swap draft index + cached context, resume.
-   [ ] Ensure cache invalidation of the old key happens after the swap (and not while calls are in-flight).

### Error handling & semantics

-   [ ] Remove `NewCacheHitError` and treat hit/miss as results (metrics), not error types.
-   [ ] Clarify `LLMCache.Get` semantics in docs (validation only; provider won’t return full context).
-   [ ] Add provider-specific error mapping to domain `CacheErrorType` for clearer ops signals.

### Observability

-   [ ] Add metrics and logs: cache store/invalidate success/fail, hit rate, token savings, per-model stats, and error counts by type.
-   [ ] Add debug logging for cache key selection and reconfiguration events (behind a verbose flag).

### Configuration

-   [ ] Expose model name, cache TTL, and caching feature flag via config/env.
-   [ ] Decide policy for cache invalidation on session end vs TTL expiration.

### Testing

-   [ ] Unit test `GeminiCache` with a mocked `genai.Client` (Store/Invalidate/Get/IsValid paths and error mapping).
-   [ ] Unit test `LLMCacheService` for concurrency (races) and version-change behavior.
-   [ ] Adapter tests for `EnrichWithCache(...)` usage accounting and fallback.
-   [ ] Integration test: `dynamic_config_update` during non-final interim text performs flush, swaps config + cache, and resumes correctly.
-   [ ] Performance/regression test validating token savings across repeated calls with stable tools.

## Implementation Notes

-   Provider API usage should be centralized in infra; domain remains provider-agnostic.
-   Avoid unbounded conversation growth in `GeminiSession`. Prefer stateless calls with cached static context and only the current user prompt (or limited rolling context). Consider periodic truncation if conversation is kept.
-   Cache TTL and invalidation policy must account for multiple concurrent sessions possibly sharing identical static context (same checksum). Optional optimization: deduplicate by version across sessions and reuse the same provider cache entry when safe.

---

This document will be updated as the above TODOs are addressed and implementation details (especially around provider SDK support for cached-content invocation) are finalized.
