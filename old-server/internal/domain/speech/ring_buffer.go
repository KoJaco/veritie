package speech

import (
	"time"
)

// RingBuffer provides circular audio buffering for fallback replay
type RingBuffer interface {
	// Write adds an audio chunk to the buffer
	Write(chunk AudioChunk)

	// ReadLast returns the last N seconds of audio
	ReadLast(duration time.Duration) []AudioChunk

	// ReadAll returns all buffered audio chunks in chronological order
	ReadAll() []AudioChunk

	// Clear empties the buffer
	Clear()

	// Size returns the current buffer size in bytes
	Size() int

	// Duration returns the total duration of buffered audio
	Duration() time.Duration

	// Capacity returns the maximum buffer duration
	Capacity() time.Duration
}

// RingBufferConfig holds configuration for audio buffering
type RingBufferConfig struct {
	// MaxDuration is the maximum duration to buffer (e.g., 10 seconds)
	MaxDuration time.Duration

	// SampleRate is the audio sample rate (used for duration calculations)
	SampleRate int

	// BytesPerSample is typically 2 for 16-bit audio
	BytesPerSample int

	// Channels is typically 1 for mono audio
	Channels int
}

// DefaultRingBufferConfig returns sensible defaults for 16-bit mono audio
func DefaultRingBufferConfig() RingBufferConfig {
	return RingBufferConfig{
		MaxDuration:    10 * time.Second,
		SampleRate:     16000, // 16kHz
		BytesPerSample: 2,     // 16-bit
		Channels:       1,     // mono
	}
}

// BufferedChunk represents an audio chunk with timestamp
type BufferedChunk struct {
	Data      AudioChunk
	Timestamp time.Time
	Duration  time.Duration // calculated duration of this chunk
}
