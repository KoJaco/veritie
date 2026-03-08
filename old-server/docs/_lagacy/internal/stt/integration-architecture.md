# STT Integration Architecture

## Overview

The Speech-to-Text (STT) integration provides a unified, swappable interface for multiple STT providers with seamless configuration-based switching. The system supports real-time streaming transcription with provider-agnostic output formats, enabling easy migration between STT services based on domain requirements, cost optimization, or quality preferences.

## Core Architecture

### Provider-Agnostic Design

```
Application Layer → STT Router → Provider Implementation → External STT API
                         ↓              ↓                      ↓
                    Unified Interface  Adapter Pattern    Provider-Specific
```

### Component Diagram

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Pipeline      │    │   STT Router    │    │   Deepgram      │
│  (Speech        │───▶│  (Provider      │───▶│   Client        │
│   Processing)   │    │   Selection)    │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       │                       ▼
┌─────────────────┐              │              ┌─────────────────┐
│   Domain        │              │              │   WebSocket     │
│  Interface      │              │              │   Streaming     │
│ (STTClient)     │              │              │                 │
└─────────────────┘              │              └─────────────────┘
                                  │
                                  ▼
                         ┌─────────────────┐    ┌─────────────────┐
                         │   Google        │    │   gRPC          │
                         │   Client        │───▶│   Streaming     │
                         │                 │    │                 │
                         └─────────────────┘    └─────────────────┘
```

## Domain Interface

### Core STT Contract (`internal/domain/speech/`)

```go
// STTClient defines the unified interface all STT providers must implement
type STTClient interface {
    // Stream processes audio chunks and returns transcript channel
    Stream(ctx context.Context, audio <-chan AudioChunk) (<-chan Transcript, error)
}

// Provider-agnostic data structures
type AudioChunk []byte

type Transcript struct {
    Text        string    `json:"text"`         // Transcribed text
    IsFinal     bool      `json:"is_final"`     // Whether transcript is finalized
    Confidence  float32   `json:"confidence"`   // Overall confidence (0.0-1.0)
    Stability   float32   `json:"stability"`    // Interim result stability
    ChunkDurSec float64   `json:"chunk_duration"` // Audio duration in seconds
    Words       []Word    `json:"words"`        // Word-level details
}

type Word struct {
    Text       string  `json:"text"`        // Word text
    Start      float32 `json:"start"`       // Start time in seconds
    End        float32 `json:"end"`         // End time in seconds
    Confidence float32 `json:"confidence"`  // Word-level confidence
}
```

**Key Design Principles:**

-   **Provider Agnostic**: Same interface regardless of underlying STT service
-   **Streaming First**: Real-time audio processing with immediate results
-   **Structured Output**: Consistent transcript format across all providers
-   **Word-Level Details**: Timing and confidence for advanced processing

## STT Router Implementation

### Router Core (`internal/infra/sttrouter/router.go`)

The router provides transparent provider switching through configuration:

```go
type Router struct {
    impl speech.STTClient // Underlying provider implementation
    name string           // Provider name for logging/metrics
}

func New(ctx context.Context, gCfg sttgoogle.Config) *Router {
    // Read provider from environment
    provider := strings.ToLower(os.Getenv("SCHMA_STT_PROVIDER"))
    if provider == "" {
        provider = "deepgram" // Default provider
    }

    var impl speech.STTClient
    var err error

    switch provider {
    case "google":
        impl, err = sttgoogle.New(ctx, gCfg)
        if err != nil {
            log.Fatalf("Failed to initialize Google STT: %v", err)
        }

    case "deepgram":
        apiKey := os.Getenv("DEEPGRAM_API_KEY")
        impl = sttdeepgram.New(apiKey, "nova-3")

    default:
        log.Fatalf("Unknown STT provider: %s (supported: google, deepgram)", provider)
    }

    log.Printf("🔊 STT router initialized with provider: %s", provider)
    return &Router{impl: impl, name: provider}
}

