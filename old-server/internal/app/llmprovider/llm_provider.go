//go:build !genai2

package llmprovider

import (
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/infra/llmgemini"
)

// ProvideLLM returns the default (v1) LLM adapter when genai2 is not enabled.
func ProvideLLM(apiKey, model string) (speech.LLM, error) {
    return llmgemini.New(apiKey, model), nil
}


