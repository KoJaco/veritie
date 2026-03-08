package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	silence_app "schma.ai/internal/app/silence"
	"schma.ai/internal/domain/silence"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// AudioSinkAdapter implements silence.AudioSink for the WebSocket audio channel
type AudioSinkAdapter struct {
	audioIn     chan speech.AudioChunk
	closed      bool
	mu          sync.RWMutex
	silenceSvc  *silence_app.Service // Reference to silence service for notifications
}

// NewAudioSinkAdapter creates a new audio sink adapter
func NewAudioSinkAdapter(audioIn chan speech.AudioChunk) *AudioSinkAdapter {
	return &AudioSinkAdapter{
		audioIn: audioIn,
		closed:  false,
	}
}

// Close marks the adapter as closed to prevent further audio sends
func (a *AudioSinkAdapter) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	logger.ServiceDebugf("AUDIO_SINK", "Closing AudioSinkAdapter")
	a.closed = true
	logger.ServiceDebugf("AUDIO_SINK", "AudioSinkAdapter marked as closed")
}

// SetSilenceService sets the silence service reference for notifications
func (a *AudioSinkAdapter) SetSilenceService(svc *silence_app.Service) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.silenceSvc = svc
}

// SendAudio attempts to send audio chunk to the STT pipeline
func (a *AudioSinkAdapter) SendAudio(chunk speech.AudioChunk) bool {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		logger.ServiceDebugf("AUDIO_SINK", "AudioSink is closed, skipping audio send")
		return false
	}
	a.mu.RUnlock()

	logger.ServiceDebugf("AUDIO_SINK", "Attempting to send audio chunk of size %d bytes", len(chunk))
	select {
	case a.audioIn <- chunk:
		logger.ServiceDebugf("AUDIO_SINK", "Successfully sent audio chunk")
		return true
	default:
		logger.ServiceDebugf("AUDIO_SINK", "Audio channel is full, cannot send chunk")
		return false // channel full
	}
}

// EventNotifierAdapter implements silence.EventNotifier for WebSocket clients
type EventNotifierAdapter struct {
	conn *websocket.Conn
}

// NewEventNotifierAdapter creates a new event notifier adapter
func NewEventNotifierAdapter(conn *websocket.Conn) *EventNotifierAdapter {
	return &EventNotifierAdapter{
		conn: conn,
	}
}

// NotifyClient sends silence event to WebSocket client
func (a *EventNotifierAdapter) NotifyClient(event silence.SilenceEvent) error {
	// Convert domain event to WebSocket message
	message := SilenceMessage{
		Type:      "in_silence",
		Duration:  event.Duration.String(),
	}

	// Send to client (non-blocking)
	if err := a.conn.WriteJSON(message); err != nil {
		logger.Warnf("⚠️ [WS] Failed to send silence notification to client: %v", err)
		return err
	}

	return nil
}

// RingBufferAdapter wraps the ring buffer with additional monitoring
type RingBufferAdapter struct {
	buffer speech.RingBuffer
}

// NewRingBufferAdapter creates a new ring buffer adapter
func NewRingBufferAdapter(buffer speech.RingBuffer) *RingBufferAdapter {
	return &RingBufferAdapter{
		buffer: buffer,
	}
}

// Write adds audio to both the ring buffer and forwards to the pipeline
func (a *RingBufferAdapter) Write(chunk speech.AudioChunk) {
	a.buffer.Write(chunk)
}

// GetFallbackAudio returns buffered audio for STT recovery
func (a *RingBufferAdapter) GetFallbackAudio(seconds int) []speech.AudioChunk {
	return a.buffer.ReadLast(time.Duration(seconds) * time.Second)
}

// GetBufferStats returns current buffer statistics
func (a *RingBufferAdapter) GetBufferStats() map[string]interface{} {
	return map[string]interface{}{
		"size_bytes":      a.buffer.Size(),
		"duration":        a.buffer.Duration().String(),
		"capacity":        a.buffer.Capacity().String(),
		"utilization_pct": float64(a.buffer.Duration()) / float64(a.buffer.Capacity()) * 100,
	}
}

// Compile-time interface checks
var _ silence.AudioSink = (*AudioSinkAdapter)(nil)
var _ silence.EventNotifier = (*EventNotifierAdapter)(nil)
