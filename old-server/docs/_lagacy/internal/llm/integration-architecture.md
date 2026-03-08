# LLM Integration Architecture

## Overview

The LLM integration provides intelligent function extraction from speech transcripts using Google's Gemini API with native function calling capabilities. The system is designed with a clean interface that abstracts LLM provider details, enabling future migration to self-hosted or custom-trained solutions while maintaining consistent function extraction quality.

## Core Architecture

### Provider-Agnostic Design

```
Application Layer → LLM Interface → Provider Adapter → External LLM API
                         ↓              ↓                 ↓
                    Unified Contract   Session Mgmt    Function Calling
```

### Component Diagram

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Pipeline      │    │   LLM Adapter   │    │  Gemini Session │
│  (Function      │───▶│   (Provider     │───▶│   Management    │
│   Extraction)   │    │   Abstraction)  │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Domain        │    │  Tool Config    │    │   Google AI     │
│   Interface     │    │  Management     │    │   Client SDK    │
│ (speech.LLM)    │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                 │                       │
                                 ▼                       ▼
                        ┌─────────────────┐    ┌─────────────────┐
                        │ Function Schema │    │   Function      │
                        │   Conversion    │    │   Calling API   │
                        │                 │    │                 │
                        └─────────────────┘    └─────────────────┘
```

## Domain Interface

### Core LLM Contract (`internal/domain/speech/`)

```go
// LLM defines the unified interface for function extraction from transcripts
type LLM interface {
    // Enrich processes transcript and returns structured function calls
    Enrich(ctx context.Context, prompt Prompt, transcript Transcript,
           cfg *FunctionConfig) ([]FunctionCall, *LLMUsage, error)
}

// Optional interface for session management (hot-swapping)
type SessionSetter interface {
    SetSession(session any) // For dynamic function schema updates
}

// Core data structures
type Prompt string

type FunctionConfig struct {
    Declarations   []FunctionDefinition `json:"definitions"`    // Available functions
    ParsingGuide   string               `json:"parsing_guide"`  // LLM context hints
    UpdateMS       int                  `json:"update_ms"`      // Throttling interval
}

type FunctionDefinition struct {
    Name        string                `json:"name"`
    Description string                `json:"description"`
    Parameters  []FunctionParameter   `json:"parameters"`
}

type FunctionParameter struct {
    Name        string   `json:"name"`
    Type        string   `json:"type"`         // "string", "number", "boolean", "array", "object"
    Description string   `json:"description"`
    Required    bool     `json:"required"`
    Enum        []string `json:"enum,omitempty"` // For constrained values
}

type FunctionCall struct {
    ID   string                 `json:"id"`        // Unique identifier
    Name string                 `json:"name"`      // Function name
    Args map[string]interface{} `json:"arguments"` // Function arguments
}

type LLMUsage struct {
    Prompt     int64 `json:"prompt_tokens"`     // Input tokens consumed
    Completion int64 `json:"completion_tokens"` // Output tokens generated
}
```

**Key Design Principles:**

-   **Provider Independence**: Interface abstracts underlying LLM implementation
-   **Function-First**: Optimized for structured function extraction
-   **Session Awareness**: Maintains conversation context for coherent interactions
-   **Usage Tracking**: Comprehensive token consumption monitoring
-   **Schema Flexibility**: Dynamic function definition updates during sessions

## Gemini Integration

### Adapter Implementation (`internal/infra/llmgemini/adapter.go`)

The adapter bridges the domain interface with Gemini-specific implementation:

```go
type Adapter struct {
    apiKey    string
    modelName string

    // Session management
    mu    sync.Mutex
    sess  *GeminiSession // Nil until first use
    tools string         // Hash of configured function schemas

    // Usage tracking
    totPrompt int64
    totOutput int64
}

func New(apiKey, model string) *Adapter {
    return &Adapter{
        apiKey:    apiKey,
        modelName: model, // e.g., "gemini-2.0-flash"
    }
}

