# 🧠 LLM Context Caching Architecture

## 📌 Purpose

The LLM Context Caching system dramatically reduces token costs by separating static expensive content (function tools and system instructions) from dynamic cheap content (transcript snippets). This is critical for real-time audio parsing where the same tools and instructions are repeatedly sent with every LLM request, consuming the majority of input tokens.

**Problem**: Growing conversation history prevents Google's automatic caching from working, forcing us to resend expensive static content on every request.

**Solution**: Manual context caching following hexagonal architecture principles for provider-agnostic implementation.

---

## 🏗️ Hexagonal Architecture Design

The caching system maintains strict layer separation following Schma's architectural principles:

```
internal/
├── domain/speech/          # Cache ports & contracts
│   ├── llm_cache_port.go  # LLMCache interface
│   └── llm_port.go        # Enhanced CachedLLM interface
├── app/services/           # Cache orchestration
│   └── llm_cache_service.go # Session-level cache management
├── infra/llmgemini/       # Google-specific implementation
│   ├── cache.go           # Implements LLMCache port
│   └── session.go         # Enhanced with cache support
└── transport/ws/          # Thin integration layer
    └── handler.go         # Cache invalidation triggers
```

---

## 🔁 Cache Lifecycle Flow

### Session Start & Cache Creation

```
[Transport] WebSocket Connect
    │
    ▼
[App] LLMCacheService.PrepareCache()
    │ (orchestrates cache lifecycle)
    ▼
[Domain] LLMCache.Store(tools + system_guide)
    │ (defines what we need)
    ▼
[Infra] GeminiCache uploads to Google
    │ (Google-specific implementation)
    ▼
[Return] CacheKey for session
```

### LLM Request with Cache

```
[App] Pipeline receives transcript
    │
    ▼
[App] LLMCacheService.EnrichWithOptimalStrategy()
    │ (smart cache vs fallback logic)
    ▼
[Domain] CachedLLM.EnrichWithCache(cacheKey + transcript)
    │ (provider-agnostic interface)
    ▼
[Infra] Gemini API call with cached context reference
    │ (only dynamic content sent)
    ▼
[Return] Function calls + drastically reduced token usage
```

### Dynamic Config Update

```
[Transport] Config update received
    │
    ▼
[App] LLMCacheService.InvalidateCurrentCache()
    │ (orchestrates cache invalidation)
    ▼
[Domain] LLMCache.Invalidate(oldKey)
    │ (generic invalidation contract)
    ▼
[Infra] Delete cached content from Google
    │
    ▼
[App] LLMCacheService.PrepareCache(newConfig)
    │ (creates new cache with updated tools)
    ▼
[Return] New CacheKey for subsequent requests
```

---

## 📋 Implementation Status & TODO List

### ✅ Completed Tasks

-   **✅ Domain Port**: LLMCache interface with comprehensive error types and contracts

    -   `LLMCache` interface with Store, Get, Delete, Invalidate, IsValid, Clear, Close methods
    -   `CachedLLM` interface extending base LLM with EnrichWithCache method
    -   Specific error types: CacheUnavailable, CacheExpired, CacheCorrupt, etc.
    -   `StaticContext` structure for cacheable content

-   **✅ Infrastructure Implementation**: GeminiCache with all required methods

    -   Complete implementation of `speech.LLMCache` interface
    -   Google-specific cache upload/download logic
    -   TTL and expiration handling at infrastructure level
    -   Compile-time interface verification

-   **✅ Session Cache Method**: CallFunctionsWithCache method added to GeminiSession

    -   `CallFunctionsWithCache` method for cached LLM calls
    -   Usage tracking and logging for cached requests
    -   Same response processing as regular function calls

-   **✅ Adapter Enhancement**: CachedLLM interface implementation

    -   `Adapter` now implements both `speech.LLM` and `speech.CachedLLM`
    -   `EnrichWithCache` method for cached enrichment calls
    -   Backward compatibility maintained with existing LLM interface

