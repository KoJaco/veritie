//go:build genai2

package llmgemini

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	g2 "google.golang.org/genai"

	"fmt"

	"schma.ai/internal/app/prompts"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// GeminiSessionV2 uses the new google.golang.org/genai SDK
type GeminiSessionV2 struct {
    client    *g2.Client
    modelName string

    // cached configuration
    toolsSet      bool
    structuredSet bool
    structuredSchema *g2.Schema

    tools      []*g2.Tool
    toolConfig *g2.ToolConfig
    system     *g2.Content

    lastUsage *g2.GenerateContentResponseUsageMetadata

    // Track config version to allow dynamic updates mid-session
    configVersion string
}

// Client exposes the underlying client for wiring cache adapters
func (s *GeminiSessionV2) Client() *g2.Client { return s.client }

// LastUsage returns the last usage metadata captured from a response
func (s *GeminiSessionV2) LastUsage() *g2.GenerateContentResponseUsageMetadata { return s.lastUsage }

// NewSessionV2 initializes a new client using the new SDK
func NewSessionV2(apiKey, modelName string) (*GeminiSessionV2, error) {
    if apiKey == "" {
        apiKey = os.Getenv("GEMINI_API_KEY")
    }
    if apiKey == "" {
        return nil, errors.New("gemini API key missing in config")
    }

    ctx := context.Background()
    cli, err := g2.NewClient(ctx, &g2.ClientConfig{APIKey: apiKey})
    if err != nil {
        return nil, err
    }

    return &GeminiSessionV2{
        client:    cli,
        modelName: modelName,
    }, nil
}

// ConfigureOnce converts our function definitions and system guide to provider types.
func (s *GeminiSessionV2) ConfigureOnce(defs []speech.FunctionDefinition, systemGuide string) {
    // Compute a simple version hash from defs + guide to allow updates when config changes
    version := computeConfigVersion(defs, systemGuide)
    if s.toolsSet && version == s.configVersion {
        return
    }

    decls := convertDefsV2(defs)
    s.tools = []*g2.Tool{{FunctionDeclarations: decls}}
    // Use permissive function calling by default (allow the model to call any of the declared tools)
    s.toolConfig = &g2.ToolConfig{
        FunctionCallingConfig: &g2.FunctionCallingConfig{
            // Model constrained to always predict function calls only -- do NOT reply in natural language
            Mode: g2.FunctionCallingConfigMode("ANY"),
        },
    }

    sys := prompts.BuildFunctionsSystemInstructionPrompt(systemGuide)
    // Pass as SystemInstruction via config. Avoid "system" role to satisfy API constraints.
    s.system = g2.NewContentFromText(sys, g2.Role(""))
    s.toolsSet = true
    s.configVersion = version
    logger.ServiceDebugf("LLM", "session configured: decls=%d version=%s", countDeclsV2(s.tools), version)
}

// computeConfigVersion builds a stable short fingerprint for defs+guide
func computeConfigVersion(defs []speech.FunctionDefinition, guide string) string {
    // Lightweight, deterministic: count names + guide length. Good enough to detect session changes.
    // If desired, replace with a proper hash.
    total := len(guide)
    for _, d := range defs {
        total += len(d.Name) + len(d.Description)
        for _, p := range d.Parameters {
            total += len(p.Name) + len(p.Type)
        }
    }
    // Encode as base-36
    return fmt.Sprintf("v%x", total)
}

