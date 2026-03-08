//go:build genai2

package llmgemini

import (
	"context"
	"errors"
	"strings"
	"time"

	g2 "google.golang.org/genai"

	"schma.ai/internal/app/prompts"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/pkg/metrics"
)

// isExpectedCacheError checks if a cache error is expected (e.g., content too small)
func isExpectedCacheError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for expected cache errors that don't need error logs
	return strings.Contains(errStr, "too small") || 
		   strings.Contains(errStr, "min_total_token_count") ||
		   strings.Contains(errStr, "INVALID_ARGUMENT")
}

// Compile-time check for interface conformance (when tag enabled)
var _ speech.LLMCache = (*GeminiCacheV2)(nil)

type GeminiCacheV2 struct {
    client    *g2.Client
    modelName string
}

func NewGeminiCacheV2(client *g2.Client, modelName string) *GeminiCacheV2 {
    return &GeminiCacheV2{client: client, modelName: modelName}
}

// Store uploads tools + system guide as cached content and returns the cache key
func (c *GeminiCacheV2) Store(ctx context.Context, static speech.StaticContext) (speech.CacheKey, error) {
    sys := prompts.BuildFunctionsSystemInstructionPrompt(static.SystemParsingGuide)
    // Provide system text via SystemInstruction field only; use empty role to avoid unsupported "system" role in contents
    system := g2.NewContentFromText(sys, g2.Role(""))
    decls := convertDefsV2(static.Tools)

    // Provider min token count for cached content (confirmed 1024)
    // Fast preflight: if below minimum, do not attempt provider call; signal unavailable
    estimated := estimateStaticTokens(sys, static.Tools)
    const minCacheTokens = 1024
    if estimated < minCacheTokens {
        logger.Warnf("[LLM v2] cache.Store skipped: estimated_tokens=%d < min=%d (decls=%d system_len=%d)", estimated, minCacheTokens, len(decls), len(sys))
        return "", speech.NewCacheUnavailableError(errors.New("cache seed below provider minimum; skipping cache creation"))
    }

    logger.Debugf("[LLM v2] cache.Store model=%s decls=%d system_len=%d version=%s",
        c.modelName, len(decls), len(sys), static.Version)

	
    cfg := &g2.CreateCachedContentConfig{
        SystemInstruction: system,
        Tools:             []*g2.Tool{{FunctionDeclarations: decls}},
        // Must include ToolConfig here because CachedContent requests cannot set it
        ToolConfig: &g2.ToolConfig{
            FunctionCallingConfig: &g2.FunctionCallingConfig{Mode: g2.FunctionCallingConfigMode("ANY")},
        },
        // TTL: time.Hour * 24,
    }


    // Defensive timeout for cache create to avoid blocking request path
    reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    start := time.Now()
    created, err := c.client.Caches.Create(reqCtx, c.modelName, cfg)
    metrics.ObserveSummary("llm_cache_store_ms", metrics.Labels{"provider": "gemini"}, time.Since(start).Milliseconds())
    if err != nil {
        // Only log as error if it's not an expected "too small" error
        if isExpectedCacheError(err) {
            logger.ServiceDebugf("LLM", "cache.Store skipped (content too small): %v", err)
        } else {
            logger.Errorf("[LLM v2] cache.Store failed: %v", err)
        }
        return "", speech.NewCacheUnavailableError(err)
    }
    if created == nil || created.Name == "" {
        logger.Errorf("[LLM v2] cache.Store returned empty name: %+v", created)
        return "", speech.NewCacheInvalidError(errors.New("empty cached content name"))
    }
    logger.ServiceDebugf("LLM", "cache.Store ok name=%s took=%dms", created.Name, time.Since(start).Milliseconds())
    return speech.CacheKey(created.Name), nil
}

func (c *GeminiCacheV2) Invalidate(ctx context.Context, key speech.CacheKey) error {
    start := time.Now()
    logger.ServiceDebugf("LLM", "cache.Invalidate name=%s", string(key))
    _, err := c.client.Caches.Delete(ctx, string(key), &g2.DeleteCachedContentConfig{})
    metrics.ObserveSummary("llm_cache_invalidate_ms", metrics.Labels{"provider": "gemini"}, time.Since(start).Milliseconds())
    if err != nil {
        logger.Errorf("[LLM v2] cache.Invalidate failed: %v", err)
        return speech.NewCacheInvalidationFailedError(err)
    }
    logger.ServiceDebugf("LLM", "cache.Invalidate ok name=%s took=%dms", string(key), time.Since(start).Milliseconds())
    return nil
}

func (c *GeminiCacheV2) IsValid(ctx context.Context, key speech.CacheKey) bool {
    start := time.Now()
    logger.Debugf("[LLM v2] cache.IsValid name=%s", string(key))
    _, err := c.client.Caches.Get(ctx, string(key), &g2.GetCachedContentConfig{})
    metrics.ObserveSummary("llm_cache_validate_ms", metrics.Labels{"provider": "gemini"}, time.Since(start).Milliseconds())
    return err == nil
}