-   **✅ Checksum Integration**: Extended existing checksum utility for performance

    -   `ComputeContextChecksum` function for tools + system guide hashing
    -   Reuses existing JSON marshaling and SHA256 logic
    -   Performance-optimized with single checksum calculation

-   **✅ Documentation**: Comprehensive architecture documentation completed
    -   Hexagonal architecture design patterns
    -   Cache lifecycle flows and integration points
    -   Performance impact analysis and success metrics

### 🔄 In Progress

-   **🔄 App Service Creation**: LLMCacheService for session-level cache orchestration
    -   Service structure defined in `app/llmcache/llm_cache_service.go`
    -   Cache lifecycle management implemented
    -   Needs integration testing and pipeline wiring

### ⏳ Pending High Priority

-   **⏳ Cache API Integration**: Implement proper Gemini cached content API

    -   Currently using fallback approach due to SDK version limitations
    -   Need to integrate with latest Gemini SDK for `CachedContent` support
    -   Update `CallFunctionsWithCache` to use proper cache reference API

-   **⏳ Pipeline Integration**: Wire cache service into pipeline processing

    -   Modify `pipeline.go` to use `LLMCacheService.EnrichWithOptimalStrategy()`
    -   Add cache preparation logic to pipeline initialization
    -   Ensure graceful fallback when caching fails

-   **⏳ WebSocket Integration**: Add cache invalidation to dynamic config updates
    -   Integrate cache invalidation into `ws/handler.go` config update flow
    -   Ensure cache is refreshed when function schemas change via dynamic watcher
    -   Add cache invalidation triggers to config update acknowledgments

### ⏳ Pending Medium Priority

-   **⏳ Dependency Injection**: Wire up all cache components in main.go

    -   Create `GeminiCache` instance with proper client configuration
    -   Initialize `LLMCacheService` with cache and LLM dependencies
    -   Connect to existing LLM adapter and update dependency graph

-   **⏳ Testing**: Add comprehensive test coverage
    -   Unit tests for domain contracts and error handling
    -   Integration tests for cache lifecycle and invalidation
    -   End-to-end tests for token cost reduction validation
    -   Performance benchmarks comparing cached vs non-cached calls

### ⏳ Pending Low Priority

-   **⏳ Advanced Features**: Enhanced cache management capabilities
    -   Cache warming for common schema combinations
    -   Cache analytics and hit/miss rate tracking
    -   Multi-tenant cache isolation and namespacing
    -   Cache compression and size optimization

---

## 🎯 Success Metrics

### Performance Targets

-   **Cost Reduction**: 80-90% token cost reduction for typical sessions with 10+ LLM calls
-   **Latency Improvement**: 20-30% faster LLM response times due to reduced payload size
-   **Cache Hit Rate**: >90% cache hit rate for sessions longer than 5 minutes

### Architecture Quality

-   **Clean Separation**: Strict hexagonal architecture compliance maintained
-   **Provider Flexibility**: Easy swapping between LLM providers (Gemini → OpenAI/Anthropic)
-   **Error Resilience**: Graceful degradation when caching fails (100% fallback success)
-   **Type Safety**: Compile-time interface verification prevents runtime errors

### Operational Metrics

-   **Cache Efficiency**: <1% cache invalidation rate during normal operations
-   **Memory Usage**: Minimal additional memory footprint for cache management
-   **API Compatibility**: Zero breaking changes to existing LLM interface usage

---

## 🚀 Next Implementation Steps

1. **Immediate** (Sprint Goal): Complete `LLMCacheService` integration testing
2. **Week 1**: Implement proper Gemini cached content API integration
3. **Week 2**: Wire cache service into pipeline and WebSocket handlers
4. **Week 3**: Complete dependency injection and end-to-end testing
5. **Week 4**: Performance validation and optimization

---

## 🔮 Future Considerations