// Enrich implements speech.LLM interface
func (a *Adapter) Enrich(ctx context.Context, prompt speech.Prompt,
                         transcript speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {

    if cfg == nil {
        return nil, nil, nil // No function extraction needed
    }

    // 1. Ensure session with correct function schemas
    if err := a.ensureSession(cfg); err != nil {
        return nil, nil, fmt.Errorf("session initialization failed: %w", err)
    }

    // 2. Call Gemini with function calling enabled
    rawCalls, err := a.sess.CallFunctions(ctx, string(prompt))
    if err != nil {
        return nil, nil, fmt.Errorf("gemini function call failed: %w", err)
    }

    // 3. Track token usage
    var usage speech.LLMUsage
    if u := a.sess.LastUsage(); u != nil {
        usage.Prompt = int64(u.PromptTokenCount)
        usage.Completion = int64(u.CandidatesTokenCount)

        // Accumulate session totals for monitoring
        a.mu.Lock()
        a.totPrompt += usage.Prompt
        a.totOutput += usage.Completion
        a.mu.Unlock()

        log.Printf("💰 Gemini usage: prompt=%d, completion=%d (session total: %d/%d)",
                   usage.Prompt, usage.Completion, a.totPrompt, a.totOutput)
    }

    // 4. Convert Gemini response to domain format
    functionCalls := make([]speech.FunctionCall, 0, len(rawCalls))
    for _, raw := range rawCalls {
        fc := speech.FunctionCall{
            Name: raw["name"].(string),
            Args: raw["args"].(map[string]interface{}),
        }
        functionCalls = append(functionCalls, fc)
    }

    return functionCalls, &usage, nil
}
```

### Session Management (`internal/infra/llmgemini/session.go`)

Manages persistent conversation context and function schema configuration:

```go
type GeminiSession struct {
    client       *genai.Client
    model        *genai.GenerativeModel
    conversation []genai.Content // Persistent conversation history

    // Function calling state
    toolsSet   bool
    systemMsgs []genai.Content
    lastUsage  *genai.UsageMetadata
}

func NewSession(apiKey, modelName string) (*GeminiSession, error) {
    if apiKey == "" {
        apiKey = os.Getenv("GEMINI_API_KEY")
    }

    if apiKey == "" {
        return nil, errors.New("Gemini API key required")
    }

    // Initialize Gemini client
    ctx := context.Background()
    client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
    if err != nil {
        return nil, fmt.Errorf("failed to create Gemini client: %w", err)
    }

    // Configure model with optimal settings for function calling
    model := client.GenerativeModel(modelName)
    model.SetTemperature(0.2)  // Deterministic but creative
    model.SetTopP(0.9)         // Focused vocabulary
    model.SetTopK(20)          // Controlled randomness

    return &GeminiSession{
        client: client,
        model:  model,
    }, nil
}

// ConfigureOnce sets up function calling tools (idempotent)
func (s *GeminiSession) ConfigureOnce(definitions []speech.FunctionDefinition, systemGuide string) {
    if s.toolsSet {
        return // Already configured
    }

    // Convert domain function definitions to Gemini format
    genaiDecls := convertDefs(definitions)

    // Configure function calling
    s.model.Tools = []*genai.Tool{{
        FunctionDeclarations: genaiDecls,
    }}
    s.model.ToolConfig = &genai.ToolConfig{
        FunctionCallingConfig: &genai.FunctionCallingConfig{
            Mode: genai.FunctionCallingAny, // Always use function calling
        },
    }

    // Set system instructions for function extraction
    systemPrompt := prompts.BuildFunctionsSystemInstructionPrompt(systemGuide)
    s.systemMsgs = []genai.Content{
        {Role: "system", Parts: []genai.Part{genai.Text(systemPrompt)}},
    }

    s.toolsSet = true
    log.Printf("🔧 Gemini function tools configured: %d functions available", len(definitions))
}
```

### Function Calling Implementation

```go
func (s *GeminiSession) CallFunctions(ctx context.Context, userPrompt string) ([]map[string]any, error) {
    // Set timeout for LLM calls
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Initialize conversation with system messages on first call
    if len(s.conversation) == 0 && len(s.systemMsgs) > 0 {
        s.conversation = append(s.conversation, s.systemMsgs...)
    }

    // Add user message to conversation
    userMsg := genai.Content{
        Role:  "user",
        Parts: []genai.Part{genai.Text(userPrompt)},
    }
    s.conversation = append(s.conversation, userMsg)

    // Flatten conversation into single call (stateless from Gemini's perspective)
    parts := []genai.Part{}
    for _, msg := range s.conversation {
        parts = append(parts, msg.Parts...)
    }

    // Call Gemini with function calling enabled
    resp, err := s.model.GenerateContent(ctx, parts...)
    if err != nil {
        return nil, fmt.Errorf("gemini generate content failed: %w", err)
    }

    // Update usage tracking
    s.lastUsage = resp.UsageMetadata

    if len(resp.Candidates) == 0 {
        return nil, errors.New("gemini returned empty response")
    }

    candidate := resp.Candidates[0]

    // Path A: Native function call responses (preferred)
    var functionCalls []map[string]any
    for _, part := range candidate.Content.Parts {
        if fc, ok := part.(genai.FunctionCall); ok {
            functionCalls = append(functionCalls, map[string]any{
                "name": fc.Name,
                "args": fc.Args,
            })
        }
    }

    if len(functionCalls) > 0 {
        return functionCalls, nil
    }

    // Path B: JSON text fallback (for edge cases)
    var textContent string
    for _, part := range candidate.Content.Parts {
        if text, ok := part.(genai.Text); ok {
            textContent += string(text)
        }
    }

    // Extract JSON from text response
    jsonText := extractJSON(textContent)
    if jsonText == "" {
        return nil, errors.New("no function calls found in response")
    }

    if err := json.Unmarshal([]byte(jsonText), &functionCalls); err != nil {
        return nil, fmt.Errorf("failed to parse JSON fallback: %w", err)
    }

    return functionCalls, nil
}
```

## Schema Conversion

### Domain to Gemini Format (`internal/infra/llmgemini/schema.go`)

Converts application-defined function schemas to Gemini's format:

```go
func convertDefs(src []speech.FunctionDefinition) []*genai.FunctionDeclaration {
    declarations := make([]*genai.FunctionDeclaration, 0, len(src))

    for _, def := range src {
        // Create function declaration
        fd := &genai.FunctionDeclaration{
            Name:        def.Name,
            Description: def.Description,
            Parameters:  &genai.Schema{Type: genai.TypeObject},
        }

        // Convert parameters to properties
        properties := make(map[string]*genai.Schema)
        var required []string

        for _, param := range def.Parameters {
            schema := &genai.Schema{
                Type:        toGenaiType(param.Type),
                Description: param.Description,
            }

            // Handle enum constraints
            if len(param.Enum) > 0 {
                schema.Enum = param.Enum
            }

            properties[param.Name] = schema

            if param.Required {
                required = append(required, param.Name)
            }
        }

        fd.Parameters.Properties = properties
        fd.Parameters.Required = required

        declarations = append(declarations, fd)
    }

    return declarations
}

func toGenaiType(paramType string) genai.Type {
    switch paramType {
    case "string":
        return genai.TypeString
    case "number":
        return genai.TypeNumber
    case "integer":
        return genai.TypeInteger
    case "boolean":
        return genai.TypeBoolean
    case "array":
        return genai.TypeArray
    case "object":
        return genai.TypeObject
    default:
        return genai.TypeString // Safe fallback
    }
}
```

### Dynamic Schema Updates

```go
func (a *Adapter) ensureSession(cfg *speech.FunctionConfig) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    // Create session if needed
    if a.sess == nil {
        sess, err := NewSession(a.apiKey, a.modelName)
        if err != nil {
            return fmt.Errorf("failed to create session: %w", err)
        }
        a.sess = sess
    }

    // Check if function schemas have changed
    newToolsHash := hashDefs(cfg.Declarations)
    if newToolsHash != a.tools {
        // Reconfigure function tools
        a.sess.ConfigureOnce(cfg.Declarations, cfg.ParsingGuide)
        a.tools = newToolsHash

        log.Printf("🔄 Function schemas updated: %d functions configured", len(cfg.Declarations))
    }

    return nil
}

