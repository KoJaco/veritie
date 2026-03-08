# Speech Processing Pipeline Architecture

## Overview

The Speech Processing Pipeline is the core component of Schma.ai that transforms real-time audio input into structured function calls. It orchestrates the flow of audio data through Speech-to-Text (STT), draft function detection, and Large Language Model (LLM) processing to produce accurate, structured output.

## Pipeline Flow

### High-Level Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Audio     │    │     STT     │    │ Fast Parser │    │     LLM     │
│   Input     │───▶│  Deepgram/  │───▶│   Draft     │───▶│   Gemini    │
│ (WebSocket) │    │   Google    │    │ Detection   │    │ Function    │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
                           │                   │                   │
                           ▼                   ▼                   ▼
                   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
                   │ Transcripts │    │   Draft     │    │  Function   │
                   │  (interim/  │    │ Functions   │    │   Calls     │
                   │   final)    │    │(similarity) │    │ (structured)│
                   └─────────────┘    └─────────────┘    └─────────────┘
                           │                   │                   │
                           └───────────────────┼───────────────────┘
                                               ▼
                                    ┌─────────────────┐
                                    │ Usage Tracking  │
                                    │ & Cost Metering │
                                    └─────────────────┘
```

### Detailed Component Flow

```
Audio Chunks ─┐
              │
              ▼
        ┌──────────────┐
        │ Ring Buffer  │ ◄─── (10-second circular buffer)
        └──────────────┘
              │
              ▼
        ┌──────────────┐
        │ STT Provider │ ◄─── Deepgram/Google routing
        │   Router     │
        └──────────────┘
              │
              ▼
      ┌─────────────────┐
      │   Transcripts   │
      │ (interim/final) │
      └─────────────────┘
              │
              ├─────────────────────────────────┐
              ▼                                 ▼
    ┌─────────────────┐              ┌─────────────────┐
    │ Spelling Cache  │              │ Draft Function  │
    │   (proper       │              │   Detector      │
    │    names)       │              │ (ML similarity) │
    └─────────────────┘              └─────────────────┘
              │                                 │
              ▼                                 ▼
    ┌─────────────────┐              ┌─────────────────┐
    │ Dynamic Prompt  │              │ Draft Function  │
    │  Construction   │              │    Output       │
    └─────────────────┘              └─────────────────┘
              │
              ▼
        ┌──────────────┐
        │ LLM (Gemini) │ ◄─── Session management & tools
        │  Function    │
        │  Calling     │
        └──────────────┘
              │
              ▼
      ┌─────────────────┐
      │ Structured      │
      │ Function Calls  │
      └─────────────────┘
```

## Core Components

### 1. Pipeline Orchestrator (`internal/app/pipeline/pipeline.go`)

The main Pipeline struct coordinates the entire flow:

```go
type Pipeline struct {
    cfg              Config
    deps             Deps
    usageAccumulator *usage.UsageAccumulator

    // Output channels
    outTr     chan speech.Transcript
    outFns    chan []speech.FunctionCall
    outDrafts chan speech.FunctionCall

    // Internal state
    knownDraft    map[string]float64 // draft similarity tracking
    spellingCache map[string]string  // proper name mapping
    bufferedTr    string             // transcript buffer
}
```

**Key Responsibilities:**

-   **Audio Flow Management**: Coordinates audio chunk processing
-   **Channel Orchestration**: Manages output channels for different data types
-   **State Tracking**: Maintains draft function and spelling state
-   **Usage Accounting**: Integrates with usage accumulator for billing
-   **Error Handling**: Manages pipeline failures and recovery

### 2. Dependencies Injection (`internal/app/pipeline/deps.go`)

Clean dependency injection pattern for external services:

```go
type Deps struct {
    STT            speech.STTClient        // Speech-to-Text provider
    FP             speech.FastParser       // ML-based draft detection
    LLM            speech.LLM              // Language model interface
    UsageMeterRepo domain_usage.UsageMeterRepo
    UsageEventRepo domain_usage.UsageEventRepo
    DraftAggRepo   domain_usage.DraftAggRepo
}
```

**Benefits:**

-   **Testability**: Easy to mock dependencies for testing
-   **Flexibility**: Swap implementations without changing pipeline
-   **Clean Architecture**: Domain interfaces, infrastructure implementations

### 3. Configuration (`internal/app/pipeline/config.go`)

Pipeline configuration encapsulates session and processing parameters:

```go
type Config struct {
    SessionID string
    AccountID string
    AppID     string
    Pricing   usage.Pricing

    // Function calling configuration
    Prompt            speech.Prompt
    FuncCfg          *speech.FunctionConfig
    InputGuide       string
    PrevFunctionsJSON string

    // Draft detection
    DraftIndex *draft.Index
}
```

## Processing Stages

### Stage 1: Audio Input & Buffering

**Input**: Raw audio chunks from WebSocket
**Processing**:

-   Audio chunks written to ring buffer for fallback replay
-   Lazy connection pattern waits for first chunk before starting STT
-   Chunks forwarded to STT provider via router

**Code Flow**:

```go
// Wait for first chunk, then create proxy channel
first, toSTT, ok := lazyConnect(upstream)
if !ok {
    log.Printf("pipeline: upstream closed before any audio")
    return
}