func (c *GeminiCacheV2) Get(ctx context.Context, key speech.CacheKey) (speech.StaticContext, error) {
    logger.Debugf("[LLM v2] cache.Get name=%s", string(key))
    _, err := c.client.Caches.Get(ctx, string(key), &g2.GetCachedContentConfig{})
    if err != nil {
        return speech.StaticContext{}, speech.NewCacheMissError(err)
    }
    // We cannot reconstruct full StaticContext from provider; return a stub with version as key
    return speech.StaticContext{Version: string(key)}, nil
}

func (c *GeminiCacheV2) Delete(ctx context.Context, key speech.CacheKey) error {
    return c.Invalidate(ctx, key)
}

func (c *GeminiCacheV2) Clear() error { return speech.NewCacheUnavailableError(errors.New("bulk clear not supported by Google cache")) }
func (c *GeminiCacheV2) Close() error { return nil }
func (c *GeminiCacheV2) IsAvailable() bool { return c.client != nil }
func (c *GeminiCacheV2) IsCorrupt() bool { return false }
func (c *GeminiCacheV2) IsExpired() bool { return false }

// estimateStaticTokens provides a cheap, deterministic token estimate for cache gating.
// We approximate tokens as characters/4 for English and add a small overhead per tool field.
func estimateStaticTokens(system string, tools []speech.FunctionDefinition) int {
    // Base from system text
    tokens := len(system) / 4
    // Add approximate overhead for each tool and its parameters
    for _, d := range tools {
        tokens += len(d.Name)/4 + len(d.Description)/4 + 8
        for _, p := range d.Parameters {
            tokens += len(p.Name)/4 + len(p.Type)/4 + len(p.Description)/4 + 4
        }
    }
    // Safety margin for JSON/structural tokens
    tokens += 128
    return tokens
}

// StoreStructured caches a structured-output static context (schema + guide) for v2 SDK.
// Uses the same pattern as functions: build system instruction prompt from guide + temporal data.
func (c *GeminiCacheV2) StoreStructured(ctx context.Context, schemaJSON []byte, guide string) (speech.CacheKey, error) {
    if c.client == nil {
        return "", speech.NewCacheUnavailableError(errors.New("nil client"))
    }
    
    // Build system instruction prompt using the same pattern as functions
    sys := prompts.BuildStructuredSystemInstructionPrompt(guide)
    
    // Add schema to the system instruction (this is the key difference from functions)
    fullSys := sys + "\n\nJSON Schema:\n" + string(schemaJSON)
    

    // Estimate tokens; skip below provider minimum
    estimated := (len(fullSys) / 4) + 128
    
    // Model-specific cache minimums based on Google's documentation
    var minCacheTokens int
    if strings.Contains(c.modelName, "2.0") {
        minCacheTokens = 4096 // Gemini 2.0 Flash requires 4096 tokens for explicit caching
    } else {
        minCacheTokens = 1024 // Gemini 2.5 Flash and other models (optimized for better caching)
    }
    
    if estimated < minCacheTokens {
        logger.Warnf("[LLM v2] cache.StoreStructured skipped: estimated_tokens=%d < min=%d (model=%s schema_len=%d guide_len=%d)", 
            estimated, minCacheTokens, c.modelName, len(schemaJSON), len(guide))
        return "", speech.NewCacheUnavailableError(errors.New("cache seed below provider minimum; skipping cache creation"))
    }

    logger.Debugf("[LLM v2] cache.StoreStructured model=%s schema_len=%d guide_len=%d", c.modelName, len(schemaJSON), len(guide))

    cfg := &g2.CreateCachedContentConfig{
        SystemInstruction: g2.NewContentFromText(fullSys, g2.Role("")),
        // Note: no tools for structured mode
    }

    reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    start := time.Now()
    created, err := c.client.Caches.Create(reqCtx, c.modelName, cfg)
    metrics.ObserveSummary("llm_cache_store_ms", metrics.Labels{"provider": "gemini", "mode": "structured"}, time.Since(start).Milliseconds())
    if err != nil {
        logger.Errorf("[LLM v2] cache.StoreStructured failed: %v", err)
        return "", speech.NewCacheUnavailableError(err)
    }
    if created == nil || created.Name == "" {
        logger.Errorf("[LLM v2] cache.StoreStructured returned empty name: %+v", created)
        return "", speech.NewCacheInvalidError(errors.New("empty cached content name"))
    }
    logger.ServiceDebugf("LLM", "cache.StoreStructured ok name=%s took=%dms", created.Name, time.Since(start).Milliseconds())
    return speech.CacheKey(created.Name), nil
}


