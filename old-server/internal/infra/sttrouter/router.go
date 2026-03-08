package sttrouter

import (
	"context"
	"os"
	"strings"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/infra/sttdeepgram"
	"schma.ai/internal/infra/sttgoogle"
	"schma.ai/internal/pkg/logger"
)

// -----------------------------------------------------------------------------
//
//	Router implements speech.STT and proxies to a concrete backend.
//
// -----------------------------------------------------------------------------
type Router struct {
	impl speech.STTClient
	name string // for logging / metrics
}

// New initialises the desired backend once, and returns a Router that
// compiles-time satisfies the speech.STT interface.
//
// ENV  ─────────────────────────────────────────────────────────────────────────
//
//	SCHMA_STT_PROVIDER   "google" / "deepgram"   (default: "deepgram")
//	GOOGLE_CREDENTIALS  *only* if you pick Google
//	DEEPGRAM_API_KEY    *only* if you pick Deepgram
//
// -----------------------------------------------------------------------------
func New(ctx context.Context, gCfg sttgoogle.Config, diarization speech.DiarizationConfig) *Router {

	p := strings.ToLower(strings.TrimSpace(os.Getenv("SCHMA_STT_PROVIDER")))
	if p == "" {
		p = "deepgram" // sensible default
	}

	var (
		impl speech.STTClient
		err  error
	)

	switch p {
	case "google":
		impl, err = sttgoogle.New(ctx, gCfg, diarization) // your existing helper that reads creds from env
		if err != nil {
			logger.Errorf("❌ [STT] Error in assigned 'google' STT provider")
		}
	case "deepgram":
		key := os.Getenv("DEEPGRAM_API_KEY")
		impl = sttdeepgram.New(key, "nova-3", diarization)
	default:
		logger.Errorf("❌ [STT] Unknown provider %q – set SCHMA_STT_PROVIDER=google|deepgram", p)
	}

	logger.ServiceDebugf("STT", "Router initialized - provider=%s", p)
	return &Router{impl: impl, name: p}
}

// Compile-time guarantee
var _ speech.STTClient = (*Router)(nil)

// -----------------------------------------------------------------------------
//
//	Stream simply proxies to the underlying implementation.
//
// -----------------------------------------------------------------------------
func (r *Router) Stream(
	ctx context.Context,
	audio <-chan speech.AudioChunk,
) (<-chan speech.Transcript, error) {
	return r.impl.Stream(ctx, audio)
}