// CallFunctions sends an uncached generation request
func (s *GeminiSessionV2) CallFunctions(ctx context.Context, userPrompt string) ([]map[string]any, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Build contents: user only. SystemInstructions are provided via config.
    contents := []*g2.Content{g2.NewContentFromText(userPrompt, g2.RoleUser)}

    // Debug: request summary
    logger.Debugf("[LLM v2] Generate (uncached) model=%s tools=%d decls=%d system_len=%d prompt=%s",
        s.modelName, len(s.tools), countDeclsV2(s.tools), contentLenSafe(s.system), preview(userPrompt))

    // Use zero temperature for maximum determinism on function extraction
    temp := float32(0.0)
    topP := float32(0.2)
    cfg := &g2.GenerateContentConfig{
        SystemInstruction: s.system,
        Tools:             s.tools,
        ToolConfig:        s.toolConfig,
        Temperature:       &temp,
        TopP: &topP,
    }

    // Use the provided context (typically from worker pool) to control deadline
    start := time.Now()
    resp, err := s.client.Models.GenerateContent(ctx, s.modelName, contents, cfg)
    took := time.Since(start)
    if resp != nil {
        s.lastUsage = resp.UsageMetadata
        if u := resp.UsageMetadata; u != nil {
            logger.ServiceDebugf("LLM", "uncached ok in %dms | prompt=%d resp=%d total=%d cached_in_prompt=%d",
                took.Milliseconds(), u.PromptTokenCount, u.CandidatesTokenCount, u.TotalTokenCount, u.CachedContentTokenCount)
        }
    }
    if err != nil {
        logger.Errorf("[LLM v2] uncached request failed in %dms: %v", took.Milliseconds(), err)
        return nil, err
    }

    // Usage (exact fields may differ; adjust when wiring fully)
    // s.lastUsage = resp.UsageMetadata // Uncomment when field is available

    // Prefer function calls if present
    if fcs := resp.FunctionCalls(); len(fcs) > 0 {
        out := make([]map[string]any, 0, len(fcs))
        for _, fc := range fcs {
            out = append(out, map[string]any{"name": fc.Name, "args": fc.Args})
        }
        return out, nil
    }
    // Fallback attempt: extract from parts if available
    if len(resp.Candidates) > 0 && resp.Candidates[0] != nil && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
        if out := extractFunctionCallsV2(resp.Candidates[0].Content.Parts); len(out) > 0 {
            return out, nil
        }
    }

    // Fallback to text
    text := resp.Text()
    if len(text) > 0 {
        logger.ServiceDebugf("LLM", "uncached text fallback preview: %s", preview(text))
    }

    text = extractJSON(text)
    if text == "" {
        return nil, errors.New("gemini: no function call returned")
    }

    var out []map[string]any
    if err := json.Unmarshal([]byte(text), &out); err != nil {
        logger.Errorf("❌ [LLM v2] Gemini v2: fallback JSON decode error=%v text=%q", err, text)
        return nil, err
    }
    return out, nil
}

// ConfigureStructuredOnce configures the session for structured JSON output using ResponseSchema
func (s *GeminiSessionV2) ConfigureStructuredOnce(schema json.RawMessage, systemGuide string) {
    if s.structuredSet && s.structuredSchema != nil {
        logger.ServiceDebugf("LLM", "ConfigureStructuredOnce: already configured; skipping")
        return
    }
    logger.ServiceDebugf("LLM", "ConfigureStructuredOnce: parsing schema bytes=%d", len(schema))
    s.structuredSchema = buildSchemaFromJSONV2(schema)
    // Build full system instruction prompt from parsing guide (same pattern as functions)
    if systemGuide != "" {
        fullSystemPrompt := prompts.BuildStructuredSystemInstructionPrompt(systemGuide)
        logger.ServiceDebugf("LLM", "Structured system prompt:\n%s", fullSystemPrompt)
        s.system = g2.NewContentFromText(fullSystemPrompt, g2.Role(""))
    }
    s.structuredSet = true
}

// CallStructured generates a single JSON object according to the configured structured SystemInstruction.
func (s *GeminiSessionV2) CallStructured(ctx context.Context, userPrompt string) (map[string]any, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    contents := []*g2.Content{g2.NewContentFromText(userPrompt, g2.RoleUser)}

    temp := float32(0.1)
    cfg := &g2.GenerateContentConfig{
        SystemInstruction: s.system,
        Temperature:       &temp,
        ResponseMIMEType:  "application/json",
        ResponseSchema:    s.structuredSchema,
    }

    start := time.Now()
    resp, err := s.client.Models.GenerateContent(ctx, s.modelName, contents, cfg)
    took := time.Since(start)
    logger.ServiceDebugf("LLM", "GenerateContent structured took=%dms err=%v", took.Milliseconds(), err)
    if resp != nil {
        s.lastUsage = resp.UsageMetadata
        if u := resp.UsageMetadata; u != nil {
            logger.ServiceDebugf("LLM", "structured ok in %dms | prompt=%d resp=%d total=%d cached_in_prompt=%d",
                took.Milliseconds(), u.PromptTokenCount, u.CandidatesTokenCount, u.TotalTokenCount, u.CachedContentTokenCount)
        }
    }
    if err != nil {
        logger.Errorf("[LLM v2] structured request failed in %dms: %v", took.Milliseconds(), err)
        return nil, err
    }

    // Prefer response candidate parts with JSON; fallback to Text()
    if len(resp.Candidates) > 0 && resp.Candidates[0] != nil && resp.Candidates[0].Content != nil {
        if parts := resp.Candidates[0].Content.Parts; len(parts) > 0 {
            logger.ServiceDebugf("LLM", "parts count=%d", len(parts))
            // Attempt to marshal first part to JSON-compatible map
            b, _ := json.Marshal(parts[0])
            var try map[string]any
            if err := json.Unmarshal(b, &try); err == nil && len(try) > 0 {
                logger.ServiceDebugf("LLM", "structured part->json ok keys=%d", len(try))
                return try, nil
            }
        }
    }
    text := resp.Text()
    logger.ServiceDebugf("LLM", "resp.Text preview len=%d", len(text))
    text = extractJSON(text)
    if text == "" {
        return nil, errors.New("gemini: no JSON object returned")
    }
    var out map[string]any
    if err := json.Unmarshal([]byte(text), &out); err != nil {
        logger.Errorf("❌ [LLM v2] structured: JSON decode error=%v text=%q", err, text)
        return nil, err
    }
    return out, nil
}