// Stream proxies to the underlying provider implementation
func (r *Router) Stream(ctx context.Context, audio <-chan speech.AudioChunk) (<-chan speech.Transcript, error) {
    return r.impl.Stream(ctx, audio)
}
```

**Router Benefits:**

-   **Zero-Downtime Switching**: Change providers with configuration only
-   **Transparent Proxying**: Application code remains unchanged
-   **Provider Abstraction**: Unified interface hides implementation details
-   **Error Isolation**: Provider-specific errors are handled consistently

### Configuration Management

```bash
# Environment Variables for Provider Selection
SCHMA_STT_PROVIDER=deepgram  # or "google"

# Provider-specific credentials (only required for selected provider)
DEEPGRAM_API_KEY=your_deepgram_key     # For Deepgram
GOOGLE_CREDENTIALS=path/to/creds.json  # For Google Cloud STT
```

**Configuration Strategy:**

-   **Single Source**: One environment variable controls all provider selection
-   **Conditional Credentials**: Only load credentials for active provider
-   **Fail-Fast**: Clear error messages for missing configuration
-   **Default Fallback**: Sensible default (Deepgram) when not specified

## Provider Implementations

### Deepgram Client (`internal/infra/sttdeepgram/client.go`)

WebSocket-based streaming implementation:

```go
type Client struct {
    apiKey     string
    model      string                    // e.g., "nova-3"
    dialer     *websocket.Dialer
    HTTPClient *http.Client
}

func New(apiKey, model string) *Client {
    return &Client{
        apiKey: apiKey,
        model:  model,
        dialer: &websocket.Dialer{
            Proxy:            http.ProxyFromEnvironment,
            HandshakeTimeout: 10 * time.Second,
        },
        HTTPClient: &http.Client{Timeout: 120 * time.Second},
    }
}

func (c *Client) Stream(ctx context.Context, audio <-chan speech.AudioChunk) (<-chan speech.Transcript, error) {
    // Build WebSocket URL with parameters
    params := url.Values{
        "model":           []string{c.model},
        "interim_results": []string{"true"},
        "punctuate":       []string{"true"},
        "endpointing":     []string{"1000"}, // Reduce premature finals
    }

    wsURL := deepgramWS + "?" + params.Encode()
    headers := http.Header{"Authorization": []string{"Token " + c.apiKey}}

    // Establish WebSocket connection
    ws, _, err := c.dialer.DialContext(ctx, wsURL, headers)
    if err != nil {
        return nil, fmt.Errorf("deepgram websocket connection failed: %w", err)
    }

    out := make(chan speech.Transcript, 32)
    done := make(chan struct{})

    // Start bidirectional goroutines
    go uplink(ctx, ws, audio, done)   // Send audio chunks
    go downlink(ctx, ws, out, done)   // Receive transcripts

    return out, nil
}
```

#### Bidirectional Communication

**Uplink (Audio → Deepgram):**

```go
func uplink(ctx context.Context, ws *websocket.Conn, audio <-chan speech.AudioChunk, done chan struct{}) {
    defer ws.Close()

    for {
        select {
        case <-ctx.Done():
            return
        case <-done:
            return
        case chunk, ok := <-audio:
            if !ok {
                // Send stop message when audio stream ends
                ws.WriteMessage(websocket.TextMessage, mustJSON(map[string]any{"type": "stop"}))
                return
            }

            // Send binary audio data
            if err := ws.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
                log.Printf("Deepgram uplink error: %v", err)
                close(done)
                return
            }
        }
    }
}
```

**Downlink (Deepgram → Transcripts):**

```go
func downlink(ctx context.Context, ws *websocket.Conn, out chan<- speech.Transcript, done chan struct{}) {
    defer close(out)
    defer close(done)
    defer ws.Close()

    for {
        _, data, err := ws.ReadMessage()
        if err != nil {
            log.Printf("Deepgram downlink error: %v", err)
            return
        }

        // Parse Deepgram response format
        var event struct {
            Type    string `json:"type"`
            Channel struct {
                Alternatives []struct {
                    Transcript string  `json:"transcript"`
                    Confidence float32 `json:"confidence"`
                    Words      []struct {
                        Word       string  `json:"word"`
                        Start      float32 `json:"start"`
                        End        float32 `json:"end"`
                        Confidence float32 `json:"confidence"`
                    } `json:"words"`
                } `json:"alternatives"`
            } `json:"channel"`
            IsFinal bool `json:"is_final"`
        }

        if json.Unmarshal(data, &event) != nil || event.Type != "Results" {
            continue
        }

        // Convert to domain format
        for _, alt := range event.Channel.Alternatives {
            if alt.Transcript == "" {
                continue // Skip empty transcripts
            }

            tr := speech.Transcript{
                Text:       alt.Transcript,
                IsFinal:    event.IsFinal,
                Confidence: alt.Confidence,
            }

            // Map word timings
            for _, w := range alt.Words {
                tr.Words = append(tr.Words, speech.Word{
                    Text:       w.Word,
                    Start:      w.Start,
                    End:        w.End,
                    Confidence: w.Confidence,
                })
            }

            out <- tr
        }
    }
}
```

### Google Cloud STT Client (`internal/infra/sttgoogle/client.go`)

gRPC-based streaming implementation:

```go
type Config struct {
    Encoding        speechpb.RecognitionConfig_AudioEncoding
    SampleRateHertz int32
    LanguageCode    string
    Punctuate       bool
}

