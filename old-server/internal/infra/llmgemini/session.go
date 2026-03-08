package llmgemini

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	// "google.golang.org/genai"

	"schma.ai/internal/app/prompts"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// Note: GeminiSession provides the underlying cache functionality
// The CachedLLM interface is implemented by the Adapter

type GeminiSession struct {
	client       *genai.Client
	model        *genai.GenerativeModel
	conversation []genai.Content // persistent convo context

	toolsSet   bool
    structuredSet bool
	systemMsgs []genai.Content

	lastUsage *genai.UsageMetadata
}

// var logOnce sync.Once

func (s *GeminiSession) LastUsage() *genai.UsageMetadata {
	return s.lastUsage
}

// Client exposes the underlying genai.Client for adapter wiring where needed.
func (s *GeminiSession) Client() *genai.Client {
    return s.client
}



// Inits a new GeminiSession, passing in required API key from env, setting timeout, handling errors. Returns the initialized GeminiSession and any errors
func NewSession(apiKey, modelName string) (*GeminiSession, error) {
	// 1. Init env with necessary config
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	// If it's still undefined, throw and log
	if apiKey == "" {
		return nil, errors.New("gemini API key missing in config")
	}

	// 2. Init context.. should we have timeout here?
	ctx := context.Background()

	// 3. Init the genai client with our background context and necessary api key
	cli, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))

	// simple http request that we should hit on audio_start ... not here then
	// TODO: Implement client.CreateContextCache(ctx, &genai.CreateContextCache{ Model: modelName, Contents: []genai.Content{systemContent, toolSchemaContent} })

	if err != nil {
		return nil, err
	}

	// 4. Set the model
	m := cli.GenerativeModel(modelName)

	// TODO: these should be LLM config variables
	m.SetTemperature(0.0) // deterministic but still a little creative
	m.SetTopP(0.2)
	m.SetTopK(20)

	return &GeminiSession{
		client: cli,
		model:  m,
	}, nil
}

func (s *GeminiSession) ConfigureOnce(defs []speech.FunctionDefinition, systemGuide string) {
	if s.toolsSet {
		return
	}

	genaiDecls := convertDefs(defs)
	s.model.Tools = []*genai.Tool{{
		FunctionDeclarations: genaiDecls,
	}}
	s.model.ToolConfig = &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingAny,
		},
	}

	LogTools(s.model.Tools, true)
	// logOnce.Do(func() { LogTools(s.model.Tools) })

	// Stsash system instructs
	sys := prompts.BuildFunctionsSystemInstructionPrompt(systemGuide)
	s.systemMsgs = []genai.Content{
		{Role: "system", Parts: []genai.Part{genai.Text(sys)}},
	}
	s.toolsSet = true
}



// ConfigureStructuredOnce configures the session for structured JSON output using a schema and guide.
// For MVP we embed the schema and guardrails into the system prompt to force JSON-only output.
func (s *GeminiSession) ConfigureStructuredOnce(schema json.RawMessage, systemGuide string) {
    if s.structuredSet {
        return
    }

    // Build a strict system instruction to return only JSON conforming to the provided schema
    sys := "You are a structured extraction engine. Return ONLY a single JSON object with no explanations. " +
        "It MUST conform to this JSON Schema. If a field cannot be determined, omit it.\n\nJSON Schema:\n" + string(schema) +
        "\n\nGuidance:\n" + systemGuide

    s.systemMsgs = []genai.Content{
        {Role: "system", Parts: []genai.Part{genai.Text(sys)}},
    }
    s.structuredSet = true
}

// CallStructured generates a single JSON object according to the configured structured system message.
func (s *GeminiSession) CallStructured(ctx context.Context, userPrompt string) (map[string]any, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    if len(s.conversation) == 0 && len(s.systemMsgs) > 0 {
        s.conversation = append(s.conversation, s.systemMsgs...)
    }

    user := genai.Content{Role: "user", Parts: []genai.Part{genai.Text(userPrompt)}}
    s.conversation = append(s.conversation, user)

    parts := []genai.Part{}
    for _, msg := range s.conversation {
        parts = append(parts, msg.Parts...)
    }

    resp, err := s.model.GenerateContent(ctx, parts...)
    if err != nil {
        return nil, err
    }

    s.lastUsage = resp.UsageMetadata

    if len(resp.Candidates) == 0 {
        return nil, errors.New("gemini: empty response")
    }

    cand := resp.Candidates[0]

    // Try to extract JSON text part
    var text string
    for _, p := range cand.Content.Parts {
        if t, ok := p.(genai.Text); ok {
            text += string(t)
        }
    }
    text = extractJSON(text)
    if text == "" {
        return nil, errors.New("gemini: no JSON object returned")
    }

    var out map[string]any
    if err := json.Unmarshal([]byte(text), &out); err != nil {
        logger.Errorf("❌ [LLM v1] Gemini structured: JSON decode error=%v text=%q", err, text)
        return nil, err
    }
    return out, nil
}

