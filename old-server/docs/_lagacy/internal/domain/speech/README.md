# Speech Domain Documentation

## Overview

The speech domain defines the core contracts and data structures for speech processing in the Schma system. It provides provider-agnostic interfaces for Speech-to-Text (STT), Language Model (LLM) integration, parsing, and caching.

## Core Concepts

### Audio Processing

-   **AudioChunk**: Raw 16-bit PCM audio data
-   **Transcript**: Provider-agnostic text output with timing and confidence
-   **Word-level data**: Detailed timing and confidence for individual words
-   **Speaker diarization**: Multi-speaker detection and separation

### Language Model Integration

-   **Function calling**: Structured extraction of function calls from speech
-   **Structured output**: Schema-constrained JSON generation
-   **Caching**: Optimization of LLM calls through context caching
-   **Usage tracking**: Token consumption and cost monitoring

## Data Structures

### AudioChunk

```go
type AudioChunk []byte
```

Raw 16-bit PCM audio data in little-endian format, single channel.

### Transcript

```go
type Transcript struct {
    Text        string  `json:"text"`
    IsFinal     bool    `json:"final"`
    Confidence  float32 `json:"confidence,omitempty"`
    Stability   float32 `json:"stability,omitempty"`
    Words       []Word  `json:"words,omitempty"`
    ChunkDurSec float64 `json:"chunk_dur_sec,omitempty"`
    Turns       []Turn  `json:"turns,omitempty"`
    Channel     int     `json:"channel,omitempty"`
}
```

**Fields:**

-   `Text`: The transcribed text content
-   `IsFinal`: Whether this is a final or interim transcript
-   `Confidence`: Overall confidence score (0.0-1.0)
-   `Stability`: Stability score for streaming transcripts
-   `Words`: Word-level timing and confidence data
-   `ChunkDurSec`: Duration of the audio chunk in seconds
-   `Turns`: Speaker diarization data (optional)
-   `Channel`: Audio channel number for multi-channel audio

### Word

```go
type Word struct {
    Text               string  `json:"text"`
    Start              float32 `json:"start"`
    End                float32 `json:"end"`
    Confidence         float32 `json:"confidence,omitempty"`
    PunctuatedWord     string  `json:"punctuated_word,omitempty"`
    Speaker            string  `json:"speaker,omitempty"`
    SpeakerConfidence  float32 `json:"speaker_confidence,omitempty"`
}
```

**Fields:**

-   `Text`: The word text
-   `Start/End`: Timing in seconds from start of audio
-   `Confidence`: Word-level confidence score
-   `PunctuatedWord`: Word with punctuation applied
-   `Speaker`: Speaker identifier (for diarization)
-   `SpeakerConfidence`: Confidence in speaker assignment

### Turn

```go
type Turn struct {
    ID          string  `json:"id"`
    Speaker     string  `json:"speaker"`
    Start       float32 `json:"start"`
    End         float32 `json:"end"`
    Words       []Word  `json:"words,omitempty"`
    Confidence  float32 `json:"confidence,omitempty"`
    Final       bool    `json:"final,omitempty"`
}
```

Represents a speaker turn in multi-speaker conversations.

## Interfaces

### STTClient

```go
type STTClient interface {
    Stream(ctx context.Context, in <-chan AudioChunk) (<-chan Transcript, error)
}
```

**Purpose**: Converts audio streams to text transcripts.

**Methods:**

-   `Stream`: Starts a bidirectional streaming session
    -   `in`: Raw PCM audio chunks from microphone
    -   Returns: Channel of incremental transcripts
    -   Closes when context is cancelled

**Implementations:**

-   `infra/sttdeepgram`: Deepgram STT provider
-   `infra/sttgoogle`: Google Speech-to-Text provider

### LLM

```go
type LLM interface {
    Enrich(ctx context.Context, prompt Prompt, partial Transcript, cfg *FunctionConfig) ([]FunctionCall, *LLMUsage, error)
}
```

**Purpose**: Extracts structured function calls from transcripts.

**Methods:**

-   `Enrich`: Processes a transcript to extract function calls
    -   `prompt`: System prompt and instructions
    -   `partial`: Current transcript to process
    -   `cfg`: Function configuration and schema
    -   Returns: Extracted function calls, usage metrics, and error

### StructuredLLM

```go
type StructuredLLM interface {
    GenerateStructured(ctx context.Context, prompt Prompt, partial Transcript, cfg *StructuredConfig) (map[string]any, *LLMUsage, error)
}
```

**Purpose**: Generates structured JSON output according to a schema.

**Methods:**

-   `GenerateStructured`: Creates structured data from transcripts
    -   `prompt`: System prompt and instructions
    -   `partial`: Current transcript to process
    -   `cfg`: Structured output configuration and schema
    -   Returns: Structured JSON object, usage metrics, and error

### FastParser

```go
type FastParser interface {
    Embed(ids, mask []int64) []float32
    Synonyms(word string, k int) []string
}
```

**Purpose**: Provides fast local inference for draft function detection.

**Methods:**

-   `Embed`: Creates dense vector embeddings for sentences
-   `Synonyms`: Returns semantic synonyms for words

## Configuration Types

### FunctionConfig

```go
type FunctionConfig struct {
    Name              string                      `json:"name,omitempty"`
    Description       string                      `json:"description,omitempty"`
    ParsingStrategy   string                      // "real-time" or "end-of-session"
    UpdateMs int                         `json:"update_frequency,omitempty"`
    Declarations      []FunctionDefinition        // Model's Tools list
    ParsingGuide      string                      // free-text system guide
    PrevContext       []FunctionCall              // Earlier calls for context
}
```