type Client struct {
    cfg Config
    svc *gstt.Client // Google Cloud STT gRPC client
}

func New(ctx context.Context, cfg Config) (*Client, error) {
    // Create Google Cloud STT client with application default credentials
    svc, err := gstt.NewClient(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to create Google STT client: %w", err)
    }

    return &Client{cfg: cfg, svc: svc}, nil
}

func (c *Client) Stream(ctx context.Context, audio <-chan speech.AudioChunk) (<-chan speech.Transcript, error) {
    // Create streaming recognize gRPC stream
    stream, err := c.svc.StreamingRecognize(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to create Google STT stream: %w", err)
    }

    // Send initial configuration
    err = stream.Send(&speechpb.StreamingRecognizeRequest{
        StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
            StreamingConfig: &speechpb.StreamingRecognitionConfig{
                Config: &speechpb.RecognitionConfig{
                    Encoding:                   c.cfg.Encoding,
                    SampleRateHertz:            c.cfg.SampleRateHertz,
                    LanguageCode:               c.cfg.LanguageCode,
                    EnableAutomaticPunctuation: c.cfg.Punctuate,
                    EnableWordTimeOffsets:      true,
                },
                InterimResults:  true,
                SingleUtterance: false,
            },
        },
    })

    if err != nil {
        return nil, fmt.Errorf("failed to send Google STT config: %w", err)
    }

    out := make(chan speech.Transcript, 32)

    // Uplink: Send audio chunks
    go func() {
        defer stream.CloseSend()
        for chunk := range audio {
            stream.Send(&speechpb.StreamingRecognizeRequest{
                StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
                    AudioContent: chunk,
                },
            })
        }
    }()

    // Downlink: Receive transcripts
    go func() {
        defer close(out)
        for {
            resp, err := stream.Recv()
            if err == io.EOF || err != nil {
                return
            }

            // Convert Google format to domain format
            for _, result := range resp.Results {
                for _, alt := range result.Alternatives {
                    tr := speech.Transcript{
                        Text:       alt.Transcript,
                        IsFinal:    result.IsFinal,
                        Confidence: alt.Confidence,
                        Stability:  result.Stability,
                    }

                    // Calculate audio duration from word timings
                    if len(alt.Words) > 0 {
                        first := alt.Words[0].StartTime.AsDuration()
                        last := alt.Words[len(alt.Words)-1].EndTime.AsDuration()
                        tr.ChunkDurSec = (last - first).Seconds()
                    }

                    // Map word details
                    for _, w := range alt.Words {
                        tr.Words = append(tr.Words, speech.Word{
                            Text:       w.Word,
                            Start:      float32(w.StartTime.AsDuration().Seconds()),
                            End:        float32(w.EndTime.AsDuration().Seconds()),
                            Confidence: w.Confidence,
                        })
                    }

                    out <- tr
                }
            }
        }
    }()

    return out, nil
}
```

## Batch Processing Support

### Deepgram Batch Client

For asynchronous file processing:

```go
func (c *Client) TranscribeFile(ctx context.Context, audioReader io.Reader) (speech.Transcript, error) {
    url := "https://api.deepgram.com/v1/listen?model=nova-3&smart_format=true"

    req, err := http.NewRequestWithContext(ctx, "POST", url, audioReader)
    if err != nil {
        return speech.Transcript{}, err
    }

    req.Header.Set("Content-Type", "audio/wav")
    req.Header.Set("Authorization", "Token "+c.apiKey)

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return speech.Transcript{}, err
    }
    defer resp.Body.Close()

    // Parse batch response format
    var batchResp struct {
        Results struct {
            Channels []struct {
                Alternatives []struct {
                    Transcript string  `json:"transcript"`
                    Confidence float32 `json:"confidence"`
                    Words      []struct {
                        Word           string  `json:"word"`
                        Start          float32 `json:"start"`
                        End            float32 `json:"end"`
                        Confidence     float32 `json:"confidence"`
                        PunctuatedWord string  `json:"punctuated_word"`
                    } `json:"words"`
                } `json:"alternatives"`
            } `json:"channels"`
        } `json:"results"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
        return speech.Transcript{}, err
    }

    // Collapse multi-channel response into single transcript
    var transcript speech.Transcript
    for _, channel := range batchResp.Results.Channels {
        for _, alt := range channel.Alternatives {
            transcript.Text += alt.Transcript + " "
            transcript.Confidence += alt.Confidence

            for _, w := range alt.Words {
                transcript.Words = append(transcript.Words, speech.Word{
                    Text:       w.Word,
                    Start:      w.Start,
                    End:        w.End,
                    Confidence: w.Confidence,
                })
            }
        }
    }

    transcript.IsFinal = true
    return transcript, nil
}
```

## Provider Selection Strategy

### Domain-Specific Optimization

```go
// Provider recommendation based on domain requirements
func RecommendProvider(domain string, language string, realtime bool) string {
    switch {
    case domain == "medical" && language == "en-US":
        return "google" // Superior medical terminology accuracy

    case domain == "legal" && language == "en-US":
        return "deepgram" // Better legal vocabulary and proper nouns

    case strings.HasPrefix(language, "en-"):
        return "deepgram" // Generally faster for English variants

    case realtime && domain == "conversational":
        return "deepgram" // Lower latency for real-time chat

    default:
        return "google" // Broader language support
    }
}
```

### Cost Optimization

```go
type ProviderCosts struct {
    Deepgram struct {
        PerMinute float64 // $0.0025/minute for Nova-3
        Features  []string // Real-time, punctuation, diarization
    }

    Google struct {
        PerMinute float64 // $0.0024/minute for enhanced
        Features  []string // Advanced ML, language detection
    }
}

// Dynamic provider selection based on cost and volume
func SelectProviderForVolume(monthlyMinutes int) string {
    if monthlyMinutes > 100000 {
        return "deepgram" // Better enterprise pricing
    }
    return "google" // Better pay-as-you-go rates
}
```

## Pipeline Integration

### Seamless Provider Switching

```go
// In pipeline initialization
func NewPipeline(cfg Config) (*Pipeline, error) {
    // Router abstracts provider selection
    sttRouter := sttrouter.New(context.Background(), cfg.GoogleSTTConfig)

    deps := Deps{
        STT: sttRouter, // speech.STTClient interface
        LLM: llmgemini.New(cfg.GeminiAPIKey, cfg.Model),
        // ... other dependencies
    }

    return &Pipeline{deps: deps}, nil
}

// Pipeline code remains unchanged regardless of STT provider
func (p *Pipeline) processAudio(ctx context.Context, audio <-chan speech.AudioChunk) {
    transcripts, err := p.deps.STT.Stream(ctx, audio)
    if err != nil {
        log.Printf("STT error: %v", err)
        return
    }

    for tr := range transcripts {
        // Process transcript regardless of source provider
        p.handleTranscript(tr)
    }
}
```

## Error Handling & Resilience

### Provider Failover

```go
type ResilientSTTClient struct {
    primary   speech.STTClient
    secondary speech.STTClient
    fallback  speech.STTClient
}

func (r *ResilientSTTClient) Stream(ctx context.Context, audio <-chan speech.AudioChunk) (<-chan speech.Transcript, error) {
    // Try primary provider
    transcripts, err := r.primary.Stream(ctx, audio)
    if err == nil {
        return transcripts, nil
    }

    log.Printf("Primary STT failed: %v, trying secondary", err)

    // Fallback to secondary provider
    transcripts, err = r.secondary.Stream(ctx, audio)
    if err == nil {
        return transcripts, nil
    }

    log.Printf("Secondary STT failed: %v, using fallback", err)

    // Last resort fallback
    return r.fallback.Stream(ctx, audio)
}
```

### Connection Recovery

```go
func (c *Client) StreamWithRetry(ctx context.Context, audio <-chan speech.AudioChunk) (<-chan speech.Transcript, error) {
    maxRetries := 3
    backoff := time.Second

    for attempt := 0; attempt < maxRetries; attempt++ {
        transcripts, err := c.Stream(ctx, audio)
        if err == nil {
            return transcripts, nil
        }

        if attempt < maxRetries-1 {
            log.Printf("STT attempt %d failed: %v, retrying in %v", attempt+1, err, backoff)

            select {
            case <-time.After(backoff):
                backoff *= 2 // Exponential backoff
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }
    }

    return nil, fmt.Errorf("STT failed after %d attempts", maxRetries)
}
```

## Performance Optimization

### Audio Chunking Strategy

```go
// Optimized audio chunking for different providers
func OptimizeChunking(provider string) ChunkingConfig {
    switch provider {
    case "deepgram":
        return ChunkingConfig{
            ChunkSize:    1024 * 4,    // 4KB chunks for WebSocket efficiency
            BufferSize:   32,          // Buffer 32 chunks
            FlushInterval: 100 * time.Millisecond,
        }

    case "google":
        return ChunkingConfig{
            ChunkSize:    1024 * 8,    // 8KB chunks for gRPC efficiency
            BufferSize:   16,          // Smaller buffer for gRPC
            FlushInterval: 200 * time.Millisecond,
        }

    default:
        return DefaultChunkingConfig()
    }
}
```

### Connection Pooling

```go
type ConnectionPool struct {
    providers map[string][]speech.STTClient
    mu        sync.RWMutex
}

func (cp *ConnectionPool) GetClient(provider string) speech.STTClient {
    cp.mu.RLock()
    clients := cp.providers[provider]
    if len(clients) > 0 {
        client := clients[0]
        cp.providers[provider] = clients[1:]
        cp.mu.RUnlock()
        return client
    }
    cp.mu.RUnlock()

    // Create new client if pool is empty
    return createSTTClient(provider)
}

func (cp *ConnectionPool) ReturnClient(provider string, client speech.STTClient) {
    cp.mu.Lock()
    cp.providers[provider] = append(cp.providers[provider], client)
    cp.mu.Unlock()
}
```

## Monitoring & Observability

### STT Metrics

```go
// Provider-specific metrics
stt_requests_total{provider="deepgram|google", status="success|failed"}
stt_latency_seconds{provider="deepgram|google", percentile="50|95|99"}
stt_confidence_score{provider="deepgram|google"}
stt_audio_duration_seconds{provider="deepgram|google"}
stt_cost_per_minute{provider="deepgram|google"}

// Router metrics
stt_provider_switches_total{from="provider", to="provider"}
stt_failover_events_total{primary="provider", fallback="provider"}
```

### Logging Examples

```
🔊 STT router initialized with provider: deepgram
🎤 Deepgram connection established (model=nova-3, interim=true)
📝 Transcript received: "hello world" (confidence=0.95, final=true, latency=156ms)
⚠️ Deepgram connection lost, attempting reconnection (attempt 1/3)
✅ STT connection recovered after 2.3s
💰 STT usage: 45.2s audio processed, cost=$0.19, provider=deepgram
🔄 Provider switched: deepgram → google (config change detected)
```

## Future Extensibility

### Adding New Providers

```go
// Step 1: Implement speech.STTClient interface
type NewProviderClient struct {
    apiKey string
    config NewProviderConfig
}

func (c *NewProviderClient) Stream(ctx context.Context, audio <-chan speech.AudioChunk) (<-chan speech.Transcript, error) {
    // Provider-specific implementation
    // Must return standardized speech.Transcript format
}

// Step 2: Add to router
func (r *Router) initProvider(provider string) speech.STTClient {
    switch provider {
    case "deepgram":
        return sttdeepgram.New(os.Getenv("DEEPGRAM_API_KEY"), "nova-3")
    case "google":
        return sttgoogle.New(ctx, googleConfig)
    case "new_provider":  // Add new provider
        return newprovider.New(os.Getenv("NEW_PROVIDER_API_KEY"))
    default:
        log.Fatalf("Unknown provider: %s", provider)
    }
}
```

### Provider Capabilities Matrix

```go
type ProviderCapabilities struct {
    RealTimeStreaming bool
    BatchProcessing   bool
    WordTimestamps    bool
    SpeakerLabels     bool
    Languages         []string
    MaxAudioLength    time.Duration
    SupportedFormats  []string
}

var ProviderMatrix = map[string]ProviderCapabilities{
    "deepgram": {
        RealTimeStreaming: true,
        BatchProcessing:   true,
        WordTimestamps:    true,
        SpeakerLabels:     true,
        Languages:         []string{"en", "es", "fr", "de"},
        MaxAudioLength:    8 * time.Hour,
        SupportedFormats:  []string{"wav", "mp3", "flac", "opus"},
    },
    "google": {
        RealTimeStreaming: true,
        BatchProcessing:   true,
        WordTimestamps:    true,
        SpeakerLabels:     true,
        Languages:         []string{"en", "es", "fr", "de", "ja", "ko", "zh"},
        MaxAudioLength:    24 * time.Hour,
        SupportedFormats:  []string{"wav", "flac", "opus"},
    },
}
```

## Testing Strategy

### Provider Testing

```go
func TestSTTProvider_Deepgram(t *testing.T) {
    client := sttdeepgram.New(testAPIKey, "nova-3")

    // Test with sample audio
    audioChannel := make(chan speech.AudioChunk, 1)
    audioChannel <- loadTestAudio("hello_world.wav")
    close(audioChannel)

    transcripts, err := client.Stream(context.Background(), audioChannel)
    assert.NoError(t, err)

    // Verify transcript quality
    var finalTranscript string
    for tr := range transcripts {
        if tr.IsFinal {
            finalTranscript = tr.Text
        }
    }

    assert.Contains(t, strings.ToLower(finalTranscript), "hello")
    assert.Contains(t, strings.ToLower(finalTranscript), "world")
}

func TestSTTRouter_ProviderSwitching(t *testing.T) {
    // Test configuration-based provider switching
    os.Setenv("SCHMA_STT_PROVIDER", "deepgram")
    router1 := sttrouter.New(context.Background(), googleConfig)

    os.Setenv("SCHMA_STT_PROVIDER", "google")
    router2 := sttrouter.New(context.Background(), googleConfig)

    // Verify different providers are used
    assert.NotEqual(t, reflect.TypeOf(router1.impl), reflect.TypeOf(router2.impl))
}
```

### Integration Testing

```go
func TestSTT_Integration(t *testing.T) {
    providers := []string{"deepgram", "google"}

    for _, provider := range providers {
        t.Run(provider, func(t *testing.T) {
            // Test same input produces consistent output format
            testAudio := loadTestAudio("test_speech.wav")

            client := createSTTClient(provider)
            transcripts := processAudio(t, client, testAudio)

            // Verify consistent transcript structure
            assert.NotEmpty(t, transcripts)
            for _, tr := range transcripts {
                assert.NotEmpty(t, tr.Text)
                assert.GreaterOrEqual(t, tr.Confidence, float32(0.0))
                assert.LessOrEqual(t, tr.Confidence, float32(1.0))

                if tr.IsFinal {
                    assert.NotEmpty(t, tr.Words)
                    for _, word := range tr.Words {
                        assert.GreaterOrEqual(t, word.Start, float32(0.0))
                        assert.GreaterOrEqual(t, word.End, word.Start)
                    }
                }
            }
        })
    }
}
```

The STT integration architecture provides a robust, swappable foundation that enables seamless provider switching based on domain requirements, cost optimization, and quality preferences while maintaining a consistent interface for the application layer.