func (s *GeminiSession) CallFunctions(
	ctx context.Context,
	userPrompt string,
) ([]map[string]any, error) {

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// On first call, seed in the system message(s)
	if len(s.conversation) == 0 && len(s.systemMsgs) > 0 {
		s.conversation = append(s.conversation, s.systemMsgs...)
	}

	// 1) add user message
	user := genai.Content{Role: "user", Parts: []genai.Part{genai.Text(userPrompt)}}
	s.conversation = append(s.conversation, user)

	// Flatten all parts into a single GenerateContent call
	parts := []genai.Part{}
	for _, msg := range s.conversation {
		parts = append(parts, msg.Parts...)
	}

	// TODO: Implement genai.WithCachedContent(cache.Name)

	resp, err := s.model.GenerateContent(ctx, parts...)

	if err != nil {
		return nil, err
	}

	// update usage
	s.lastUsage = resp.UsageMetadata

	// Usage metadata
	if u := resp.UsageMetadata; u != nil {
		logger.Debugf("💰 [LLM v1] Gemini usage: prompt=%d, response=%d, total=%d tokens",
			u.PromptTokenCount, u.CandidatesTokenCount,
			u.TotalTokenCount)
	}

	if len(resp.Candidates) == 0 {
		return nil, errors.New("gemini: empty response")
	}

	cand := resp.Candidates[0]

	// ─ Path A: native FunctionCall parts ─────────────────────────────
	var out []map[string]any
	for _, p := range cand.Content.Parts {
		if fc, ok := p.(genai.FunctionCall); ok {
			out = append(out, map[string]any{
				"name": fc.Name,
				"args": fc.Args,
			})
		}
	}
	if len(out) > 0 {
		return out, nil
	}

	// ─ Path B: JSON text fallback ────────────────────────────────────
	var text string
	for _, p := range cand.Content.Parts {
		if t, ok := p.(genai.Text); ok {
			text += string(t)
		}
	}
	text = extractJSON(text)
	if text == "" {
		return nil, errors.New("gemini: no function call returned")
	}

	if err := json.Unmarshal([]byte(text), &out); err != nil {
		logger.Errorf("❌ [LLM v1] Gemini: fallback JSON decode error=%v text=%q", err, text)
		return nil, err
	}
	return out, nil
}

// CallFunctionsWithCache uses a cached content reference for the LLM call
func (s *GeminiSession) CallFunctionsWithCache(ctx context.Context, cacheKey speech.CacheKey, userPrompt string) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

    // Use cached content reference if supported by the SDK.
    // If not available, gracefully fall back to an uncached generation for now.
    var resp *genai.GenerateContentResponse
    var err error

    // TODO: Use cached content when SDK exposes it; fallback to uncached call for now
    resp, err = s.model.GenerateContent(ctx, genai.Text(userPrompt))
    if err != nil {
        return nil, err
    }
	
	// Update usage tracking
	s.lastUsage = resp.UsageMetadata
	
	// Usage metadata logging
	if u := resp.UsageMetadata; u != nil {
		logger.Debugf("💰 [LLM v1] Gemini cached usage: prompt=%d, response=%d, total=%d tokens",
			u.PromptTokenCount, u.CandidatesTokenCount,
			u.TotalTokenCount)
	}

	if len(resp.Candidates) == 0 {
		return nil, errors.New("gemini: empty response from cached call")
	}

	cand := resp.Candidates[0]
	
	// Same response processing as CallFunctions
	var out []map[string]any
	for _, p := range cand.Content.Parts {
		if fc, ok := p.(genai.FunctionCall); ok {
			out = append(out, map[string]any{
				"name": fc.Name,
				"args": fc.Args,
			})
		}
	}
	if len(out) > 0 {
		return out, nil
	}

	// Path B: JSON text fallback
	var text string
	for _, p := range cand.Content.Parts {
		if t, ok := p.(genai.Text); ok {
			text += string(t)
		}
	}
	text = extractJSON(text)
	if text == "" {
		return nil, errors.New("[LLM v1] Gemini cached: no function call returned from cached call")
	}

	if err := json.Unmarshal([]byte(text), &out); err != nil {
		logger.Errorf("❌ [LLM v1] Gemini cached: fallback JSON decode error=%v text=%q", err, text)
		return nil, err
	}
	return out, nil
}

// TODO: enhance the session.