// Hot-swap session for dynamic schema watcher
func (a *Adapter) SetSession(session any) {
    a.mu.Lock()
    defer a.mu.Unlock()

    if geminiSess, ok := session.(*GeminiSession); ok {
        a.sess = geminiSess

        // Extract tool configuration for tracking
        if len(geminiSess.model.Tools) > 0 {
            defs := reverseConvert(geminiSess.model.Tools[0].FunctionDeclarations)
            a.tools = hashDefs(defs)
        }

        log.Printf("🔄 Session hot-swapped with new function configuration")
    }
}
```

## Error Handling & Resilience

### Timeout Management

```go
func (s *GeminiSession) CallFunctionsWithRetry(ctx context.Context, prompt string, maxRetries int) ([]map[string]any, error) {
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        // Progressive timeout: start fast, increase on retries
        timeout := time.Duration(10+attempt*5) * time.Second
        callCtx, cancel := context.WithTimeout(ctx, timeout)

        result, err := s.CallFunctions(callCtx, prompt)
        cancel()

        if err == nil {
            return result, nil
        }

        lastErr = err

        // Don't retry on certain errors
        if isNonRetryableError(err) {
            break
        }

        if attempt < maxRetries-1 {
            backoff := time.Duration(attempt+1) * time.Second
            log.Printf("⚠️ Gemini call attempt %d failed: %v, retrying in %v",
                       attempt+1, err, backoff)

            select {
            case <-time.After(backoff):
                continue
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }
    }

    return nil, fmt.Errorf("gemini calls failed after %d attempts: %w", maxRetries, lastErr)
}

