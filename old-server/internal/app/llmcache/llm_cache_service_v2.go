//go:build genai2

package services

import (
	v2 "schma.ai/internal/infra/llmgemini"
)

// NewV2Shim wires the v2 cache + adapter under the same service surface
func NewV2Shim(apiKey, model string) (*LLMShim, error) {
    adapter := v2.NewV2(apiKey, model)
    // Build a temporary session to get a client for the cache
    sess, err := v2.NewSessionV2(apiKey, model)
    if err != nil {
        return nil, err
    }
    cache := v2.NewGeminiCacheV2(sess.Client(), model)
    svc := NewLLMCacheService(cache, adapter, "gemini")
    return NewLLMShim(svc), nil
}


