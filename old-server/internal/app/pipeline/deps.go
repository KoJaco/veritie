package pipeline

import (
	"context"

	db_domain "schma.ai/internal/domain/db"
	normalizer_domain "schma.ai/internal/domain/normalizer"
	redaction_domain "schma.ai/internal/domain/redaction"
	speech_domain "schma.ai/internal/domain/speech"
	usage_domain "schma.ai/internal/domain/usage"
)

// TODO: Move to domain package yeah...
// STTFactory creates STT clients based on provider configuration
type STTFactory interface {
	CreateSTTClient(ctx context.Context, provider string, diarization speech_domain.DiarizationConfig) (speech_domain.STTClient, error)
}

// Deps is constructed in cmd/memoinic-server and passed down to every new Pipeline
type Deps struct {
	STT        speech_domain.STTClient  // Default STT client (fallback)
	STTFactory STTFactory        // Factory for creating STT clients per session
	FP         speech_domain.FastParser // distilBert and Fasttext adapter
	LLM        speech_domain.LLM

	// Text Normalizer
	Normalizer normalizer_domain.Normalizer

	// Redaction
	RedactionService redaction_domain.Redactor

	// Usage repositories (to create usage accumulators per session)
	UsageMeterRepo usage_domain.UsageMeterRepo
	UsageEventRepo usage_domain.UsageEventRepo
	// Function specific agg
	DraftAggRepo   usage_domain.DraftAggRepo
	
	// Session data repositories for Functions
	FunctionCallsRepo   db_domain.FunctionCallsRepo
	FunctionSchemasRepo db_domain.FunctionSchemasRepo

	// Session data repositories for Structured Outputs
	StructuredOutputsRepo db_domain.StructuredOutputsRepo
	StructuredOutputSchemasRepo db_domain.StructuredOutputSchemasRepo

	TranscriptsRepo     db_domain.TranscriptsRepo
}


