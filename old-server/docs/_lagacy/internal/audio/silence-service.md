# Silence Detection Service

## Overview

The Silence Detection Service provides intelligent audio activity monitoring and STT (Speech-to-Text) connection keep-alive functionality. It has been architecturally refactored from an embedded WebSocket handler component into a dedicated, testable service with clean separation of concerns.

## Architecture Refactoring

### Before: Embedded Handler (Problems)

```
❌ WebSocket Handler
   ├── Protocol Management
   ├── Audio Pipeline
   ├── Silence Detection (embedded)  ← Tight coupling
   ├── Keep-alive Logic (embedded)   ← Hard to test
   └── Client Notifications         ← Mixed responsibilities
```

**Issues:**

-   **Tight Coupling**: Silence logic embedded in transport layer
-   **Single Responsibility Violation**: WebSocket handler doing too much
-   **Hard to Test**: No interfaces, embedded state
-   **Not Reusable**: Locked to WebSocket implementation

### After: Service Architecture (Solutions)

```
✅ Clean Separation
   ├── Domain Layer (interfaces)
   ├── App Layer (service)
   ├── Infrastructure (implementations)
   └── Transport (adapters)
```

**Benefits:**

-   **Clean Domain Interfaces**: Testable contracts
-   **Dependency Injection**: Pluggable implementations
-   **Event-Driven Design**: Observable silence events
-   **Adapter Pattern**: Clean transport integration

## Domain Layer

### Core Interfaces (`internal/domain/silence/port.go`)

```go
// Main service interface
type Handler interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    OnAudioReceived()
    Events() <-chan SilenceEvent
    IsInSilence() bool
    SilenceDuration() time.Duration
}

// Audio output dependency
type AudioSink interface {
    SendAudio(chunk speech.AudioChunk) bool
}

// Client notification dependency
type EventNotifier interface {
    NotifyClient(event SilenceEvent) error
}
```

### Event Types

```go
type SilenceEvent struct {
    Type      EventType     `json:"type"`
    Timestamp time.Time     `json:"timestamp"`
    Duration  time.Duration `json:"duration,omitempty"`
}

const (
    EventSilenceStarted EventType = "silence_started"
    EventSilenceEnded   EventType = "silence_ended"
    EventKeepAliveSent  EventType = "keep_alive_sent"
)
```

### Configuration

```go
type Config struct {
    SilenceThreshold          time.Duration  // 3 seconds
    KeepAliveInterval         time.Duration  // 2 seconds
    EnableClientNotifications bool           // true
}

func DefaultConfig() Config {
    return Config{
        SilenceThreshold:          3 * time.Second,
        KeepAliveInterval:         2 * time.Second,
        EnableClientNotifications: true,
    }
}
```

## Service Implementation

### Service Structure (`internal/app/silence/service.go`)

```go
type Service struct {
    config       silence.Config
    audioSink    silence.AudioSink      // STT integration
    notifier     silence.EventNotifier  // Client notifications

    // Thread-safe state
    mu             sync.RWMutex
    inSilence      bool
    silenceStarted time.Time
    lastAudioTime  time.Time

    // Event-driven control
    ctx            context.Context
    cancel         context.CancelFunc
    events         chan silence.SilenceEvent
    audioActivity  chan struct{}

    // Timers
    silenceTimer   *time.Timer
    keepAliveTimer *time.Timer
}
```

### Key Features

#### 1. **Event-Driven Architecture**

```go
func (s *Service) monitorLoop() {
    for {
        select {
        case <-s.ctx.Done():
            return
        case <-s.audioActivity:
            s.handleAudioActivity()
        }
    }
}
```

#### 2. **Thread-Safe State Management**

```go
func (s *Service) OnAudioReceived() {
    s.mu.Lock()
    s.lastAudioTime = time.Now()
    s.mu.Unlock()

    // Non-blocking notification
    select {
    case s.audioActivity <- struct{}{}:
    default:
    }
}
```

