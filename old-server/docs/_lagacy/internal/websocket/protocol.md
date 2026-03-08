# WebSocket Protocol

## Overview

The WebSocket protocol provides real-time, bidirectional communication between clients and the Schma.ai server for audio streaming and function extraction. The protocol supports authentication, configuration, audio streaming, and structured data delivery.

## Connection Lifecycle

### 1. Connection Establishment

```
Client                                    Server
  │                                         │
  │ ──── HTTP Upgrade Request ────────────▶ │
  │      (Authorization: Bearer <token>)    │
  │                                         │
  │ ◄─── HTTP 101 Switching Protocols ──── │
  │                                         │
  │ ◄──────── WebSocket Connected ────────► │
```

**HTTP Upgrade Headers:**

```http
GET /ws HTTP/1.1
Host: api.schma.ai
Upgrade: websocket
Connection: Upgrade
Authorization: Bearer <API_KEY>
Sec-WebSocket-Key: <key>
Sec-WebSocket-Version: 13
```

### 2. Session Configuration

```
Client                                    Server
  │                                         │
  │ ──── ConfigMessage ─────────────────▶ │
  │      (session params, functions)      │
  │                                         │
  │ ◄─── AckMessage ────────────────────── │
  │      (session_id, status)             │
  │                                         │
  │ ◄──────── Ready for Audio ──────────► │
```

### 3. Audio Streaming & Processing

```
Client                                    Server
  │                                         │
  │ ──── Binary Audio Chunks ───────────▶ │ ──┐
  │      (continuous stream)              │   │ STT Processing
  │                                         │   │
  │ ◄─── TranscriptMessage ─────────────── │ ◄─┘
  │      (interim transcripts)            │
  │                                         │
  │ ◄─── DraftFunctionMessage ──────────── │ ──┐
  │      (real-time function detection)   │   │ Draft Detection
  │                                         │ ◄─┘
  │ ◄─── FunctionMessage ─────────────────── │ ──┐
  │      (final structured functions)     │   │ LLM Processing
  │                                         │ ◄─┘
```

## Message Types

### Client → Server Messages

#### 1. Configuration Message

```json
{
    "type": "config",
    "session_id": "optional-session-id",
    "language": "en-US",
    "stt": {
        "provider": "deepgram",
        "sample_hertz": 16000,
        "encoding": "opus"
    },
    "function_config": {
        "definitions": [
            {
                "name": "create_meeting",
                "description": "Create a calendar meeting",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "title": { "type": "string" },
                        "participants": {
                            "type": "array",
                            "items": { "type": "string" }
                        },
                        "start_time": {
                            "type": "string",
                            "format": "date-time"
                        }
                    },
                    "required": ["title", "start_time"]
                }
            }
        ],
        "parsing_guide": "Extract meeting details from natural speech",
        "update_ms": 2000
    },
    "input_context": {
        "current_raw_transcript": "Previous conversation context...",
        "current_functions": []
    }
}
```

**Configuration Fields:**

| Field             | Type           | Required | Description                          |
| ----------------- | -------------- | -------- | ------------------------------------ |
| `type`            | string         | ✅       | Must be "config"                     |
| `session_id`      | string         | ❌       | Optional session ID for resuming     |
| `language`        | string         | ❌       | STT language code (default: "en-US") |
| `stt`             | STTConfig      | ✅       | Speech-to-text configuration         |
| `function_config` | FunctionConfig | ❌       | Function extraction settings         |
| `input_context`   | InputContext   | ❌       | Previous session context             |

**STT Configuration:**

```go
type STTConfig struct {
    Provider    string `json:"provider"`     // "deepgram" or "google"
    SampleHertz int    `json:"sample_hertz"` // Audio sample rate
    Encoding    string `json:"encoding"`     // "opus", "pcm", "wav"
}
```

**Function Configuration:**

```go
type FunctionConfig struct {
    Definitions  []speech.FunctionDefinition `json:"definitions"`
    ParsingGuide string                      `json:"parsing_guide"`
    UpdateMS     int                         `json:"update_ms"` // LLM throttling
}
```

#### 2. Binary Audio Data

-   **Format**: Binary WebSocket frames
-   **Content**: Raw audio data in configured encoding
-   **Streaming**: Continuous chunks sent as available
-   **Buffering**: Server uses ring buffer for replay capability

```go
// Audio chunk structure (internal)
type AudioChunk struct {
    Data      []byte
    Timestamp time.Time
    Duration  time.Duration
}
```

### Server → Client Messages

#### 1. Acknowledgment Message

```json
{
    "type": "ack",
    "session_id": "abc123-def456-789"
}
```

**Sent when:**

-   Configuration message successfully processed
-   Session initialized and ready for audio
-   Function definitions validated and loaded

#### 2. Transcript Message

```json
{
    "type": "transcript",
    "text": "create a meeting with john doe tomorrow at 3pm",
    "final": true,
    "confidence": 0.95,
    "words": [
        {
            "word": "create",
            "start": 0.0,
            "end": 0.5,
            "confidence": 0.98
        },
        {
            "word": "meeting",
            "start": 0.6,
            "end": 1.1,
            "confidence": 0.96
        }
    ]
}
```

