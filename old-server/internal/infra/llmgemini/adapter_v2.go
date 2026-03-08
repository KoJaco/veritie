//go:build genai2

package llmgemini

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/pkg/metrics"
)

// AdapterV2 implements speech.CachedLLM using the new google.golang.org/genai SDK (via GeminiSessionV2)
type AdapterV2 struct {
    apiKey    string
    modelName string

    mu   sync.Mutex
    // sessions keyed by version string: "func:<hash>" or "struct:<hash>"
    sessions map[string]*GeminiSessionV2
}

func NewV2(apiKey, model string) *AdapterV2 {
    return &AdapterV2{apiKey: apiKey, modelName: model, sessions: make(map[string]*GeminiSessionV2)}
}

func (a *AdapterV2) getSession(key string) (*GeminiSessionV2, error) {
    a.mu.Lock()
    defer a.mu.Unlock()
    if s, ok := a.sessions[key]; ok {
        return s, nil
    }
    s, err := NewSessionV2(a.apiKey, a.modelName)
    if err != nil {
        return nil, err
    }
    a.sessions[key] = s
    return s, nil
}

func hashFunctions(defs []speech.FunctionDefinition, guide string) string {
    h := sha256.New()
    h.Write([]byte(guide))
    for _, d := range defs {
        h.Write([]byte(d.Name))
        h.Write([]byte(d.Description))
        for _, p := range d.Parameters {
            h.Write([]byte(p.Name))
            h.Write([]byte(p.Type))
        }
    }
    return hex.EncodeToString(h.Sum(nil))
}

func hashStructured(schema []byte, guide string) string {
    h := sha256.New()
    h.Write(schema)
    h.Write([]byte(guide))
    return hex.EncodeToString(h.Sum(nil))
}

func (a *AdapterV2) Enrich(ctx context.Context, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
    if cfg == nil {
        return nil, nil, nil
    }
    key := "func:" + hashFunctions(cfg.Declarations, cfg.ParsingGuide)
    sess, err := a.getSession(key)
    if err != nil { return nil, nil, err }

    // Configure tools/system for this version (idempotent per session)
    sess.ConfigureOnce(cfg.Declarations, cfg.ParsingGuide)

    raw, err := sess.CallFunctions(ctx, string(prompt))
    if err != nil {
        return nil, nil, err
    }

    out := make([]speech.FunctionCall, 0, len(raw))
    for _, m := range raw {
        name, _ := m["name"].(string)
        args, _ := m["args"].(map[string]any)
        out = append(out, speech.FunctionCall{Name: name, Args: args})
    }

    usage := &speech.LLMUsage{}
    if u := sess.LastUsage(); u != nil {
        usage.Prompt = int64(u.PromptTokenCount)
        usage.Completion = int64(u.CandidatesTokenCount)
    }
    return out, usage, nil
}

func (a *AdapterV2) EnrichWithCache(ctx context.Context, cacheKey speech.CacheKey, prompt speech.Prompt, partial speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {
    key := "func:" + hashFunctions(cfg.Declarations, cfg.ParsingGuide)
    sess, err := a.getSession(key)
    if err != nil { return nil, nil, err }
    if cfg != nil { sess.ConfigureOnce(cfg.Declarations, cfg.ParsingGuide) }

    raw, err := sess.CallFunctionsWithCache(ctx, cacheKey, string(prompt))
    if err != nil {
        return nil, nil, err
    }
    out := make([]speech.FunctionCall, 0, len(raw))
    for _, m := range raw {
        name, _ := m["name"].(string)
        args, _ := m["args"].(map[string]any)
        out = append(out, speech.FunctionCall{Name: name, Args: args})
    }
    usage := &speech.LLMUsage{Cached: true}
    if u := sess.LastUsage(); u != nil {
        usage.Prompt = int64(u.PromptTokenCount)
        usage.Completion = int64(u.CandidatesTokenCount)
        usage.SavedPromptTokens = int64(u.CachedContentTokenCount)
    }
    logger.Debugf("🧠 [LLM v2] cached enrich executed")
    return out, usage, nil
}

// Ensure interface conformance
var _ speech.LLM = (*AdapterV2)(nil)
var _ speech.CachedLLM = (*AdapterV2)(nil)

// ── Structured LLM support ────────────────────────────────────────────────────

func (a *AdapterV2) GenerateStructured(
    ctx context.Context,
    prompt speech.Prompt,
    partial speech.Transcript,
    cfg *speech.StructuredConfig,
) (map[string]any, *speech.LLMUsage, error) {
    if cfg == nil { return nil, nil, nil }
    key := "struct:" + hashStructured(cfg.Schema, cfg.ParsingGuide)
    sess, err := a.getSession(key)
    if err != nil { return nil, nil, err }

    logger.Debugf("🧪 [LLM v2] ConfigureStructuredOnce: schema_len=%d guide_len=%d", len(cfg.Schema), len(cfg.ParsingGuide))
    sess.ConfigureStructuredOnce(cfg.Schema, cfg.ParsingGuide)

    logger.Debugf("🧪 [LLM v2] CallStructured: prompt_len=%d", len(prompt))
    obj, err := sess.CallStructured(ctx, string(prompt))
    if err != nil {
        metrics.IncCounter("llm_request_error_total", metrics.Labels{"provider": "gemini", "cached": "false", "mode": "structured"}, 1)
        return nil, nil, err
    }
    usage := &speech.LLMUsage{}
    if u := sess.LastUsage(); u != nil {
        usage.Prompt = int64(u.PromptTokenCount)
        usage.Completion = int64(u.CandidatesTokenCount)
    }
    return obj, usage, nil
}

func (a *AdapterV2) GenerateStructuredWithCache(
    ctx context.Context,
    cacheKey speech.CacheKey,
    prompt speech.Prompt,
    partial speech.Transcript,
    cfg *speech.StructuredConfig,
) (map[string]any, *speech.LLMUsage, error) {
    if cfg == nil { return nil, nil, nil }
    key := "struct:" + hashStructured(cfg.Schema, cfg.ParsingGuide)
    sess, err := a.getSession(key)
    if err != nil { return nil, nil, err }

    logger.Debugf("🧪 [LLM v2] CallStructuredWithCache: prompt_len=%d cache_key=%s", len(prompt), cacheKey)
    obj, err := sess.CallStructuredWithCache(ctx, cacheKey, string(prompt))
    if err != nil {
        metrics.IncCounter("llm_request_error_total", metrics.Labels{"provider": "gemini", "cached": "true", "mode": "structured"}, 1)
        return nil, nil, err
    }
    usage := &speech.LLMUsage{Cached: true}
    if u := sess.LastUsage(); u != nil {
        usage.Prompt = int64(u.PromptTokenCount)
        usage.Completion = int64(u.CandidatesTokenCount)
        usage.SavedPromptTokens = int64(u.CachedContentTokenCount)
    }
    logger.Debugf("🧠 [LLM v2] cached structured generation executed")
    return obj, usage, nil
}

var _ speech.StructuredLLM = (*AdapterV2)(nil)
var _ speech.CachedStructuredLLM = (*AdapterV2)(nil)