// internal/infra/llmgemini/adapter.go
package llmgemini

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/pkg/metrics"
)

type Adapter struct {
	apiKey    string
	modelName string

	mu    sync.Mutex
	sess  *GeminiSession // nil until first use
	tools string         // hash of last configured tools

	// trackers
	totPrompt int64
	totOutput int64
}

type LLMUsage struct{ Prompt, Completion int64 }

// New returns the concrete implementation of the speech.LLM port.
func New(apiKey, model string) *Adapter {
	return &Adapter{apiKey: apiKey, modelName: model}
}

// ── speech.LLM interface ──────────────────────────────────────────────────────
func (a *Adapter) Enrich(
	ctx context.Context,
	prompt speech.Prompt,
	partial speech.Transcript,
	cfg *speech.FunctionConfig,
) ([]speech.FunctionCall, *speech.LLMUsage, error) {

	if cfg == nil { // nothing to do
		return nil, nil, nil
	}

	// 1. Make sure we have a session
	if err := a.ensureSession(cfg); err != nil {
		return nil, nil, err
	}

	// 2. Call Gemini
    raw, err := a.sess.CallFunctions(ctx, string(prompt))
	if err != nil {
        metrics.IncCounter("llm_request_error_total", metrics.Labels{"provider": "gemini", "cached": "false"}, 1)
		return nil, nil, err
	}

	// Accu tokens
	var delta speech.LLMUsage
	if u := a.sess.LastUsage(); u != nil {
		delta.Prompt = int64(u.PromptTokenCount)
		delta.Completion = int64(u.CandidatesTokenCount)

		// keep a running DEBUG counter (optional)
        a.mu.Lock()
		a.totPrompt += delta.Prompt
		a.totOutput += delta.Completion
		logger.ServiceDebugf("LLM", "Gemini running cost: prompt=%d, output=%d tokens",
			a.totPrompt, a.totOutput)
		a.mu.Unlock()
        metrics.IncCounter("llm_tokens_prompt_total", metrics.Labels{"provider": "gemini", "cached": "false"}, uint64(delta.Prompt))
        metrics.IncCounter("llm_tokens_completion_total", metrics.Labels{"provider": "gemini", "cached": "false"}, uint64(delta.Completion))
	}

	// 3. Convert []map[string]any → []speech.FunctionCall
	out := make([]speech.FunctionCall, 0, len(raw))
	for _, m := range raw {
		fc := speech.FunctionCall{
			Name: m["name"].(string),
			Args: m["args"].(map[string]any),
		}
		out = append(out, fc)
	}

	return out, &delta, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────
func (a *Adapter) ensureSession(cfg *speech.FunctionConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Build (or rebuild) session only if tools changed
	// toolHash := hashDefs(cfg.Declarations)

	if a.sess == nil {
		s, err := NewSession(a.apiKey, a.modelName)
		if err != nil {
			return err
		}
		a.sess = s
	}

	// if toolHash != a.tools {
	// 	a.sess.ConfigureTools(cfg.Declarations)
	// 	a.tools = toolHash
	// }

	a.sess.ConfigureOnce(cfg.Declarations, cfg.ParsingGuide)

	return nil
}

func (a *Adapter) SetSession(s any) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session, ok := s.(*GeminiSession); ok {
		a.sess = session

		if len(session.model.Tools) > 0 {
			defs := reverseConvert(session.model.Tools[0].FunctionDeclarations)
			a.tools = hashDefs(defs)
		}
	}
}

// helper:
func reverseConvert(decls []*genai.FunctionDeclaration) []speech.FunctionDefinition {
	out := make([]speech.FunctionDefinition, 0, len(decls))
	for _, d := range decls {
		out = append(out, speech.FunctionDefinition{
			Name:        d.Name,
			Description: d.Description,
			// params not used again by the adapter; imit to keep it light
		})
	}

	return out
}

