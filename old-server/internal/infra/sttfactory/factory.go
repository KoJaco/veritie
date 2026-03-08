package sttfactory

import (
	"context"
	"fmt"

	"schma.ai/internal/domain/speech"
	"schma.ai/internal/infra/sttdeepgram"
	"schma.ai/internal/infra/sttgoogle"
	"schma.ai/internal/pkg/logger"
)

// Factory creates STT clients based on provider configuration
type Factory struct {
	googleConfig sttgoogle.Config
	deepgramKey  string
}

// NewFactory creates a new STT factory with default configurations
func NewFactory(googleConfig sttgoogle.Config, deepgramKey string) *Factory {
	return &Factory{
		googleConfig: googleConfig,
		deepgramKey:  deepgramKey,
	}
}

// CreateSTTClient creates an STT client for the specified provider
func (f *Factory) CreateSTTClient(ctx context.Context, provider string, diarization speech.DiarizationConfig) (speech.STTClient, error) {
    // Debug diarization passed into factory
    logger.Debugf("🔧 [STT] Factory creating client: provider=%s diarize.enable=%v min=%d max=%d",
        provider,
        diarization.EnableSpeakerDiarization,
        diarization.MinSpeakerCount,
        diarization.MaxSpeakerCount,
    )
	switch provider {
	case "google":
		client, err := sttgoogle.New(ctx, f.googleConfig, diarization)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google STT client: %w", err)
		}
		return client, nil

	case "deepgram", "": // Default to Deepgram if not specified
		if f.deepgramKey == "" {
			return nil, fmt.Errorf("DEEPGRAM_API_KEY environment variable not set")
		}
		client := sttdeepgram.New(f.deepgramKey, "nova-3", diarization)
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported STT provider: %s (supported: google, deepgram)", provider)
	}
}

// GetSupportedProviders returns the list of supported STT providers
func (f *Factory) GetSupportedProviders() []string {
	return []string{"google", "deepgram"}
}