#### 3. **Configurable Timers**

```go
// Start silence detection
s.silenceTimer = time.AfterFunc(s.config.SilenceThreshold, s.onSilenceDetected)

// Start keep-alive during silence
s.keepAliveTimer = time.AfterFunc(s.config.KeepAliveInterval, s.sendKeepAlive)
```

## Integration Adapters

### WebSocket Adapters (`internal/transport/ws/adapters.go`)

#### Audio Sink Adapter

```go
type AudioSinkAdapter struct {
    audioIn chan speech.AudioChunk
}

func (a *AudioSinkAdapter) SendAudio(chunk speech.AudioChunk) bool {
    select {
    case a.audioIn <- chunk:
        return true
    default:
        return false // channel full
    }
}
```

#### Event Notifier Adapter

```go
type EventNotifierAdapter struct {
    conn *websocket.Conn
}

func (a *EventNotifierAdapter) NotifyClient(event silence.SilenceEvent) error {
    message := SilenceMessage{
        Type:      "silence_status",
        InSilence: event.Type == silence.EventSilenceStarted,
        Duration:  event.Duration.String(),
    }
    return a.conn.WriteJSON(message)
}
```

## WebSocket Integration

### Service Initialization

```go
// Create dependencies
audioSink := NewAudioSinkAdapter(audioIn)
eventNotifier := NewEventNotifierAdapter(conn)
silenceConfig := silence.DefaultConfig()

// Initialize service
silenceService := silence_app.NewService(silenceConfig, audioSink, eventNotifier)
err = silenceService.Start(r.Context())
defer silenceService.Stop(r.Context())
```

### Audio Activity Notification

```go
case websocket.BinaryMessage:
    chunk := speech.AudioChunk(data)

    // Write to ring buffer
    ringBuffer.Write(chunk)

    // Send to STT pipeline
    audioIn <- chunk

    // Notify silence service
    silenceService.OnAudioReceived()
```

### Client Monitoring

```go
case "buffer_stats":
    stats := map[string]interface{}{
        "type": "buffer_stats",
        "silence": map[string]interface{}{
            "in_silence": silenceService.IsInSilence(),
            "duration":   silenceService.SilenceDuration().String(),
        },
    }
    _ = conn.WriteJSON(stats)
```

## Client Protocol

### Silence Status Messages

When silence detection triggers:

```json
{
    "type": "silence_status",
    "in_silence": true
}
```

When audio resumes:

```json
{
    "type": "silence_status",
    "in_silence": false,
    "duration": "4.2s"
}
```

### Statistics Integration

```json
{
  "type": "buffer_stats",
  "silence": {
    "in_silence": false,
    "duration": "0s"
  },
  "ring_buffer": { ... }
}
```

## Behavioral Characteristics

### Silence Detection Flow

1. **Audio Activity**: Reset silence timer to threshold (3s)
2. **Threshold Reached**: Enter silence mode, start keep-alive timer
3. **Keep-Alive Interval**: Send 4-byte silence chunk every 2s
4. **Audio Resume**: Exit silence mode, stop keep-alive timer

### Keep-Alive Mechanism

```go
func (s *Service) sendKeepAlive() {
    silenceChunk := make([]byte, 4) // 4 bytes of zeros

    if s.audioSink.SendAudio(speech.AudioChunk(silenceChunk)) {
        log.Printf("📡 sent STT keep-alive ping")
        s.emitEvent(silence.SilenceEvent{
            Type:      silence.EventKeepAliveSent,
            Timestamp: time.Now(),
        })
    }
}
```

### Event Emission

```go
func (s *Service) emitEvent(event silence.SilenceEvent) {
    select {
    case s.events <- event:
    default:
        log.Printf("⚠️ silence event channel full, dropping event")
    }
}
```

## Error Handling & Edge Cases

### Channel Backpressure

-   **Audio Activity**: Non-blocking notification with fallback
-   **Event Emission**: Non-blocking with dropped event logging
-   **Keep-Alive**: Channel full detection and error logging

