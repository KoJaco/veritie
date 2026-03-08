//go:build genai2

package llmprovider

import (
	services "schma.ai/internal/app/llmcache"
	"schma.ai/internal/domain/speech"
)

// ProvideLLM returns the v2 LLM shim (cache-aware) under genai2 build tag.
func ProvideLLM(apiKey, model string) (speech.LLM, error) {
    shim, err := services.NewV2Shim(apiKey, model)
    if err != nil {
        return nil, err
    }
    return shim, nil
}


