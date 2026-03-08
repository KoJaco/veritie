# Ring Buffer System

## Overview

The Ring Buffer system provides circular audio buffering for STT (Speech-to-Text) fallback replay capabilities. It maintains a configurable duration of recent audio chunks in memory to enable recovery from connection interruptions and gap bridging during brief STT service outages.

## Architecture

### Domain Layer (`internal/domain/speech/ring_buffer.go`)

The domain layer defines the core interfaces and types:

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

type RingBufferConfig struct {
    MaxDuration    time.Duration  // e.g., 10 seconds
    SampleRate     int           // e.g., 16000 Hz
    BytesPerSample int           // e.g., 2 for 16-bit
    Channels       int           // e.g., 1 for mono
}

type BufferedChunk struct {
    Data      AudioChunk
    Timestamp time.Time
    Duration  time.Duration
}
```

### Implementation (`internal/infra/audio/ring_buffer.go`)

The infrastructure layer provides a thread-safe circular buffer implementation:

-   **Circular Buffer**: Fixed-capacity array with head pointer advancement
-   **Thread Safety**: RWMutex for concurrent read/write operations
-   **Duration Tracking**: Real-time calculation of buffered audio duration
-   **Automatic Trimming**: Removes old chunks when max duration exceeded
-   **Statistics**: Comprehensive buffer utilization metrics

## Configuration

### Default Configuration

```go
func DefaultRingBufferConfig() RingBufferConfig {
    return RingBufferConfig{
        MaxDuration:    10 * time.Second,  // 10 seconds retention
        SampleRate:     16000,             // 16kHz audio
        BytesPerSample: 2,                 // 16-bit samples
        Channels:       1,                 // Mono audio
    }
}
```

### Capacity Calculation

The buffer automatically calculates capacity based on:

-   **Estimated chunks per second**: ~50 (assuming 20ms chunks)
-   **Duration-based sizing**: `capacity = duration * chunks_per_second`
-   **Minimum capacity**: 100 chunks for edge cases

## Usage Patterns

### 1. Continuous Buffering

```go
ringBuffer := audio.NewRingBuffer(speech.DefaultRingBufferConfig())

// Write audio chunks as they arrive
for chunk := range audioStream {
    ringBuffer.Write(chunk)
}
```

### 2. Fallback Replay

```go
// On STT connection failure, replay last 5 seconds
fallbackAudio := ringBuffer.ReadLast(5 * time.Second)
for _, chunk := range fallbackAudio {
    sttClient.SendChunk(chunk)
}
```

### 3. Buffer Monitoring

```go
stats := ringBuffer.Stats()
log.Printf("Buffer utilization: %.1f%% (%d chunks, %s duration)",
    stats.UtilizationPct, stats.ChunkCount, stats.TotalDuration)
