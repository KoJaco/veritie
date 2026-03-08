package services

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"schma.ai/internal/app/checksum"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/metrics"
)

type LLMCacheService struct {
    cache    speech.LLMCache
    llm      speech.CachedLLM
    provider string // Provider identifier (do not prefix cache keys with this)
    
    // Session-scoped cache state
    mu          sync.RWMutex
    currentKey  speech.CacheKey
    lastVersion string
    failedVersion string // version that failed creation (e.g., below provider min); skip reattempts until version changes
}

func NewLLMCacheService(cache speech.LLMCache, llm speech.CachedLLM, provider string) *LLMCacheService {
    return &LLMCacheService{
        cache:    cache,
        llm:      llm,
        provider: provider,
    }
}

// App layer orchestration logic
func (s *LLMCacheService) PrepareCache(ctx context.Context, cfg *speech.FunctionConfig) (speech.CacheKey, error) {
    // Calculate version checksum using existing infrastructure
    version, err := checksum.ComputeFunctionsContextChecksum(cfg.Declarations)
    if err != nil {
        return "", err
    }
    
    // Thread safety
    s.mu.RLock()
    sameVersion := (version == s.lastVersion)
    failedVersion := s.failedVersion
    s.mu.RUnlock()

    // If a previous attempt for this exact version failed due to provider constraints, skip reattempts
    if version == failedVersion {
        metrics.IncCounter("llm_cache_skip_failed_version_total", metrics.Labels{"provider": s.provider}, 1)
        return "", speech.NewCacheUnavailableError(nil)
    }

    // Check if we need to update cache
    if !sameVersion {
        // Invalidate old cache
        s.mu.RLock()
        oldKey := s.currentKey
        s.mu.RUnlock()
        if oldKey != "" {
            if err := s.cache.Invalidate(ctx, oldKey); err == nil {
                metrics.IncCounter("llm_cache_invalidate_success_total", metrics.Labels{"provider": s.provider}, 1)
            } else {
                metrics.IncCounter("llm_cache_invalidate_failure_total", metrics.Labels{"provider": s.provider}, 1)
            }
        }
        
        // Create new cache with pre-calculated version
        static := speech.NewStaticContextWithVersion(cfg.Declarations, cfg.ParsingGuide, version)
        
        rawKey, err := s.cache.Store(ctx, static)
        if err != nil {
            metrics.IncCounter("llm_cache_store_failure_total", metrics.Labels{"provider": s.provider}, 1)
            // If provider indicated the cache is unavailable (e.g., too small), remember this version to avoid reattempts
            if ce, ok := err.(*speech.CacheError); ok && ce.Type == speech.CacheUnavailable {
                s.mu.Lock()
                s.failedVersion = version
                s.mu.Unlock()
            }
            return "", err
        }
        metrics.IncCounter("llm_cache_store_success_total", metrics.Labels{"provider": s.provider}, 1)
        
        s.mu.Lock()
        s.currentKey = rawKey // keep raw provider key; do not prefix
        s.lastVersion = version
        // Clear failedVersion on success
        if s.failedVersion == version {
            s.failedVersion = ""
        }
        s.mu.Unlock()
    }
    
    s.mu.RLock()
    key := s.currentKey
    s.mu.RUnlock()
    return key, nil
}

func (s *LLMCacheService) EnrichWithOptimalStrategy(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
    // Try cached approach first
    cacheKey, err := s.PrepareCache(ctx, cfg)
    if err == nil && cacheKey != "" {
        metrics.IncCounter("llm_cache_attempt_total", metrics.Labels{"provider": s.provider}, 1)
        calls, usage, err := s.llm.EnrichWithCache(ctx, cacheKey, prompt, partial, cfg)
        if err == nil {
            metrics.IncCounter("llm_cache_hit_total", metrics.Labels{"provider": s.provider}, 1)
        } else {
            metrics.IncCounter("llm_cache_error_total", metrics.Labels{"provider": s.provider, "stage": "cached_enrich"}, 1)
        }
        return calls, usage, err
    }
    
    // Graceful fallback to traditional approach
    metrics.IncCounter("llm_cache_fallback_total", metrics.Labels{"provider": s.provider, "reason": reason(err)}, 1)
    return s.llm.Enrich(ctx, prompt, partial, cfg)
}

// reason maps an error to a short label for metrics.
func reason(err error) string {
    if err == nil {
        return "invalid_or_disabled"
    }
    return "error"
}

// NewLLMShim adapts LLMCacheService to speech.LLM so existing pipeline code can use caching transparently.
type LLMShim struct{ svc *LLMCacheService }

func NewLLMShim(svc *LLMCacheService) *LLMShim { return &LLMShim{svc: svc} }

// Enrich routes to the optimal strategy (cached first, fallback to normal).
func (s *LLMShim) Enrich(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
    return s.svc.EnrichWithOptimalStrategy(ctx, prompt, partial, cfg)
}

// PrepareCache exposes proactive cache creation for dynamic config updates
func (s *LLMShim) PrepareCache(ctx context.Context, cfg *speech.FunctionConfig) (speech.CacheKey, error) {
    return s.svc.PrepareCache(ctx, cfg)
}

