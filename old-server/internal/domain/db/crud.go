package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
)

// Database repositories for session data storage
type FunctionCallsRepo interface {
	StoreFunctionCall(ctx context.Context, sessionID pgtype.UUID, name string, args map[string]interface{}) (db.FunctionCall, error)
	GetFunctionCallsBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.FunctionCall, error)
}

type FunctionSchemasRepo interface {
	StoreOrGetFunctionSchema(ctx context.Context, appID, sessionID pgtype.UUID, config speech.FunctionConfigWithoutContext) (pgtype.UUID, error)
	LinkSchemaToSession(ctx context.Context, sessionID, schemaID pgtype.UUID) error
	GetSchemasBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.FunctionSchema, error)
}

type TranscriptsRepo interface {
	StoreTranscript(ctx context.Context, sessionID pgtype.UUID, transcript speech.Transcript) (db.Transcript, error)
	GetTranscriptsBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.Transcript, error)
}

type StructuredOutputsRepo interface {
	StoreStructuredOutput(ctx context.Context, sessionID pgtype.UUID, output speech.StructuredOutputUpdate) (db.StructuredOutput, error)
    // StoreStructuredOutputWithSchema persists output using an explicit schema ID and does not attempt discovery
    StoreStructuredOutputWithSchema(ctx context.Context, sessionID, schemaID pgtype.UUID, output speech.StructuredOutputUpdate) (db.StructuredOutput, error)
	GetStructuredOutputsBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.StructuredOutput, error)
}

type StructuredOutputSchemasRepo interface {
	StoreOrGetSchema(ctx context.Context, appID, sessionID pgtype.UUID, config speech.StructuredOutputConfig) (pgtype.UUID, error)
	LinkSchemaToSession(ctx context.Context, sessionID, schemaID pgtype.UUID) error
	GetSchemasBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.StructuredOutputSchema, error)
}