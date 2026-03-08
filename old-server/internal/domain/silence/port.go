package silence

import (
	"context"
	"time"

	"schma.ai/internal/domain/speech"
)

// SilenceEvent represents silence state changes
type SilenceEvent struct {
	Type      EventType     `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// EventType represents the type of silence event
type EventType string

const (
	EventSilenceStarted EventType = "silence_started"
	EventSilenceEnded   EventType = "silence_ended"
	EventKeepAliveSent  EventType = "keep_alive_sent"
)

// Config holds silence handler configuration
type Config struct {
	// SilenceThreshold is the duration after which silence is detected
	SilenceThreshold time.Duration

	// KeepAliveInterval is how often to send keep-alive pings during silence
	KeepAliveInterval time.Duration

	// EnableClientNotifications determines if client gets silence status updates
	EnableClientNotifications bool
}

// DefaultConfig returns sensible defaults for silence detection
func DefaultConfig() Config {
	return Config{
		SilenceThreshold:          3 * time.Second,
		KeepAliveInterval:         2 * time.Second,
		EnableClientNotifications: true,
	}
}

// Handler manages silence detection and keep-alive behavior
type Handler interface {
	// Start begins silence monitoring
	Start(ctx context.Context) error

	// Stop terminates silence monitoring and cleanup
	Stop(ctx context.Context) error

	// OnAudioReceived notifies handler of audio activity
	OnAudioReceived()

	// Events returns a channel of silence events for observation
	Events() <-chan SilenceEvent

	// IsInSilence returns current silence state
	IsInSilence() bool

	// SilenceDuration returns how long we've been in silence (0 if not silent)
	SilenceDuration() time.Duration
}

// AudioSink represents a destination for audio chunks (typically STT input channel)
type AudioSink interface {
	// SendAudio attempts to send audio chunk, returns false if unable
	SendAudio(chunk speech.AudioChunk) bool
}

// EventNotifier handles silence event notifications (typically WebSocket client)
type EventNotifier interface {
	// NotifyClient sends silence status to client
	NotifyClient(event SilenceEvent) error
}