**Fields:**

-   `Name/Description`: Optional labels for the configuration
-   `ParsingStrategy`: When to extract functions ("real-time" or "end-of-session")
-   `UpdateMs`: Throttling window for real-time parsing
-   `Declarations`: Function definitions (tools schema)
-   `ParsingGuide`: Free-text instructions for the LLM
-   `PrevContext`: Previous function calls for context

### StructuredConfig

```go
type StructuredConfig struct {
    Schema       json.RawMessage // JSON Schema (draft-07+)
    ParsingGuide string          // system/guide text for structured extraction
    UpdateMS     int             // throttle window for realtime calls
}
```

**Fields:**

-   `Schema`: JSON Schema defining the output structure
-   `ParsingGuide`: System instructions for structured extraction
-   `UpdateMS`: Throttling window for real-time updates

## Caching System

### LLMCache

```go
type LLMCache interface {
    Store(ctx context.Context, context StaticContext) (CacheKey, error)
    Get(ctx context.Context, key CacheKey) (StaticContext, error)
    Delete(ctx context.Context, key CacheKey) error
    Invalidate(ctx context.Context, key CacheKey) error
    IsValid(ctx context.Context, key CacheKey) bool
    Clear() error
    Close() error
    IsAvailable() bool
    IsCorrupt() bool
    IsExpired() bool
}
```

**Purpose**: Caches static LLM context to reduce token usage and latency.

**Key Concepts:**

-   **StaticContext**: Immutable context (function schemas, system prompts)
-   **CacheKey**: Unique identifier for cached contexts
-   **Cache Errors**: Detailed error types for different failure modes

### CachePreparer

```go
type CachePreparer interface {
    PrepareCache(ctx context.Context, cfg *FunctionConfig) (CacheKey, error)
}
```

**Purpose**: Proactively prepares cache entries for new configurations.

## Audio Buffering

### RingBuffer

```go
type RingBuffer interface {
    Write(chunk AudioChunk)
    ReadLast(duration time.Duration) []AudioChunk
    ReadAll() []AudioChunk
    Clear()
    Size() int
    Duration() time.Duration
    Capacity() time.Duration
}
```

**Purpose**: Provides circular audio buffering for fallback replay and error recovery.

**Use Cases:**

-   **Fallback replay**: Replay audio when STT fails
-   **Error recovery**: Retry processing with buffered audio
-   **Debugging**: Capture audio for analysis

## Usage Examples

### Basic STT Processing

```go
// Create STT client
sttClient := deepgram.NewClient(config)

// Start streaming session
audioChunks := make(chan AudioChunk)
transcripts, err := sttClient.Stream(ctx, audioChunks)

// Process transcripts
for transcript := range transcripts {
    if transcript.IsFinal {
        // Process final transcript
        processTranscript(transcript)
    }
}
```

### Function Extraction

```go
// Configure function extraction
config := &speech.FunctionConfig{
    ParsingStrategy: "real-time",
    UpdateMs: 1000,
    Declarations: []speech.FunctionDefinition{
        {
            Name: "create_task",
            Parameters: []speech.FunctionParam{
                {Name: "title", Type: "string", Required: true},
                {Name: "priority", Type: "string", Enum: []string{"low", "medium", "high"}},
            },
        },
    },
    ParsingGuide: "Extract task creation requests from speech",
}

// Extract functions
calls, usage, err := llm.Enrich(ctx, prompt, transcript, config)
```

### Structured Output

```go
// Configure structured output
config := &speech.StructuredConfig{
    Schema: json.RawMessage(`{
        "type": "object",
        "properties": {
            "name": {"type": "string"},
            "email": {"type": "string", "format": "email"}
        },
        "required": ["name"]
    }`),
    ParsingGuide: "Extract contact information from speech",
    UpdateMS: 2000,
}

// Generate structured data
data, usage, err := structuredLLM.GenerateStructured(ctx, prompt, transcript, config)
```

## Error Handling

### Cache Errors

The speech domain defines specific cache error types:

-   `CacheUnavailable`: Cache service is down
-   `CacheInvalid`: Cache data is invalid
-   `CacheExpired`: Cache entry has expired
-   `CacheCorrupt`: Cache data is corrupted
-   `CacheMiss`: Requested key not found
-   `CacheHit`: Successful cache retrieval
-   `CacheFull`: Cache storage is full
-   `CacheSizeLimitExceeded`: Entry exceeds size limits
-   `CacheInvalidationFailed`: Failed to invalidate cache

## Testing

The speech domain interfaces are designed for easy testing:

-   **No external dependencies**: All interfaces are pure Go
-   **Mockable**: Easy to create test doubles
-   **Deterministic**: Same inputs produce same outputs
-   **Fast**: No network calls or heavy computation

## Implementation Notes

### Provider Agnosticism

All interfaces are designed to be provider-agnostic:

-   **STT**: Works with Deepgram, Google, or any STT provider
-   **LLM**: Works with Gemini, OpenAI, or any LLM provider
-   **Caching**: Works with Redis, in-memory, or any cache provider

### Performance Considerations

-   **Streaming**: All interfaces support streaming for real-time processing
-   **Caching**: LLM context caching reduces token usage
-   **Buffering**: Audio buffering enables error recovery
-   **Throttling**: Configurable update frequencies prevent spam

### Extensibility

The domain is designed for easy extension:

-   **New STT providers**: Implement `STTClient` interface
-   **New LLM providers**: Implement `LLM` or `StructuredLLM` interfaces
-   **New cache providers**: Implement `LLMCache` interface
-   **New parsing strategies**: Extend `FunctionConfig` and `StructuredConfig`