// PrepareStructuredCache implements structured output caching
func (s *LLMCacheService) PrepareStructuredCache(ctx context.Context, cfg *speech.StructuredConfig) (speech.CacheKey, error) {
    // Calculate version checksum using structured context infrastructure
    schemaBytes, _ := json.Marshal(cfg.Schema)
    version, err := checksum.ComputeStructuredContextChecksum(cfg.Schema)
    if err != nil {
        return "", err
    }
    
    // Thread safety
    s.mu.RLock()
    sameVersion := (version == s.lastVersion)
    failedVersion := s.failedVersion
    s.mu.RUnlock()

    // If a previous attempt for this exact version failed due to provider constraints, skip reattempts
    if version == failedVersion {
        metrics.IncCounter("llm_cache_skip_failed_version_total", metrics.Labels{"provider": s.provider, "mode": "structured"}, 1)
        return "", speech.NewCacheUnavailableError(nil)
    }

    // Check if we need to update cache
    if !sameVersion {
        // Invalidate old cache
        s.mu.RLock()
        oldKey := s.currentKey
        s.mu.RUnlock()
        if oldKey != "" {
            if err := s.cache.Invalidate(ctx, oldKey); err == nil {
                metrics.IncCounter("llm_cache_invalidate_success_total", metrics.Labels{"provider": s.provider, "mode": "structured"}, 1)
            } else {
                metrics.IncCounter("llm_cache_invalidate_failure_total", metrics.Labels{"provider": s.provider, "mode": "structured"}, 1)
            }
        }
        
        // Check if we have a structured cache preparer
        if structuredCache, ok := s.cache.(interface {
            StoreStructured(ctx context.Context, schemaJSON []byte, guide string) (speech.CacheKey, error)
        }); ok {
            // Create cache with schema and guide
            rawKey, err := structuredCache.StoreStructured(ctx, schemaBytes, cfg.ParsingGuide)
            if err != nil {
                metrics.IncCounter("llm_cache_store_failure_total", metrics.Labels{"provider": s.provider, "mode": "structured"}, 1)
                // If provider indicated the cache is unavailable (e.g., too small), remember this version to avoid reattempts
                if ce, ok := err.(*speech.CacheError); ok && ce.Type == speech.CacheUnavailable {
                    s.mu.Lock()
                    s.failedVersion = version
                    s.mu.Unlock()
                }
                return "", err
            }
            metrics.IncCounter("llm_cache_store_success_total", metrics.Labels{"provider": s.provider, "mode": "structured"}, 1)
            
            s.mu.Lock()
            s.currentKey = rawKey // keep raw provider key; do not prefix
            s.lastVersion = version
            // Clear failedVersion on success
            if s.failedVersion == version {
                s.failedVersion = ""
            }
            s.mu.Unlock()
        } else {
            // Fallback: cache not available for structured mode
            return "", speech.NewCacheUnavailableError(errors.New("structured caching not supported by cache provider"))
        }
    }
    
    s.mu.RLock()
    key := s.currentKey
    s.mu.RUnlock()
    return key, nil
}

// GenerateStructured forwards to the underlying LLM if it supports structured generation.
// The cache layer is currently functions-only, so this is a straight passthrough.
func (s *LLMShim) GenerateStructured(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error) {
    if ll, ok := s.svc.llm.(speech.StructuredLLM); ok {
        return ll.GenerateStructured(ctx, prompt, partial, cfg)
    }
    return nil, nil, errors.New("structured generation not supported by underlying LLM")
}

// PrepareStructuredCache implements structured output caching
func (s *LLMShim) PrepareStructuredCache(ctx context.Context, cfg *speech.StructuredConfig) (speech.CacheKey, error) {
    return s.svc.PrepareStructuredCache(ctx, cfg)
}

// GenerateStructuredWithOptimalStrategy implements cached structured generation
func (s *LLMShim) GenerateStructuredWithOptimalStrategy(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.StructuredConfig) (map[string]any, *speech.LLMUsage, error) {
    // Try cached approach first
    cacheKey, err := s.PrepareStructuredCache(ctx, cfg)
    if err == nil && cacheKey != "" {
        metrics.IncCounter("llm_cache_attempt_total", metrics.Labels{"provider": s.svc.provider, "mode": "structured"}, 1)
        if cachedLLM, ok := s.svc.llm.(speech.CachedStructuredLLM); ok {
            obj, usage, err := cachedLLM.GenerateStructuredWithCache(ctx, cacheKey, prompt, partial, cfg)
            if err == nil {
                metrics.IncCounter("llm_cache_hit_total", metrics.Labels{"provider": s.svc.provider, "mode": "structured"}, 1)
            } else {
                metrics.IncCounter("llm_cache_error_total", metrics.Labels{"provider": s.svc.provider, "mode": "structured", "stage": "cached_generate"}, 1)
            }
            return obj, usage, err
        }
    }
    
    // Graceful fallback to traditional approach
    metrics.IncCounter("llm_cache_fallback_total", metrics.Labels{"provider": s.svc.provider, "mode": "structured", "reason": reason(err)}, 1)
    if structuredLLM, ok := s.svc.llm.(speech.StructuredLLM); ok {
        return structuredLLM.GenerateStructured(ctx, prompt, partial, cfg)
    }
    return nil, nil, errors.New("structured generation not supported by underlying LLM")
}

// GetUnderlyingLLM returns the underlying LLM service for session updates
func (s *LLMShim) GetUnderlyingLLM() speech.LLM {
    return s.svc.llm
}