// Start STT with audio chunks
sttOut, err := p.deps.STT.Stream(ctx, toSTT)
```

### Stage 2: Speech-to-Text Processing

**Input**: Audio chunks
**Output**: Transcript objects (interim and final)
**Processing**:

-   STT provider (Deepgram/Google) converts audio to text
-   Both interim and final transcripts are emitted
-   Usage tracking records audio duration and provider

**Transcript Structure**:

```go
type Transcript struct {
    Text         string
    IsFinal      bool
    Confidence   float32
    Words        []Word
    ChunkDurSec  float64  // for usage tracking
}
```

### Stage 3: Text Enhancement & Spelling

**Input**: Raw transcripts
**Output**: Enhanced text with proper names
**Processing**:

-   Transcript text accumulated in buffered cache
-   Spelling extraction identifies proper names (e.g., "John Doe")
-   Spelling cache maps lowercase → proper case
-   Cache used for LLM prompt enhancement

**Spelling Cache Example**:

```go
// Extract proper names from buffered transcript
names := spelling.ExtractSpelledNames(p.bufferedTr)
for lower, proper := range names {
    p.spellingCache[lower] = proper
    // "john doe" -> "John Doe"
}
```

### Stage 4: Draft Function Detection

**Input**: Interim transcripts
**Output**: Draft function calls with similarity scores
**Processing**:

-   ML-based similarity matching against function definitions
-   Only sends draft if new or better similarity score
-   Runs on interim transcripts for real-time feedback

**Draft Detection Logic**:

```go
if !tr.IsFinal && p.cfg.DraftIndex != nil {
    if d := p.cfg.DraftIndex.Detect(tr.Text); d != nil {
        best, seen := p.knownDraft[d.Name]

        // Send only if new or strictly better
        if !seen || d.SimilarityScore > best {
            p.knownDraft[d.Name] = d.SimilarityScore
            p.outDrafts <- *d
        }
    }
}
```

### Stage 5: LLM Function Generation

**Input**: Final transcripts + function configuration
**Output**: Structured function calls
**Processing**:

-   Triggers only on final transcripts
-   Throttling prevents excessive LLM calls
-   Dynamic prompt construction with spelling enhancement
-   Function call merging with previous state

**LLM Processing Flow**:

```go
if tr.IsFinal && p.deps.LLM != nil && p.cfg.FuncCfg != nil {
    // Throttling check
    if time.Since(lastLLM) < throttleWindow {
        continue
    }

    // Build enhanced prompt
    dynPrompt := prompts.BuildFunctionParsingPrompt(tr.Text, p.spellingCache)

    // Call LLM
    calls, usage, err := p.deps.LLM.Enrich(ctx, prompt, tr, p.cfg.FuncCfg)

    // Merge with previous function state
    merged := fnutil.MergeUpdate(prevCalls, calls)
    if !reflect.DeepEqual(merged, prevCalls) {
        p.outFns <- merged
        prevCalls = merged
    }
}
```

## Output Channels

The pipeline produces three types of output via dedicated channels:

### 1. Transcript Channel (`outTr`)

```go
type TranscriptMessage struct {
    Type       string        `json:"type"`     // "transcript"
    Text       string        `json:"text"`
    Final      bool          `json:"final"`
    Confidence float32       `json:"confidence"`
    Words      []speech.Word `json:"words"`
}
```

**Use Cases:**

-   Real-time transcript display
-   Session recording and analysis
-   Quality assurance and debugging

### 2. Function Calls Channel (`outFns`)

```go
type FunctionMessage struct {
    Type      string                `json:"type"`  // "functions"
    Functions []speech.FunctionCall `json:"functions"`
}
```

**Use Cases:**

-   Structured action execution
-   API integrations
-   Workflow automation

### 3. Draft Functions Channel (`outDrafts`)

```go
type DraftFunctionMessage struct {
    Type  string              `json:"type"`  // "function_draft_extracted"
    Draft speech.FunctionCall `json:"draft_function"`
}
```

**Use Cases:**

-   Real-time user feedback
-   Intent prediction
-   UI suggestions and hints

## Usage Tracking Integration

The pipeline integrates comprehensive usage tracking:

### STT Usage

```go
if tr.ChunkDurSec > 0 {
    provider := "deepgram" // or "google"
    p.usageAccumulator.AddSTT(tr.ChunkDurSec, provider)
}
```

### LLM Usage

```go
if usage != nil {
    provider := "gemini"
    model := "gemini-2.0-flash"
    p.usageAccumulator.AddLLM(usage.Prompt, usage.Completion, provider, model)
}
```

### Draft Function Analytics

```go
// Tracked in WebSocket handler when drafts are sent
usageAccumulator.AddDraftFunction(draft.Name, draft.SimilarityScore, draftArgs)
```

## Performance Optimizations

### 1. Lazy Connection Pattern

```go
func lazyConnect(upstream <-chan speech.AudioChunk) (
    first speech.AudioChunk,
    toSTT chan speech.AudioChunk,
    ok bool,
) {
    // Wait for first chunk before starting STT
    first, ok = <-upstream
    if !ok {
        return
    }

    // Create proxy channel and relay chunks
    toSTT = make(chan speech.AudioChunk, 64)
    go func() {
        toSTT <- first // send buffered first chunk
        for c := range upstream {
            toSTT <- c
        }
        close(toSTT)
    }()

    return
}
```

**Benefits:**

-   No audio loss during STT provider initialization
-   Handles slow STT service startup gracefully
-   Buffered channel prevents backpressure

### 2. LLM Throttling

```go
// Throttle LLM calls to prevent cost explosion
if gap := time.Duration(p.cfg.FuncCfg.UpdateMs) * time.Millisecond; gap > 0 {
    if time.Since(lastLLM) < gap {
        continue // skip this final transcript
    }
}
lastLLM = time.Now()
```

**Benefits:**

-   Cost control for high-frequency transcripts
-   Reduces LLM provider load
-   Configurable per-session throttling

### 3. Draft Similarity Deduplication

```go
best, seen := p.knownDraft[d.Name]
if !seen || d.SimilarityScore > best {
    p.knownDraft[d.Name] = d.SimilarityScore
    p.outDrafts <- *d
} else {
    // Skip inferior draft predictions
}
```

**Benefits:**

-   Reduces noise in draft function output
-   Progressive improvement in predictions
-   Better user experience with quality filtering

## Error Handling

### Pipeline Resilience

```go
go func() {
    defer close(p.outTr)
    defer close(p.outFns)
    defer close(p.outDrafts)

    // Ensure all channels are closed on exit
    // Prevents goroutine leaks and deadlocks
}()
```

### STT Connection Failures

```go
sttOut, err := p.deps.STT.Stream(ctx, toSTT)
if err != nil {
    // Log error and terminate pipeline gracefully
    return
}
```

### LLM Call Failures

```go
calls, usage, err := p.deps.LLM.Enrich(ctx, prompt, tr, p.cfg.FuncCfg)
if err == nil && len(calls) > 0 {
    // Only process successful LLM responses
    // Failed calls don't affect pipeline state
}
```

## Testing Strategy

### Unit Testing

```go
func TestPipeline_ProcessesTranscripts(t *testing.T) {
    // Mock dependencies
    mockSTT := &MockSTTClient{}
    mockLLM := &MockLLMClient{}

    // Create pipeline with mocks
    pipeline, err := New(testConfig, Deps{
        STT: mockSTT,
        LLM: mockLLM,
    })

    // Test transcript processing
    // Verify output channels receive expected data
}
```

### Integration Testing

```go
func TestPipeline_EndToEnd(t *testing.T) {
    // Test with real STT and LLM providers
    // Verify complete audio -> function call flow
    // Measure performance and resource usage
}
```

### Performance Testing

```go
func BenchmarkPipeline_Throughput(b *testing.B) {
    // Measure audio processing throughput
    // Test concurrent session capacity
    // Profile memory and CPU usage
}
```

## Configuration Options

### Function Call Configuration

```go
type FunctionConfig struct {
    ParsingStrategy   string                // "realtime"
    UpdateMs int                   // LLM throttling
    Declarations      []FunctionDefinition  // Available functions
    ParsingGuide      string                // LLM context hints
}
```

### Usage Pricing Configuration

```go
type Pricing struct {
    CostAudioPerMin   float64  // STT cost per minute
    CostPromptPer1K   float64  // LLM prompt token cost
    CostCompletionPer1K float64 // LLM completion token cost
}
```

## Monitoring & Observability

### Key Metrics

-   **Pipeline Latency**: Time from audio input to function output
-   **STT Processing Time**: Speech-to-text conversion duration
-   **LLM Response Time**: Function generation latency
-   **Draft Accuracy**: Similarity score distributions
-   **Channel Utilization**: Buffer sizes and backpressure

### Logging Examples

```
🔄 Pipeline started for session abc123 (functions=5, drafts=enabled)
📝 STT transcript: "create a meeting with john" (final=true, confidence=0.95)
🎯 Draft detected: create_meeting (similarity=0.87)
🤖 LLM response: 1 function call (latency=245ms, tokens=15/45)
💰 Usage tracked: 2.3s audio, 15 prompt tokens, 45 completion tokens
📊 Pipeline complete: 3.2s total latency, $0.0012 cost
```

The Speech Processing Pipeline forms the backbone of Schma.ai's real-time speech-to-function capabilities, providing reliable, scalable, and cost-effective audio processing with comprehensive monitoring and error handling.