**Transcript Fields:**

| Field        | Type    | Description                            |
| ------------ | ------- | -------------------------------------- |
| `type`       | string  | Always "transcript"                    |
| `text`       | string  | Transcribed text content               |
| `final`      | boolean | True if transcript is finalized        |
| `confidence` | float   | STT confidence score (0.0-1.0)         |
| `words`      | Word[]  | Individual word timings and confidence |

**Word Timing:**

```go
type Word struct {
    Word       string  `json:"word"`
    Start      float64 `json:"start"`      // Start time in seconds
    End        float64 `json:"end"`        // End time in seconds
    Confidence float64 `json:"confidence"` // Word-level confidence
}
```

#### 3. Draft Function Message

```json
{
    "type": "function_draft_extracted",
    "draft_function": {
        "id": "draft_001",
        "name": "create_meeting",
        "arguments": {
            "title": "meeting with john doe",
            "start_time": "tomorrow at 3pm"
        },
        "similarity_score": 0.87,
        "raw_text": "create a meeting with john doe tomorrow at 3pm"
    }
}
```

**Draft Function Fields:**

| Field            | Type         | Description                             |
| ---------------- | ------------ | --------------------------------------- |
| `type`           | string       | Always "function_draft_extracted"       |
| `draft_function` | FunctionCall | Detected function with similarity score |

**Function Call Structure:**

```go
type FunctionCall struct {
    ID             string                 `json:"id"`
    Name           string                 `json:"name"`
    Arguments      map[string]interface{} `json:"arguments"`
    SimilarityScore float64               `json:"similarity_score,omitempty"`
    RawText        string                 `json:"raw_text,omitempty"`
}
```

#### 4. Function Message

```json
{
    "type": "functions",
    "functions": [
        {
            "id": "func_001",
            "name": "create_meeting",
            "arguments": {
                "title": "Meeting with John Doe",
                "participants": ["john.doe@company.com"],
                "start_time": "2025-01-16T15:00:00Z",
                "duration_minutes": 60
            }
        },
        {
            "id": "func_002",
            "name": "send_email",
            "arguments": {
                "to": ["john.doe@company.com"],
                "subject": "Meeting Confirmation",
                "body": "Hi John, confirming our meeting tomorrow at 3pm."
            }
        }
    ]
}
```

**Functions Array:**

-   Contains all currently active function calls
-   Updated incrementally as LLM refines understanding
-   Function IDs preserved across updates for client state management

#### 5. Silence Status Message

```json
{
    "type": "silence_status",
    "in_silence": true,
    "duration": "3.2s"
}
```

**Silence Detection:**

-   Sent when silence state changes
-   `in_silence: true` when audio quiet for >3 seconds
-   `duration` indicates how long silence has lasted
-   Used for STT keep-alive and UI feedback

#### 6. Error Message

```json
{
    "type": "error",
    "error": "STT provider unavailable: connection timeout",
    "code": "STT_TIMEOUT",
    "retry_after": 30
}
```

**Error Types:**

| Code             | Description                        | Action                           |
| ---------------- | ---------------------------------- | -------------------------------- |
| `AUTH_FAILED`    | Invalid API key or expired token   | Reconnect with valid credentials |
| `RATE_LIMITED`   | Too many requests from client      | Wait for `retry_after` seconds   |
| `STT_TIMEOUT`    | Speech-to-text service unavailable | Retry connection                 |
| `LLM_ERROR`      | Language model processing failed   | Continue with transcripts only   |
| `CONFIG_INVALID` | Invalid function definitions       | Fix configuration and reconnect  |

## Authentication Flow

### API Key Authentication

```http
Authorization: Bearer mk_live_abc123def456...
```

**Process:**

1. Extract Bearer token from WebSocket upgrade headers
2. Validate API key against database
3. Load app configuration and rate limits
4. Inject authenticated Principal into session context

### Session Context

```go
type Principal struct {
    AppID     string
    AccountID string
    APIKey    string
    Settings  AppSettings
    RateLimit RateLimitConfig
}
```

## Real-Time Features

### 1. Interim Transcripts

```json
// Interim (not final)
{"type": "transcript", "text": "create a meet", "final": false, "confidence": 0.85}
{"type": "transcript", "text": "create a meeting", "final": false, "confidence": 0.90}
{"type": "transcript", "text": "create a meeting with", "final": false, "confidence": 0.88}

// Final transcript
{"type": "transcript", "text": "create a meeting with john", "final": true, "confidence": 0.95}
```

**Benefits:**

-   Real-time user feedback
-   Progressive understanding
-   Early draft function detection

### 2. Draft Function Evolution

```json
// First detection
{"type": "function_draft_extracted", "draft_function": {"name": "create_meeting", "similarity_score": 0.75}}

// Improved detection
{"type": "function_draft_extracted", "draft_function": {"name": "create_meeting", "similarity_score": 0.87}}

// Final LLM processing
{"type": "functions", "functions": [{"name": "create_meeting", "arguments": {...}}]}
```

