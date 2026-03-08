package silence

import (
	"context"
	"sync"
	"time"

	"schma.ai/internal/domain/silence"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
)

// TODO: extrapolate out to domain, make sure to follow hexagonal architecture.
// TODO: inject through deps in main, pass through to pipeline.
// Service implements silence.Handler interface
type Service struct {
	config    silence.Config
	audioSink silence.AudioSink
	notifier  silence.EventNotifier

	// State
	mu             sync.RWMutex
	inSilence      bool
	silenceStarted time.Time
	lastAudioTime  time.Time
	stopped        bool
	sttActive      bool // Track if STT connection is active

	// Control
	ctx    context.Context
	cancel context.CancelFunc

	// Channels
	events        chan silence.SilenceEvent
	audioActivity chan struct{}

	// Timers
	silenceTimer   *time.Timer
	keepAliveTimer *time.Timer
}

// NewService creates a new silence detection service
func NewService(
	config silence.Config,
	audioSink silence.AudioSink,
	notifier silence.EventNotifier,
) *Service {
	return &Service{
		config:        config,
		audioSink:     audioSink,
		notifier:      notifier,
		events:        make(chan silence.SilenceEvent, 10),
		audioActivity: make(chan struct{}, 1),
		lastAudioTime: time.Now(),
		sttActive:     true, // Assume STT is active when service starts
	}
}

// Start begins silence monitoring
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return nil // already started
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	
	logger.ServiceDebugf("SILENCE", "silence service started (client-side detection mode)")

	return nil
}

// Stop terminates silence monitoring
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	
	if s.cancel == nil {
		logger.ServiceDebugf("SILENCE", "already stopped, returning")
		return nil // already stopped
	}

	
	s.cancel()
	s.cancel = nil

	// Stop timers
	if s.silenceTimer != nil {
		logger.ServiceDebugf("SILENCE", "stopping silence timer")
		s.silenceTimer.Stop()
		s.silenceTimer = nil
	}

	if s.keepAliveTimer != nil {
		logger.ServiceDebugf("SILENCE", "stopping keep-alive timer")
		s.keepAliveTimer.Stop()
		s.keepAliveTimer = nil
	}

	// Mark as stopped before closing channel
	s.stopped = true
	logger.ServiceDebugf("SILENCE", "marked as stopped")

	// Close events channel
	close(s.events)

	return nil
}

// OnAudioReceived notifies the service of audio activity
// DISABLED: We now use client-side silence detection instead of server-side
func (s *Service) OnAudioReceived() {
	// Server-side silence detection is disabled in favor of client-side detection
	// This method is kept for interface compatibility but does nothing
}

// OnClientSilenceStatus handles silence status updates from the client
func (s *Service) OnClientSilenceStatus(inSilence bool, duration string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	logger.ServiceDebugf("SILENCE", "client silence status: inSilence=%v, duration=%s", inSilence, duration)
	
	if inSilence {
		// Client is in silence - send Deepgram KeepAlive message
		if s.sttActive {
			s.sendDeepgramKeepAlive()
		} else {
			logger.Warnf("⚠️ [SILENCE] client in silence but STT connection is inactive")
		}
	} else {
		// Client detected audio again - log the transition
		logger.ServiceDebugf("SILENCE", "client detected audio - normal flow resumed")
	}
}

// MarkSTTInactive marks the STT connection as inactive
func (s *Service) MarkSTTInactive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	logger.ServiceDebugf("SILENCE", "marking STT connection as inactive")
	s.sttActive = false
	
	// Stop any ongoing keep-alive timer
	if s.keepAliveTimer != nil {
		s.keepAliveTimer.Stop()
		s.keepAliveTimer = nil
		logger.ServiceDebugf("SILENCE", "stopped keep-alive timer - STT connection inactive")
	}
}

// Events returns a channel of silence events
func (s *Service) Events() <-chan silence.SilenceEvent {
	return s.events
}

// IsInSilence returns current silence state
func (s *Service) IsInSilence() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inSilence
}

// SilenceDuration returns how long we've been in silence
func (s *Service) SilenceDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.inSilence {
		return 0
	}

	return time.Since(s.silenceStarted)
}



// sendDeepgramKeepAlive sends a Deepgram KeepAlive message
func (s *Service) sendDeepgramKeepAlive() {
	// Send a special keep-alive audio chunk that will be interpreted as a keep-alive message
	// We'll use a special marker in the audio sink to send a text message instead
	keepAliveChunk := []byte("KEEP_ALIVE_MARKER")
	
	if s.audioSink != nil {
		logger.ServiceDebugf("SILENCE", "Sending Deepgram KeepAlive message via audio sink")
		if s.audioSink.SendAudio(speech.AudioChunk(keepAliveChunk)) {
			logger.ServiceDebugf("SILENCE", "✅  [SILENCE] Successfully sent Deepgram KeepAlive message")
		} else {
			logger.Warnf("⚠️ [SILENCE] Failed to send Deepgram KeepAlive message")
		}
	} else {
		logger.Warnf("⚠️ [SILENCE] audioSink is nil, cannot send Deepgram KeepAlive message")
	}
}

// Compile-time interface check
var _ silence.Handler = (*Service)(nil)