// todo: cache JSON marhsal to avoid re-allocation?
func hashDefs(defs []speech.FunctionDefinition) string {
	b, _ := json.Marshal(defs)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// Implements speech.CachedLLM.EnrichWithCache
func (a *Adapter) EnrichWithCache(
	ctx context.Context,
	cacheKey speech.CacheKey,
	prompt speech.Prompt,
    partial speech.Transcript,
    cfg *speech.FunctionConfig,
) ([]speech.FunctionCall, *speech.LLMUsage, error) {

    if a.sess == nil {
        s, err := NewSession(a.apiKey, a.modelName)
        if err != nil {
            return nil, nil, err
        }
        a.sess = s
    }
    if cfg != nil {
        a.sess.ConfigureOnce(cfg.Declarations, cfg.ParsingGuide)
    }

	// Call Gemini with cache reference
    raw, err := a.sess.CallFunctionsWithCache(ctx, cacheKey, string(prompt))
	if err != nil {
        metrics.IncCounter("llm_request_error_total", metrics.Labels{"provider": "gemini", "cached": "true"}, 1)
		return nil, nil, err
	}

	// Accumulate tokens (same as regular Enrich)
	var delta speech.LLMUsage
	if u := a.sess.LastUsage(); u != nil {
		delta.Prompt = int64(u.PromptTokenCount)
		delta.Completion = int64(u.CandidatesTokenCount)
		delta.Cached = true

		// Keep running DEBUG counter
		a.mu.Lock()
		a.totPrompt += delta.Prompt
		a.totOutput += delta.Completion
		logger.ServiceDebugf("LLM", "Gemini cached running cost: prompt=%d, output=%d tokens",
			a.totPrompt, a.totOutput)
		a.mu.Unlock()
		metrics.IncCounter("llm_tokens_prompt_total", metrics.Labels{"provider": "gemini", "cached": "true"}, uint64(delta.Prompt))
		metrics.IncCounter("llm_tokens_completion_total", metrics.Labels{"provider": "gemini", "cached": "true"}, uint64(delta.Completion))
	}

	// Convert []map[string]any → []speech.FunctionCall (same as regular Enrich)
	out := make([]speech.FunctionCall, 0, len(raw))
	for _, m := range raw {
		fc := speech.FunctionCall{
			Name: m["name"].(string),
			Args: m["args"].(map[string]any),
		}
		out = append(out, fc)
	}

    return out, &delta, nil
}

// Comp-time guarantee
var _ speech.LLM = (*Adapter)(nil)
var _ speech.CachedLLM = (*Adapter)(nil)
var _ speech.SessionSetter = (*Adapter)(nil)

// ── Structured LLM support ────────────────────────────────────────────────────

// GenerateStructured implements speech.StructuredLLM using schema-guided JSON output
func (a *Adapter) GenerateStructured(
    ctx context.Context,
    prompt speech.Prompt,
    partial speech.Transcript,
    cfg *speech.StructuredConfig,
) (map[string]any, *speech.LLMUsage, error) {
    if cfg == nil {
        return nil, nil, nil
    }

    a.mu.Lock()
    if a.sess == nil {
        s, err := NewSession(a.apiKey, a.modelName)
        if err != nil {
            a.mu.Unlock()
            return nil, nil, err
        }
        a.sess = s
    }
    a.mu.Unlock()

    // Configure structured mode once
    a.sess.ConfigureStructuredOnce(cfg.Schema, cfg.ParsingGuide)

    obj, err := a.sess.CallStructured(ctx, string(prompt))
    if err != nil {
        metrics.IncCounter("llm_request_error_total", metrics.Labels{"provider": "gemini", "cached": "false", "mode": "structured"}, 1)
        return nil, nil, err
    }

    var delta speech.LLMUsage
    if u := a.sess.LastUsage(); u != nil {
        delta.Prompt = int64(u.PromptTokenCount)
        delta.Completion = int64(u.CandidatesTokenCount)
        a.mu.Lock()
        a.totPrompt += delta.Prompt
        a.totOutput += delta.Completion
        a.mu.Unlock()
        metrics.IncCounter("llm_tokens_prompt_total", metrics.Labels{"provider": "gemini", "cached": "false", "mode": "structured"}, uint64(delta.Prompt))
        metrics.IncCounter("llm_tokens_completion_total", metrics.Labels{"provider": "gemini", "cached": "false", "mode": "structured"}, uint64(delta.Completion))
    }

    return obj, &delta, nil
}

var _ speech.StructuredLLM = (*Adapter)(nil)