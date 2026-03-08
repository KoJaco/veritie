## LLM Service Migration Plan: google.golang.org/genai

This document describes the refactor to migrate our LLM integration from the deprecated `github.com/google/generative-ai-go/genai` SDK to the new `google.golang.org/genai` SDK. The goal is to unblock context caching and ensure long-term compatibility while maintaining our domain contracts.

### Objectives

-   Adopt `google.golang.org/genai` across the LLM integration.
-   Maintain domain stability (no breaking changes to `internal/domain/speech` types and ports).
-   Preserve and improve caching behavior with the new cached-content API.
-   Preserve usage extraction and enhance token-savings telemetry.

## Scope

-   Files primarily impacted:

    -   `internal/infra/llmgemini/session.go`
    -   `internal/infra/llmgemini/adapter.go`
    -   `internal/infra/llmgemini/cache.go`
    -   `internal/app/prompts/*` (system instructions formatting/placement)
    -   `internal/app/llmcache/llm_cache_service.go` (integration only)

-   Files expected to remain stable:
    -   `internal/domain/speech/*` (ports and models)
    -   `internal/app/pipeline/*`
    -   `internal/app/usage/*`

## SDK Differences to Account For

-   Client/model initialization changes

    -   Old: `genai.NewClient(ctx, option.WithAPIKey(...))`, `client.GenerativeModel("gemini-2.0-flash")`
    -   New: `genai.NewClient(ctx, &genai.ClientConfig{ APIKey: ... })`, updated model accessors (confirm exact API)

-   Content/request construction

    -   New content/parts types
    -   System instructions likely configured via dedicated fields instead of embedding as user content

-   Tools/function calling configuration

    -   New structs/enums for function declarations and function-calling modes

-   Responses

    -   Function call results and usage metadata fields have changed names/shapes
    -   Prefer native function-call extraction; fallback to text/JSON if needed

-   Caching (context cache)
    -   New create/get/delete cached-content APIs
    -   New way to reference cached content during generation (model options/parameters)

## Refactor Plan (Phased)

### Phase 1: Baseline compilation on new SDK (uncached path)

-   Replace imports in `internal/infra/llmgemini/session.go`:
    -   Use `google.golang.org/genai`
    -   Initialize client using the new API
    -   Acquire model appropriately
-   Update request build to new content types
-   Parse responses to extract either function calls or JSON fallback
-   Map new usage metadata to `speech.LLMUsage{ Prompt, Completion }`
-   Update `internal/infra/llmgemini/adapter.go` `Enrich(...)` to work with the new session

Acceptance criteria:

-   Code compiles; existing unit tests (accumulator) pass; adapter builds

### Phase 2: Tools and system instructions

-   Update `convertDefs` to produce new `genai.FunctionDeclaration` values
-   Configure tools and function-calling mode on the model using new structs/enums
-   Configure system instructions using the recommended fields (not user content)

Acceptance criteria:

-   Tools are visible in model configuration (debug logs), and no runtime errors on first call

### Phase 3: Cached content integration

-   Implement `Store`/`Get`/`Invalidate` in `internal/infra/llmgemini/cache.go` using new cached-content API
-   In `session.CallFunctionsWithCache`, invoke generation with the cached-content reference using SDK-supported options
-   Keep graceful fallback to uncached generation if the cache cannot be used
-   Ensure `LLMUsage.Cached = true` on cached path; usage metadata still captured

Acceptance criteria:

-   Cached calls work (manual test), and fallback works if cache is unavailable

### Phase 4: Wire into app cache service

-   `internal/app/llmcache/llm_cache_service.go` should require no API change; confirm cache keys flow end-to-end
-   Confirm dynamic config watcher triggers cache refresh via the service

Acceptance criteria:

-   Dynamic config update hot-swaps tools/system and refreshes cached content without reconnect

### Phase 5: Tests and verification

-   Unit tests: adapter/session function-call parsing with synthetic responses
-   Verify accumulator tests still pass; add any new LLM mocks if useful
-   Manual integration: run server and validate with frontend

## Domain and Contracts

-   Preserve `speech` ports and types:
    -   `LLM.Enrich(...)`, `CachedLLM.EnrichWithCache(...)`
    -   `FunctionDefinition`, `FunctionCall`, `FunctionConfig`, `LLMUsage`
-   Only adjust mappers/utilities that translate domain → provider types

## Risks and Mitigations

-   Function-calling config mismatch

    -   Mitigation: read SDK docs; assert/compile-time checks; add debug logs for tools config

-   Response parsing changes

    -   Mitigation: guard against empty candidates; fallback to JSON parsing; unit tests

-   Cached-content invocation API mismatch
    -   Mitigation: behind a safe feature toggle and fallback to uncached, log metrics

## Metrics and Observability

-   Preserve existing counters/summaries in `internal/pkg/metrics`
-   Add labels for `provider=gemini`, `cached=true|false`, and failures per stage (store, validate, enrich)
-   Track token savings via `LLMUsage` and accumulator as implemented

## Rollout Steps

1. Implement Phase 1 and build
2. Implement Phase 2 and run basic manual verification
3. Implement Phase 3 and verify cache flows; fallback tested
4. Run unit tests; fix any regressions
5. Manual end-to-end via frontend

## Backout Plan

-   If the new SDK path blocks progress, re-enable the old code behind build tags or a provider switch while completing migration

## Checklist

-   [ ] Session client/model init migrated to `google.golang.org/genai`
-   [ ] Tools + system instruction wiring updated
-   [ ] Response parsing updated; function calls extracted
-   [ ] Usage metadata mapped to `LLMUsage`
-   [ ] Cached-content store/get/invalidate implemented
-   [ ] Cached-content invocation wired with fallback
-   [ ] Unit tests for adapter/session parsing
-   [ ] Manual integration verified
