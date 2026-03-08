package llmgemini

import (
	"context"
	"errors"
	"time"

	"github.com/google/generative-ai-go/genai"
	"schma.ai/internal/app/prompts"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/metrics"
)

// Compile-time check that GeminiCache implements speech.LLMCache
var _ speech.LLMCache = (*GeminiCache)(nil)

type GeminiCache struct {
    client    *genai.Client
    modelName string
}

func NewGeminiCache(client *genai.Client, modelName string) *GeminiCache {
    return &GeminiCache{
        client:    client,
        modelName: modelName,
    }
}

// Implements speech.LLMCache.Store
func (c *GeminiCache) Store(ctx context.Context, static speech.StaticContext) (speech.CacheKey, error) {
    // Build system message using app layer prompts
    systemMsg := prompts.BuildFunctionsSystemInstructionPrompt(static.SystemParsingGuide)
    
    // Convert domain tools to Gemini format
    genaiTools := convertDefs(static.Tools)
    
    // Upload to Google's cache endpoint
    cacheReq := &genai.CachedContent{
        Model: c.modelName,
        Tools: []*genai.Tool{{FunctionDeclarations: genaiTools}},
		Contents: []*genai.Content{{	
			Role: "system",
			Parts: []genai.Part{genai.Text(systemMsg)},
		}},
		Expiration:   genai.ExpireTimeOrTTL{TTL: time.Hour * 24}, // Infrastructure concern
    }
    
    start := time.Now()
    cached, err := c.client.CreateCachedContent(ctx, cacheReq)
    metrics.ObserveSummary("llm_cache_store_ms", metrics.Labels{"provider": "gemini"}, time.Since(start).Milliseconds())
    if err != nil {
        return "", speech.NewCacheUnavailableError(err)
    }
    
    return speech.CacheKey(cached.Name), nil
}

// Implements speech.LLMCache.Invalidate  
func (c *GeminiCache) Invalidate(ctx context.Context, key speech.CacheKey) error {
    start := time.Now()
    err := c.client.DeleteCachedContent(ctx, string(key))
    metrics.ObserveSummary("llm_cache_invalidate_ms", metrics.Labels{"provider": "gemini"}, time.Since(start).Milliseconds())
    if err != nil {
        return speech.NewCacheInvalidationFailedError(err)
    }
    return nil
}

// Implements speech.LLMCache.IsValid
func (c *GeminiCache) IsValid(ctx context.Context, key speech.CacheKey) bool {
    start := time.Now()
    _, err := c.client.GetCachedContent(ctx, string(key))
    metrics.ObserveSummary("llm_cache_validate_ms", metrics.Labels{"provider": "gemini"}, time.Since(start).Milliseconds())
    return err == nil
}

// Implements speech.LLMCache.Get
func (c *GeminiCache) Get(ctx context.Context, key speech.CacheKey) (speech.StaticContext, error) {
    _, err := c.client.GetCachedContent(ctx, string(key))
    if err != nil {
        return speech.StaticContext{}, speech.NewCacheMissError(err)
    }
    
    // Note: We can't reconstruct the full StaticContext from Google's cache
    // This method is mainly for validation. The actual content is used via cache reference.
    return speech.StaticContext{
        Version: string(key), // Use cache key as version identifier
    }, nil
}

// Implements speech.LLMCache.Delete
func (c *GeminiCache) Delete(ctx context.Context, key speech.CacheKey) error {
    return c.Invalidate(ctx, key) // Same operation for Google's cache
}

// Implements speech.LLMCache.Clear
func (c *GeminiCache) Clear() error {
    // Google doesn't provide a bulk clear operation
    // This would need to be implemented at a higher level if needed
    return speech.NewCacheUnavailableError(errors.New("bulk clear not supported by Google cache"))
}

// Implements speech.LLMCache.Close
func (c *GeminiCache) Close() error {
    // Google client doesn't need explicit closing
    return nil
}

// Implements speech.LLMCache.IsAvailable
func (c *GeminiCache) IsAvailable() bool {
    // Basic availability check - we could enhance this with a ping operation
    return c.client != nil
}

// Implements speech.LLMCache.IsCorrupt
func (c *GeminiCache) IsCorrupt() bool {
    // Google handles corruption internally, assume false
    return false
}

// Implements speech.LLMCache.IsExpired
func (c *GeminiCache) IsExpired() bool {
    // Google handles expiration internally, use IsValid for specific keys
    return false
}