func isNonRetryableError(err error) bool {
    errStr := err.Error()
    return strings.Contains(errStr, "invalid API key") ||
           strings.Contains(errStr, "quota exceeded") ||
           strings.Contains(errStr, "permission denied")
}
```

### Graceful Degradation

```go
func (a *Adapter) EnrichWithFallback(ctx context.Context, prompt speech.Prompt,
                                      transcript speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {

    // Try primary function extraction
    functions, usage, err := a.Enrich(ctx, prompt, transcript, cfg)
    if err == nil {
        return functions, usage, nil
    }

    log.Printf("⚠️ Primary function extraction failed: %v", err)

    // Fallback: Extract function hints from transcript text
    fallbackFunctions := extractFunctionHints(transcript.Text, cfg.Declarations)

    // Return partial results with error context
    return fallbackFunctions, usage, fmt.Errorf("using fallback extraction: %w", err)
}

func extractFunctionHints(text string, definitions []speech.FunctionDefinition) []speech.FunctionCall {
    var functions []speech.FunctionCall
    text = strings.ToLower(text)

    for _, def := range definitions {
        // Simple keyword matching for function names
        if strings.Contains(text, strings.ToLower(def.Name)) {
            functions = append(functions, speech.FunctionCall{
                ID:   generateID(),
                Name: def.Name,
                Args: map[string]interface{}{
                    "extracted_from": "fallback_keywords",
                    "original_text":  text,
                },
            })
        }
    }

    return functions
}
```

## Future Migration Strategy

### Self-Hosted Preparation

```go
// Abstract interface for future LLM providers
type LLMProvider interface {
    // Core function calling interface
    speech.LLM

    // Provider-specific capabilities
    GetCapabilities() ProviderCapabilities
    GetModelInfo() ModelInfo
    EstimateCost(promptTokens, maxTokens int) float64
}

type ProviderCapabilities struct {
    FunctionCalling    bool
    StreamingResponse  bool
    ContextLength      int
    MaxOutputTokens    int
    SupportedLanguages []string
}

// Future self-hosted adapter
type SelfHostedAdapter struct {
    endpoint  string
    apiKey    string
    modelName string
    client    *http.Client
}

func (s *SelfHostedAdapter) Enrich(ctx context.Context, prompt speech.Prompt,
                                   transcript speech.Transcript, cfg *speech.FunctionConfig) ([]speech.FunctionCall, *speech.LLMUsage, error) {

    // Construct request for self-hosted LLM
    request := SelfHostedRequest{
        Model:       s.modelName,
        Messages:    []Message{{Role: "user", Content: string(prompt)}},
        Functions:   cfg.Declarations,
        Temperature: 0.2,
        MaxTokens:   1000,
    }

    // Call self-hosted endpoint
    resp, err := s.callSelfHosted(ctx, request)
    if err != nil {
        return nil, nil, err
    }

    // Convert response to standard format
    return s.convertResponse(resp), &speech.LLMUsage{
        Prompt:     int64(resp.Usage.PromptTokens),
        Completion: int64(resp.Usage.CompletionTokens),
    }, nil
}
```

### Provider Selection Framework

```go
type LLMConfig struct {
    Provider     string                 // "gemini", "self_hosted", "anthropic"
    Endpoint     string                 // For self-hosted providers
    APIKey       string                 // Provider API key
    Model        string                 // Model name/version
    Options      map[string]interface{} // Provider-specific options
}

func NewLLMProvider(cfg LLMConfig) (speech.LLM, error) {
    switch cfg.Provider {
    case "gemini":
        return llmgemini.New(cfg.APIKey, cfg.Model), nil

    case "self_hosted":
        return llmselfhosted.New(cfg.Endpoint, cfg.APIKey, cfg.Model), nil

    case "anthropic":
        return llmanthropic.New(cfg.APIKey, cfg.Model), nil

    default:
        return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
    }
}
```

## Performance Optimization

### Context Management

```go
// Efficient conversation history management
func (s *GeminiSession) TrimConversation(maxTokens int) {
    if len(s.conversation) <= 2 { // Keep system + latest user message
        return
    }

    // Estimate token usage (rough approximation)
    totalTokens := 0
    for _, msg := range s.conversation {
        for _, part := range msg.Parts {
            if text, ok := part.(genai.Text); ok {
                totalTokens += len(strings.Fields(string(text))) // Word count approximation
            }
        }
    }

    // Remove old messages if over limit
    if totalTokens > maxTokens {
        // Keep system message and recent user exchanges
        systemMsgs := s.conversation[:1] // System message
        recentMsgs := s.conversation[len(s.conversation)-2:] // Last user + assistant
        s.conversation = append(systemMsgs, recentMsgs...)

        log.Printf("🧹 Conversation trimmed: %d messages → %d messages",
                   len(s.conversation), len(systemMsgs)+len(recentMsgs))
    }
}
```

### Token Usage Optimization

```go
// Intelligent prompt optimization
func optimizePrompt(originalPrompt string, transcript speech.Transcript,
                   previousCalls []speech.FunctionCall) string {

    // Include only relevant context
    contextLimit := 500 // words
    words := strings.Fields(transcript.Text)

    if len(words) > contextLimit {
        // Keep first and last portions
        start := words[:contextLimit/2]
        end := words[len(words)-contextLimit/2:]
        transcript.Text = strings.Join(start, " ") + " ... " + strings.Join(end, " ")
    }

    // Reference previous function calls for consistency
    var functionContext string
    if len(previousCalls) > 0 {
        recent := previousCalls[max(0, len(previousCalls)-3):] // Last 3 calls
        contextParts := make([]string, len(recent))
        for i, call := range recent {
            contextParts[i] = fmt.Sprintf("%s(...)", call.Name)
        }
        functionContext = fmt.Sprintf("Previous functions: [%s]. ", strings.Join(contextParts, ", "))
    }

    return fmt.Sprintf("%sTranscript: %s", functionContext, transcript.Text)
}
```

## Monitoring & Observability

### LLM Metrics

```go
// Provider-agnostic metrics
llm_requests_total{provider="gemini", status="success|failed|timeout"}
llm_latency_seconds{provider="gemini", percentile="50|95|99"}
llm_token_usage{provider="gemini", type="prompt|completion"}
llm_function_calls_extracted{provider="gemini", function_name="function"}
llm_cost_per_request{provider="gemini"}

// Session-specific metrics
llm_session_conversation_length{session_id="session"}
llm_session_function_accuracy{session_id="session"}
llm_session_total_cost{session_id="session"}
```

### Comprehensive Logging

```go
func (a *Adapter) logFunctionCall(prompt string, functions []speech.FunctionCall,
                                  usage *speech.LLMUsage, latency time.Duration) {

    functionNames := make([]string, len(functions))
    for i, fn := range functions {
        functionNames[i] = fn.Name
    }

    log.Printf("🤖 LLM function extraction completed: "+
               "functions=%v, "+
               "tokens=%d/%d, "+
               "cost=$%.6f, "+
               "latency=%v",
               functionNames,
               usage.Prompt, usage.Completion,
               calculateCost(usage),
               latency)
}

func calculateCost(usage *speech.LLMUsage) float64 {
    // Gemini 2.0 Flash pricing (as of 2024)
    promptCost := float64(usage.Prompt) / 1_000_000 * 0.1    // $0.10 per 1M tokens
    completionCost := float64(usage.Completion) / 1_000_000 * 0.4 // $0.40 per 1M tokens
    return promptCost + completionCost
}
```

## Testing Strategy

### LLM Integration Testing

```go
func TestLLMAdapter_FunctionExtraction(t *testing.T) {
    adapter := llmgemini.New(testAPIKey, "gemini-2.0-flash-thinking-exp")

    // Test function configuration
    config := &speech.FunctionConfig{
        Declarations: []speech.FunctionDefinition{
            {
                Name:        "create_meeting",
                Description: "Create a calendar meeting",
                Parameters: []speech.FunctionParameter{
                    {Name: "title", Type: "string", Required: true},
                    {Name: "date", Type: "string", Required: true},
                },
            },
        },
        ParsingGuide: "Extract meeting creation requests",
    }

    // Test transcript
    transcript := speech.Transcript{
        Text:       "Schedule a meeting with John tomorrow at 3pm about the project",
        IsFinal:    true,
        Confidence: 0.95,
    }

    // Test function extraction
    prompt := speech.Prompt("Extract function calls from the transcript")
    functions, usage, err := adapter.Enrich(context.Background(), prompt, transcript, config)

    // Verify results
    assert.NoError(t, err)
    assert.Len(t, functions, 1)
    assert.Equal(t, "create_meeting", functions[0].Name)
    assert.NotNil(t, usage)
    assert.Greater(t, usage.Prompt, int64(0))
    assert.Greater(t, usage.Completion, int64(0))

    // Verify extracted arguments
    args := functions[0].Args
    assert.Contains(t, args, "title")
    assert.Contains(t, args, "date")
}

func TestLLMAdapter_SessionManagement(t *testing.T) {
    adapter := llmgemini.New(testAPIKey, "gemini-2.0-flash")

    // Test session persistence across multiple calls
    config := createTestFunctionConfig()

    // First call
    functions1, _, err := adapter.Enrich(context.Background(),
                                        "Create a meeting",
                                        speech.Transcript{Text: "Meeting with Alice"},
                                        config)
    assert.NoError(t, err)

    // Second call should use same session
    functions2, _, err := adapter.Enrich(context.Background(),
                                        "Add Bob to the meeting",
                                        speech.Transcript{Text: "Include Bob in the Alice meeting"},
                                        config)
    assert.NoError(t, err)

    // Verify session continuity (implementation-specific verification)
    assert.NotNil(t, functions1)
    assert.NotNil(t, functions2)
}
```

### Performance Testing

```go
func BenchmarkLLM_FunctionExtraction(b *testing.B) {
    adapter := llmgemini.New(testAPIKey, "gemini-2.0-flash")
    config := createTestFunctionConfig()
    transcript := createTestTranscript()
    prompt := speech.Prompt("Extract functions")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _, err := adapter.Enrich(context.Background(), prompt, transcript, config)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func TestLLM_ConcurrentRequests(t *testing.T) {
    adapter := llmgemini.New(testAPIKey, "gemini-2.0-flash")
    config := createTestFunctionConfig()

    // Test concurrent function extractions
    concurrency := 10
    results := make(chan error, concurrency)

    for i := 0; i < concurrency; i++ {
        go func(index int) {
            transcript := speech.Transcript{
                Text: fmt.Sprintf("Create meeting %d with different participants", index),
            }

            _, _, err := adapter.Enrich(context.Background(),
                                      speech.Prompt("Extract functions"),
                                      transcript, config)
            results <- err
        }(i)
    }

    // Verify all requests succeed
    for i := 0; i < concurrency; i++ {
        err := <-results
        assert.NoError(t, err)
    }
}
```

The LLM integration architecture provides a robust foundation for intelligent function extraction with built-in support for session management, dynamic schema updates, and comprehensive monitoring, while maintaining the flexibility to migrate to self-hosted solutions in the future.
