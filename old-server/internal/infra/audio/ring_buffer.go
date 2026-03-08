package audio

import (
	"sync"
	"time"

	"schma.ai/internal/domain/speech"
)

// RingBuffer implements speech.RingBuffer interface with thread-safe circular buffering
type RingBuffer struct {
	config speech.RingBufferConfig
	mu     sync.RWMutex

	// Circular buffer
	buffer []speech.BufferedChunk
	head   int // next write position
	size   int // current number of chunks

	// Statistics
	totalBytes    int
	totalDuration time.Duration
}

// NewRingBuffer creates a new ring buffer with the given configuration
func NewRingBuffer(config speech.RingBufferConfig) *RingBuffer {
	// Calculate buffer capacity based on max duration and typical chunk size
	// Assume ~20ms chunks (50 chunks per second)
	estimatedChunksPerSecond := 50
	capacity := int(config.MaxDuration.Seconds()) * estimatedChunksPerSecond

	// Minimum capacity to handle edge cases
	if capacity < 100 {
		capacity = 100
	}

	return &RingBuffer{
		config: config,
		buffer: make([]speech.BufferedChunk, capacity),
	}
}

// Write adds an audio chunk to the buffer
func (rb *RingBuffer) Write(chunk speech.AudioChunk) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Calculate chunk duration
	chunkDuration := rb.calculateChunkDuration(chunk)

	// Create buffered chunk
	bufferedChunk := speech.BufferedChunk{
		Data:      chunk,
		Timestamp: time.Now(),
		Duration:  chunkDuration,
	}

	// If buffer is full, remove oldest chunk
	if rb.size == len(rb.buffer) {
		oldChunk := rb.buffer[rb.head]
		rb.totalBytes -= len(oldChunk.Data)
		rb.totalDuration -= oldChunk.Duration
	} else {
		rb.size++
	}

	// Add new chunk
	rb.buffer[rb.head] = bufferedChunk
	rb.totalBytes += len(chunk)
	rb.totalDuration += chunkDuration

	// Advance head pointer (circular)
	rb.head = (rb.head + 1) % len(rb.buffer)

	// Trim buffer if it exceeds max duration
	rb.trimToMaxDuration()
}

// ReadLast returns the last N seconds of audio
func (rb *RingBuffer) ReadLast(duration time.Duration) []speech.AudioChunk {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return nil
	}

	// Find chunks within the requested duration
	cutoff := time.Now().Add(-duration)
	var result []speech.AudioChunk

	// Iterate from most recent to oldest
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - 1 - i + len(rb.buffer)) % len(rb.buffer)
		chunk := rb.buffer[idx]

		if chunk.Timestamp.Before(cutoff) {
			break // reached cutoff time
		}

		result = append([]speech.AudioChunk{chunk.Data}, result...)
	}

	return result
}

// ReadAll returns all buffered audio chunks in chronological order
func (rb *RingBuffer) ReadAll() []speech.AudioChunk {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return nil
	}

	result := make([]speech.AudioChunk, rb.size)

	// Read from oldest to newest
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - rb.size + i + len(rb.buffer)) % len(rb.buffer)
		result[i] = rb.buffer[idx].Data
	}

	return result
}

// Clear empties the buffer
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.size = 0
	rb.head = 0
	rb.totalBytes = 0
	rb.totalDuration = 0
}

// Size returns the current buffer size in bytes
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.totalBytes
}

// Duration returns the total duration of buffered audio
func (rb *RingBuffer) Duration() time.Duration {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.totalDuration
}

// Capacity returns the maximum buffer duration
func (rb *RingBuffer) Capacity() time.Duration {
	return rb.config.MaxDuration
}

// calculateChunkDuration estimates the duration of an audio chunk
func (rb *RingBuffer) calculateChunkDuration(chunk speech.AudioChunk) time.Duration {
	bytesPerSecond := rb.config.SampleRate * rb.config.BytesPerSample * rb.config.Channels
	seconds := float64(len(chunk)) / float64(bytesPerSecond)
	return time.Duration(seconds * float64(time.Second))
}

// trimToMaxDuration removes old chunks if total duration exceeds max
func (rb *RingBuffer) trimToMaxDuration() {
	for rb.totalDuration > rb.config.MaxDuration && rb.size > 0 {
		// Remove oldest chunk
		oldestIdx := (rb.head - rb.size + len(rb.buffer)) % len(rb.buffer)
		oldChunk := rb.buffer[oldestIdx]

		rb.totalBytes -= len(oldChunk.Data)
		rb.totalDuration -= oldChunk.Duration
		rb.size--
	}
}

// Stats returns buffer statistics for monitoring
func (rb *RingBuffer) Stats() BufferStats {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var oldestTime, newestTime time.Time
	if rb.size > 0 {
		oldestIdx := (rb.head - rb.size + len(rb.buffer)) % len(rb.buffer)
		newestIdx := (rb.head - 1 + len(rb.buffer)) % len(rb.buffer)

		oldestTime = rb.buffer[oldestIdx].Timestamp
		newestTime = rb.buffer[newestIdx].Timestamp
	}

	return BufferStats{
		ChunkCount:     rb.size,
		TotalBytes:     rb.totalBytes,
		TotalDuration:  rb.totalDuration,
		MaxDuration:    rb.config.MaxDuration,
		UtilizationPct: float64(rb.totalDuration) / float64(rb.config.MaxDuration) * 100,
		OldestChunk:    oldestTime,
		NewestChunk:    newestTime,
	}
}

// BufferStats provides monitoring information about the ring buffer
type BufferStats struct {
	ChunkCount     int           `json:"chunk_count"`
	TotalBytes     int           `json:"total_bytes"`
	TotalDuration  time.Duration `json:"total_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	UtilizationPct float64       `json:"utilization_percent"`
	OldestChunk    time.Time     `json:"oldest_chunk"`
	NewestChunk    time.Time     `json:"newest_chunk"`
}

// Compile-time interface check
var _ speech.RingBuffer = (*RingBuffer)(nil)