### Concurrent Access

-   **State Reads**: RWMutex for efficient concurrent access
-   **Timer Management**: Mutex protection for timer lifecycle
-   **Context Cancellation**: Graceful shutdown on context done

### Resource Cleanup

```go
func (s *Service) Stop(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Cancel context
    s.cancel()

    // Stop timers
    if s.silenceTimer != nil {
        s.silenceTimer.Stop()
    }
    if s.keepAliveTimer != nil {
        s.keepAliveTimer.Stop()
    }

    // Close events channel
    close(s.events)

    return nil
}
```

## Performance Characteristics

### Memory Usage

-   **Fixed Allocation**: Pre-allocated channels and timers
-   **Event Buffer**: 10-event channel capacity
-   **State Overhead**: Minimal struct fields
-   **No Goroutine Leaks**: Proper context cancellation

### CPU Impact

-   **Event-Driven**: No polling loops or busy waiting
-   **Timer Efficiency**: Go runtime timer optimization
-   **Lock Contention**: RWMutex minimizes write blocking
-   **Channel Operations**: Efficient Go channel implementation

## Testing Strategy

### Unit Tests

```go
func TestSilenceDetection(t *testing.T) {
    mockSink := &MockAudioSink{}
    mockNotifier := &MockEventNotifier{}

    service := silence_app.NewService(
        silence.DefaultConfig(),
        mockSink,
        mockNotifier,
    )

    // Test silence detection
    service.Start(context.Background())
    time.Sleep(4 * time.Second) // Exceed threshold
    assert.True(t, service.IsInSilence())
}
```

### Integration Tests

```go
func TestWebSocketIntegration(t *testing.T) {
    // Test full WebSocket integration
    // Verify silence notifications
    // Check keep-alive behavior
}
```

### Performance Tests

```go
func BenchmarkSilenceService(b *testing.B) {
    // Measure audio activity processing
    // Test concurrent access performance
    // Benchmark memory allocation
}
```

## Monitoring & Observability

### Service Metrics

-   **Silence Events**: Count of silence starts/ends
-   **Keep-Alive Pings**: Frequency and success rate
-   **State Transitions**: Timing and patterns
-   **Event Channel**: Buffer utilization

### Log Output Examples

```
🔇 silence service started (threshold=3s, keep-alive=2s)
🔇 silence detected - starting keep-alive pings
📡 sent STT keep-alive ping
🔊 exiting silence mode - audio resumed after 4.2s
🔇 silence service stopped
```

### Error Scenarios

```
⚠️ failed to send keep-alive - audio sink unavailable
⚠️ silence event channel full, dropping event: keep_alive_sent
⚠️ failed to send silence notification to client: connection closed
```

## Future Enhancements

### Planned Features

1. **Adaptive Thresholds**: ML-based silence detection tuning
2. **Audio Quality**: Keep-alive chunk audio characteristics
3. **Batch Events**: Efficient event aggregation
4. **Metrics Export**: Prometheus integration

### Configuration Extensions

```go
type AdvancedConfig struct {
    Config
    AdaptiveThresholds    bool
    KeepAliveAudioQuality AudioQuality
    EventBatchSize       int
    MetricsEnabled       bool
}
```

## Comparison: Before vs After

| Aspect              | Before (Embedded) | After (Service)    |
| ------------------- | ----------------- | ------------------ |
| **Coupling**        | Tight (WebSocket) | Loose (interfaces) |
| **Testability**     | Hard              | Easy (mocks)       |
| **Reusability**     | None              | High               |
| **Observability**   | Limited           | Event-driven       |
| **Configuration**   | Hardcoded         | Flexible           |
| **Error Handling**  | Basic             | Comprehensive      |
| **Performance**     | Good              | Optimized          |
| **Maintainability** | Poor              | Excellent          |

The refactored Silence Service demonstrates clean architecture principles while maintaining all original functionality and adding significant improvements in testability, observability, and maintainability.