// CallFunctionsWithCache uses a cached content reference for the LLM call
func (s *GeminiSessionV2) CallFunctionsWithCache(ctx context.Context, cacheKey speech.CacheKey, userPrompt string) ([]map[string]any, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Build contents: user only. SystemInstructions are provided via config.
    contents := []*g2.Content{g2.NewContentFromText(userPrompt, g2.RoleUser)}

    // Debug: request summary with cache
    logger.ServiceDebugf("LLM", "Generate (cached) model=%s cache_key=%s tools=%d decls=%d system_len=%d prompt=%s",
        s.modelName, string(cacheKey), len(s.tools), countDeclsV2(s.tools), contentLenSafe(s.system), preview(userPrompt))

    // Use zero temperature for maximum determinism on function extraction (cached)
    temp := float32(0.0)
    topP := float32(0.2)
    // With CachedContent, we must not send SystemInstruction, Tools or ToolConfig
    cfg := &g2.GenerateContentConfig{
        CachedContent: string(cacheKey),
        Temperature:   &temp,
        TopP: &topP,
    }

    start := time.Now()
    resp, err := s.client.Models.GenerateContent(ctx, s.modelName, contents, cfg)
    took := time.Since(start)
    if resp != nil {
        s.lastUsage = resp.UsageMetadata
        if u := resp.UsageMetadata; u != nil {
            logger.ServiceDebugf("LLM", "cached ok in %dms | prompt=%d resp=%d total=%d cached_in_prompt=%d",
                took.Milliseconds(), u.PromptTokenCount, u.CandidatesTokenCount, u.TotalTokenCount, u.CachedContentTokenCount)
        }
    }
    if err != nil {
        logger.Errorf("[LLM v2] cached request failed in %dms (key=%s): %v", took.Milliseconds(), string(cacheKey), err)
        return nil, err
    }

    if fcs := resp.FunctionCalls(); len(fcs) > 0 {
        logger.ServiceDebugf("LLM", "cached functionCalls() count=%d", len(fcs))
        out := make([]map[string]any, 0, len(fcs))
        for _, fc := range fcs {
            out = append(out, map[string]any{"name": fc.Name, "args": fc.Args})
        }
        return out, nil
    }
    // Fallback attempt: extract from parts if available
    if len(resp.Candidates) > 0 && resp.Candidates[0] != nil && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
        logger.ServiceDebugf("LLM", "cached candidates parts=%d; attempting part extraction", len(resp.Candidates[0].Content.Parts))
        if out := extractFunctionCallsV2(resp.Candidates[0].Content.Parts); len(out) > 0 {
            return out, nil
        }
    }

    text := resp.Text()
    if len(text) > 0 {
        logger.ServiceDebugf("LLM", "cached text fallback preview: %s", preview(text))
    }
    if text == "" {
        logger.Warnf("[LLM v2] cached response contained no function calls and no text fallback")
    }

    text = extractJSON(text)
    if text == "" {
        return nil, errors.New("gemini: no function call returned from cached call")
    }

    var out []map[string]any
    if err := json.Unmarshal([]byte(text), &out); err != nil {
        logger.Errorf("❌ [LLM v2] Gemini v2 cached: fallback JSON decode error=%v text=%q", err, text)
        return nil, err
    }
    return out, nil
}