### 3. Function Call Updates

```json
// Initial LLM output
{"type": "functions", "functions": [{"id": "f1", "name": "create_meeting", "arguments": {"title": "meeting"}}]}

// Updated with more context
{"type": "functions", "functions": [{"id": "f1", "name": "create_meeting", "arguments": {"title": "meeting with john", "start_time": "tomorrow"}}]}

// Final structured output
{"type": "functions", "functions": [{"id": "f1", "name": "create_meeting", "arguments": {"title": "Meeting with John Doe", "start_time": "2025-01-16T15:00:00Z"}}]}
```

## Error Handling Patterns

### Connection Resilience

```javascript
// Client-side reconnection logic
websocket.onclose = (event) => {
    if (event.code !== 1000) {
        // Not normal closure
        setTimeout(() => reconnect(), backoffDelay);
    }
};

websocket.onerror = (error) => {
    console.error("WebSocket error:", error);
    // Attempt graceful degradation
};
```

### Graceful Degradation

```json
// Server continues with reduced functionality
{
    "type": "error",
    "error": "LLM service temporarily unavailable",
    "code": "LLM_UNAVAILABLE",
    "fallback_mode": "transcripts_only"
}
```

### Audio Buffer Management

```go
// Server-side ring buffer prevents audio loss
type RingBuffer struct {
    chunks   []AudioChunk
    capacity int
    head     int
    tail     int
    mutex    sync.RWMutex
}

// Replay capability for STT failures
func (rb *RingBuffer) ReplayFrom(timestamp time.Time) []AudioChunk {
    // Return buffered chunks from specified time
}
```

## Performance Optimizations

### 1. Message Batching

```go
// Batch multiple transcripts if arriving rapidly
type BatchedTranscript struct {
    Type        string       `json:"type"`
    Transcripts []Transcript `json:"transcripts"`
}
```

### 2. Compression

```http
Sec-WebSocket-Extensions: permessage-deflate
```

### 3. Throttling

```json
{
    "function_config": {
        "update_ms": 2000 // Limit LLM calls to every 2 seconds
    }
}
```

## Monitoring & Observability

### Connection Metrics

-   Active WebSocket connections
-   Connection duration
-   Disconnect reasons and frequency
-   Message throughput (messages/second)

### Processing Metrics

-   STT latency (audio → transcript)
-   LLM latency (transcript → functions)
-   Draft detection accuracy
-   Function extraction success rate

### Example Logs

```
🔌 WebSocket connected: session=abc123, app=demo-app, ip=192.168.1.1
📝 Config received: functions=3, stt=deepgram, language=en-US
🎤 Audio streaming started: encoding=opus, sample_rate=16000Hz
📄 Transcript: "create meeting" (interim=true, confidence=0.85, latency=120ms)
🎯 Draft detected: create_meeting (similarity=0.87, latency=15ms)
🤖 LLM response: 1 function (latency=340ms, tokens=25/78)
💰 Usage: 4.2s audio, 25 prompt tokens, 78 completion tokens
🔌 WebSocket closed: session=abc123, duration=2m30s, reason=client_disconnect
```

## Client Implementation Examples

### JavaScript Client

```javascript
class Schma.aiClient {
    constructor(apiKey) {
        this.apiKey = apiKey;
        this.ws = null;
    }

    connect() {
        this.ws = new WebSocket("wss://api.schma.ai/ws", [], {
            headers: { Authorization: `Bearer ${this.apiKey}` },
        });

        this.ws.onmessage = this.handleMessage.bind(this);
    }

    configure(config) {
        this.ws.send(
            JSON.stringify({
                type: "config",
                ...config,
            })
        );
    }

    sendAudio(audioData) {
        if (this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(audioData);
        }
    }

    handleMessage(event) {
        const message = JSON.parse(event.data);

        switch (message.type) {
            case "transcript":
                this.onTranscript(message);
                break;
            case "functions":
                this.onFunctions(message.functions);
                break;
            case "function_draft_extracted":
                this.onDraftFunction(message.draft_function);
                break;
            case "error":
                this.onError(message);
                break;
        }
    }
}
```

### Go Client

```go
type Client struct {
    conn   *websocket.Conn
    apiKey string
}

func (c *Client) Connect(ctx context.Context) error {
    headers := http.Header{}
    headers.Set("Authorization", "Bearer "+c.apiKey)

    conn, _, err := websocket.DefaultDialer.DialContext(
        ctx, "wss://api.schma.ai/ws", headers)
    if err != nil {
        return err
    }

    c.conn = conn
    go c.readMessages()
    return nil
}

func (c *Client) SendConfig(config ConfigMessage) error {
    return c.conn.WriteJSON(config)
}

func (c *Client) SendAudio(audioData []byte) error {
    return c.conn.WriteMessage(websocket.BinaryMessage, audioData)
}
```

The WebSocket protocol provides a robust, real-time communication layer that enables seamless audio streaming and structured data extraction with comprehensive error handling and performance optimizations.
