package speech

import (
	"context"
	"encoding/json"
)

type Prompt string
// TODO: Large refactor needed here... must support array items...
// TODO: Post MVP we should start supporting more complex tool parameters and function calling https://pkg.go.dev/google.golang.org/genai#Schema
// FunctionParam mirrors and OpenAI / Gemini tool param
type FunctionParam struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "string", "number", "boolean" …
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// TODO: Large refactor pending, need to change Parameters type to GenaiSchema
// Fuck
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  []FunctionParam `json:"parameters"`
}

type FunctionCall struct {
	ID              string                 `json:"id,omitempty"`
	Name            string                 `json:"name"`
	Args            map[string]interface{} `json:"args"`
	SimilarityScore float64                `json:"similarity_score,omitempty"`
}

type GenaiSchema struct {
	Type string `json:"type"` // "string" | "number" | "boolean" | "object" | "unspecified" | "integer" | "array"
	Description string `json:"description,omitempty"`
	Format string `json:"format,omitempty"`
	Nullable bool `json:"nullable,omitempty"`
	Enum []string `json:"enum,omitempty"`
	Items *GenaiSchema `json:"items,omitempty"`
	Properties map[string]GenaiSchema `json:"properties,omitempty"`
	Required []string `json:"required,omitempty"`
}

type StructuredOutputSchema struct {
	Name string `json:"name"`	
	Description string `json:"description,omitempty"`
	Type string `json:"type"` // "object"
	AdditionalProperties bool `json:"additionalProperties,omitempty"`
	Properties map[string]GenaiSchema `json:"properties,omitempty"`
	Required []string `json:"required,omitempty"`
}

type StructuredOutputUpdate struct {
	Rev int
	Delta map[string]any
	Final map[string]any
}

type TranscriptInclusionPolicy struct {
	TranscriptMode string `json:"transcript_mode,omitempty"` // "full" or "window"
	WindowTokenSize int `json:"window_token_size,omitempty"` // In tokens
	TailSentences int `json:"tail_sentences,omitempty"` // In sentences (continuity before NEW)
}

type PrevOutputInclusionPolicy struct {
	PrevOutputMode string `json:"prev_output_mode,omitempty"` // "apply" or "ignore" or "keys-only" or "window"
}

type ParsingConfig struct {
	ParsingStrategy string                    `json:"parsing_strategy,omitempty"` // "auto", "update-ms", "after-silence" or "end-of-session", "manual"
	TranscriptInclusionPolicy TranscriptInclusionPolicy `json:"transcript_inclusion_policy,omitempty"`
	PrevOutputInclusionPolicy PrevOutputInclusionPolicy `json:"prev_output_inclusion_policy,omitempty"`
}

type FunctionConfigWithoutContext struct {
	Name     string                      `json:"name,omitempty"`     
	Description string                      `json:"description,omitempty"`    
	UpdateMs int                  `json:"update_frequency,omitempty"`
	Declarations      []FunctionDefinition // Model's Tools list
	ParsingGuide      string               // free-text system guide
	ParsingConfig     ParsingConfig        // Parsing configuration
}

// TODO: Update SDK to reflect this.
type FunctionConfig struct {
	Name     string                      `json:"name,omitempty"`     
	Description string                      `json:"description,omitempty"`    
	UpdateMs int                  `json:"update_frequency,omitempty"`
	Declarations      []FunctionDefinition // Model's Tools list
	ParsingGuide      string               // free-text system guide
	PrevContext       []FunctionCall       // Earlier calls outputted by LLM or parsed into LLM output
	ParsingConfig     ParsingConfig        // Parsing configuration
}

type StructuredOutputConfig struct {
	UpdateMs int `json:"update_frequency,omitempty"`
	Schema StructuredOutputSchema
	ParsingGuide string
	ParsingConfig ParsingConfig
}

type LLMUsage struct{
    Prompt             int64
    Completion         int64
    // Optional enrichment when cached path is used
    SavedPromptTokens  int64
    Cached             bool
}

type SessionSetter interface {
	SetSession(any)
}

type LLM interface {

	// Send redacted delta for context updates (streaming).
	// TODO: should we add redaction delta handling here as a method, or orchestrate using our pipeline? The prompt could include redaction details and the LLM struct doesn't need to be aware of this.
	Enrich(ctx context.Context, prompt Prompt, partial Transcript, cfg *FunctionConfig) ([]FunctionCall, *LLMUsage, error)
}

// Optional extension for implementations that support proactive cache preparation
type CachePreparer interface {
    PrepareCache(ctx context.Context, cfg *FunctionConfig) (CacheKey, error)
}

// Structured output configuration for schema-constrained JSON generations
type StructuredConfig struct {
    Schema       json.RawMessage // JSON Schema (draft-07+)
    ParsingGuide string          // system/guide text for structured extraction
    UpdateMS     int             // throttle window for realtime calls
}

// Optional interface for LLMs that support structured JSON output
type StructuredLLM interface {
    GenerateStructured(ctx context.Context, prompt Prompt, partial Transcript, cfg *StructuredConfig) (map[string]any, *LLMUsage, error)
}

// Optional interface for LLMs that support cached structured JSON output
type CachedStructuredLLM interface {
    StructuredLLM
    GenerateStructuredWithCache(ctx context.Context, cacheKey CacheKey, prompt Prompt, partial Transcript, cfg *StructuredConfig) (map[string]any, *LLMUsage, error)
}

// Optional extension for cache preparers to handle structured static context
type StructuredCachePreparer interface {
    PrepareStructuredCache(ctx context.Context, cfg *StructuredConfig) (CacheKey, error)
}