// CallStructuredWithCache uses a cached content reference for structured JSON generation
func (s *GeminiSessionV2) CallStructuredWithCache(ctx context.Context, cacheKey speech.CacheKey, userPrompt string) (map[string]any, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Build contents: user only. SystemInstructions are provided via cache.
    contents := []*g2.Content{g2.NewContentFromText(userPrompt, g2.RoleUser)}

    // Debug: request summary with cache
    logger.ServiceDebugf("LLM", "GenerateStructured (cached) model=%s cache_key=%s prompt=%s",
        s.modelName, string(cacheKey), preview(userPrompt))

    // Use a low temperature for determinism
    temp := float32(0.1)
    // With CachedContent, we must not send SystemInstruction, Tools or ToolConfig
    cfg := &g2.GenerateContentConfig{
        CachedContent: string(cacheKey),
        Temperature:   &temp,
    }

    start := time.Now()
    resp, err := s.client.Models.GenerateContent(ctx, s.modelName, contents, cfg)
    took := time.Since(start)
    if resp != nil {
        s.lastUsage = resp.UsageMetadata
        if u := resp.UsageMetadata; u != nil {
            logger.ServiceDebugf("LLM", "cached structured ok in %dms | prompt=%d resp=%d total=%d cached_in_prompt=%d",
                took.Milliseconds(), u.PromptTokenCount, u.CandidatesTokenCount, u.TotalTokenCount, u.CachedContentTokenCount)
        }
    }
    if err != nil {
        logger.Errorf("[LLM v2] cached structured request failed in %dms (key=%s): %v", took.Milliseconds(), string(cacheKey), err)
        return nil, err
    }

    // Prefer response candidate parts with JSON; fallback to Text()
    if len(resp.Candidates) > 0 && resp.Candidates[0] != nil && resp.Candidates[0].Content != nil {
        if parts := resp.Candidates[0].Content.Parts; len(parts) > 0 {
            logger.ServiceDebugf("LLM", "cached structured parts count=%d", len(parts))
            // Attempt to marshal first part to JSON-compatible map
            b, _ := json.Marshal(parts[0])
            var try map[string]any
            if err := json.Unmarshal(b, &try); err == nil && len(try) > 0 {
                logger.ServiceDebugf("LLM", "cached structured part->json ok keys=%d", len(try))
                return try, nil
            }
        }
    }
    text := resp.Text()
    logger.ServiceDebugf("LLM", "cached structured resp.Text preview len=%d", len(text))
    text = extractJSON(text)
    if text == "" {
        return nil, errors.New("gemini: no JSON object returned from cached structured call")
    }
    var out map[string]any
    if err := json.Unmarshal([]byte(text), &out); err != nil {
        logger.Errorf("❌ [LLM v2] cached structured: JSON decode error=%v text=%q", err, text)
        return nil, err
    }
    return out, nil
}

// convertDefsV2 maps our domain function defs to the new SDK declarations
func convertDefsV2(defs []speech.FunctionDefinition) []*g2.FunctionDeclaration {
    out := make([]*g2.FunctionDeclaration, 0, len(defs))
    for _, d := range defs {
        decl := &g2.FunctionDeclaration{
            Name:        d.Name,
            Description: d.Description,
        }
        if p := buildSchemaFromParamsV2(d.Parameters); p != nil {
            decl.Parameters = p
        }
        out = append(out, decl)
    }
    return out
}

// buildSchemaFromParamsV2 creates a JSON schema (OBJECT) from domain params
func buildSchemaFromParamsV2(params []speech.FunctionParam) *g2.Schema {
    if len(params) == 0 {
        return nil
    }
    props := map[string]*g2.Schema{}
    required := make([]string, 0)
    for _, p := range params {
        props[p.Name] = &g2.Schema{Type: mapParamTypeV2(p.Type)}
        if p.Required {
            required = append(required, p.Name)
        }
    }
    return &g2.Schema{Type: g2.Type("OBJECT"), Properties: props, Required: required}
}

func mapParamTypeV2(t string) g2.Type {
    switch t {
    case "string", "STRING":
        return g2.Type("STRING")
    case "number", "NUMBER":
        return g2.Type("NUMBER")
    case "integer", "INTEGER":
        return g2.Type("INTEGER")
    case "boolean", "BOOLEAN":
        return g2.Type("BOOLEAN")
    case "array", "ARRAY":
        return g2.Type("ARRAY")
    case "object", "OBJECT":
        return g2.Type("OBJECT")
    default:
        return g2.Type("STRING")
    }
}

// tryPartTextV2 attempts to recover a text payload from a Part (placeholder until API fully wired)
func tryPartTextV2(p *g2.Part) string { return "" }

// extractFunctionCallsV2 tries to read function calls from parts
func extractFunctionCallsV2(parts []*g2.Part) []map[string]any {
    // The new SDK exposes explicit function call parts; map them if present
    out := make([]map[string]any, 0)
    for _, p := range parts {
        if p == nil || p.FunctionCall == nil {
            continue
        }
        fc := p.FunctionCall
        out = append(out, map[string]any{
            "name": fc.Name,
            "args": fc.Args,
        })
    }
    return out
}

// countDeclsV2 returns count of function declarations across toolset
func countDeclsV2(tools []*g2.Tool) int {
    total := 0
    for _, t := range tools {
        if t == nil {
            continue
        }
        total += len(t.FunctionDeclarations)
    }
    return total
}

// contentLenSafe returns approximate length of system text if present
func contentLenSafe(c *g2.Content) int {
    if c == nil || len(c.Parts) == 0 {
        return 0
    }
    // Best-effort; only checks first part's text length
    if p := c.Parts[0]; p != nil && p.Text != "" {
        return len(p.Text)
    }
    return 0
}

// preview returns a safe short preview of user prompt
func preview(s string) string {
    const n = 80
    if len(s) <= n {
        return s
    }
    return s[:n] + "…"
}