```

## WebSocket Integration

### Client Statistics Request

Clients can request buffer statistics:

```json
{
    "type": "buffer_stats"
}
```

### Server Response

```json
{
    "type": "buffer_stats",
    "ring_buffer": {
        "chunk_count": 245,
        "total_bytes": 78400,
        "total_duration": "9.8s",
        "max_duration": "10s",
        "utilization_percent": 98.0,
        "oldest_chunk": "2025-01-15T10:30:00Z",
        "newest_chunk": "2025-01-15T10:30:09Z"
    }
}
```

## Performance Characteristics

### Memory Usage

-   **Fixed Memory Footprint**: Circular buffer prevents unbounded growth
-   **Chunk Size**: Typically ~320 bytes per 20ms chunk at 16kHz
-   **10-second buffer**: ~16KB total memory usage
-   **Efficient Allocation**: Pre-allocated array, no runtime allocation

### Thread Safety

-   **Read Operations**: Multiple concurrent readers supported
-   **Write Operations**: Single writer with reader coordination
-   **Lock Granularity**: RWMutex minimizes contention
-   **Non-blocking Reads**: Statistics and buffer access don't block writes

### Time Complexity

-   **Write**: O(1) - constant time insertion
-   **ReadLast**: O(n) where n is chunks in time window
-   **ReadAll**: O(n) where n is current buffer size
-   **Statistics**: O(1) - pre-calculated values

## Error Handling & Edge Cases

### Buffer Overflow

When buffer reaches capacity:

1. Oldest chunk is automatically removed
2. New chunk replaces it at head position
3. Statistics are updated atomically
4. No data loss occurs (FIFO behavior)

### Duration Calculation

Accurate duration tracking via:

```go
func calculateChunkDuration(chunk AudioChunk) time.Duration {
    bytesPerSecond := sampleRate * bytesPerSample * channels
    seconds := float64(len(chunk)) / float64(bytesPerSecond)
    return time.Duration(seconds * float64(time.Second))
}
```

### Concurrent Access

-   **Write conflicts**: Serialized by mutex
-   **Read during write**: Consistent snapshots via RWMutex
-   **Statistics accuracy**: Atomic updates ensure consistency

## Monitoring & Observability

### Buffer Statistics

The `BufferStats` struct provides comprehensive monitoring:

```go
type BufferStats struct {
    ChunkCount     int           // Number of buffered chunks
    TotalBytes     int           // Total buffer size in bytes
    TotalDuration  time.Duration // Total audio duration
    MaxDuration    time.Duration // Buffer capacity
    UtilizationPct float64       // Percentage of capacity used
    OldestChunk    time.Time     // Timestamp of oldest chunk
    NewestChunk    time.Time     // Timestamp of newest chunk
}
```

### Key Metrics to Monitor

-   **Utilization**: Buffer fullness percentage
-   **Turnover Rate**: How quickly chunks are replaced
-   **Duration Accuracy**: Calculated vs. actual durations
-   **Memory Efficiency**: Bytes per second ratio

## Use Cases

### 1. STT Connection Recovery

**Problem**: STT service disconnects briefly, losing recent audio
**Solution**: Ring buffer maintains last 10 seconds for replay on reconnection

### 2. Gap Bridging

**Problem**: Network hiccups cause temporary STT interruptions
**Solution**: Buffer provides continuous audio stream during recovery

### 3. Quality Assurance

**Problem**: Need to analyze recent audio for transcription accuracy
**Solution**: Ring buffer enables examination of problematic segments

### 4. Debug & Troubleshooting

**Problem**: Investigating audio quality or processing issues
**Solution**: Buffer provides recent audio samples for analysis

## Integration Points

### WebSocket Handler

```go
// Create ring buffer
ringBuffer := audio.NewRingBuffer(speech.DefaultRingBufferConfig())

// Buffer incoming audio
chunk := speech.AudioChunk(data)
ringBuffer.Write(chunk)

// Send to STT pipeline
audioIn <- chunk
```

### Silence Service

The ring buffer integrates with the silence detection service to:

-   Maintain audio continuity during silence periods
-   Provide fallback audio for keep-alive scenarios
-   Enable analysis of silence detection accuracy

## Future Enhancements

### Planned Features

1. **Compression**: Reduce memory usage with audio compression
2. **Persistence**: Optional disk-based buffering for longer retention
3. **Multi-Rate Support**: Handle variable sample rate audio
4. **Adaptive Sizing**: Dynamic buffer size based on usage patterns

### Configuration Extensions

```go
type AdvancedRingBufferConfig struct {
    RingBufferConfig
    CompressionEnabled bool
    PersistencePath    string
    AdaptiveSizing     bool
    MaxMemoryMB        int
}
```

## Testing Strategy

### Unit Tests

-   Buffer capacity limits
-   Circular wrap-around behavior
-   Duration calculations
-   Thread safety under load
-   Statistics accuracy

### Integration Tests

-   WebSocket handler integration
-   Silence service coordination
-   STT pipeline compatibility
-   Performance under realistic load

### Performance Tests

-   Memory usage patterns
-   Concurrent access performance
-   Buffer utilization efficiency
-   Garbage collection impact
