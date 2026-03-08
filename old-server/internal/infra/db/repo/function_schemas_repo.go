package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db_domain "schma.ai/internal/domain/db"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
)

// compile-time check
var _ db_domain.FunctionSchemasRepo = (*FunctionSchemasRepo)(nil)

type FunctionSchemasRepo struct {
	q *db.Queries
}

func NewFunctionSchemasRepo(pool *pgxpool.Pool) *FunctionSchemasRepo {
	return &FunctionSchemasRepo{
		q: db.New(pool),
	}
}

// calculateChecksum generates a SHA256 checksum for the entire function config
func (r *FunctionSchemasRepo) calculateChecksum(config speech.FunctionConfigWithoutContext) (string, error) {
	// Marshal the entire config for checksum calculation
	configBytes, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(configBytes)
	return hex.EncodeToString(hash[:]), nil
}

// StoreOrGetFunctionSchema stores a function config or gets existing one if already exists (deduplication)
func (r *FunctionSchemasRepo) StoreOrGetFunctionSchema(ctx context.Context, appID, sessionID pgtype.UUID, config speech.FunctionConfigWithoutContext) (pgtype.UUID, error) {
	// Calculate checksum for the entire config
	checksum, err := r.calculateChecksum(config)
	if err != nil {
		return pgtype.UUID{}, err
	}

	// Try to get existing schema by checksum first
	existingID, err := r.q.GetFunctionSchemaIDByChecksum(ctx, db.GetFunctionSchemaIDByChecksumParams{
		AppID:    appID,
		Checksum: checksum,
	})
	if err == nil {
		// Schema already exists, return its ID
		return existingID, nil
	}

	// Schema doesn't exist, try to insert new one
	// Marshal declarations to JSON for storage
	declarationsBytes, err := json.Marshal(config.Declarations)
	if err != nil {
		return pgtype.UUID{}, err
	}

	description := pgtype.Text{String: config.Description, Valid: config.Description != ""}
	name := pgtype.Text{String: config.Name, Valid: config.Name != ""}        
	parsingGuide := pgtype.Text{String: config.ParsingGuide, Valid: config.ParsingGuide != ""}
	updateMS := pgtype.Int4{Int32: int32(config.UpdateMs), Valid: config.UpdateMs > 0}

	parsingStrategy := db.SchemaParsingStrategyEnum(config.ParsingConfig.ParsingStrategy)

	schemaID, err := r.q.InsertFunctionSchemaIfNotExists(ctx, db.InsertFunctionSchemaIfNotExistsParams{
		AppID:         appID,
		SessionID:     sessionID,
		Name:          name,
		Description:   description,
		ParsingGuide:  parsingGuide,
		UpdateMs:      updateMS,
		ParsingStrategy: parsingStrategy,
		Declarations:  declarationsBytes,
		Checksum:      checksum,
		CreatedAt:     pgtype.Timestamp{Time: time.Now(), Valid: true},
	})

	if err != nil {
		// Insert failed, might be due to race condition (another process inserted same schema)
		// Try to get existing schema again
		existingID, getErr := r.q.GetFunctionSchemaIDByChecksum(ctx, db.GetFunctionSchemaIDByChecksumParams{
			AppID:    appID,
			Checksum: checksum,
		})
		if getErr == nil {
			// Found existing schema, return its ID
			return existingID, nil
		}
		// Both insert and get failed, return original insert error
		return pgtype.UUID{}, err
	}

	return schemaID, nil
}

// LinkSchemaToSession creates a many-to-many relationship between session and function schema
func (r *FunctionSchemasRepo) LinkSchemaToSession(ctx context.Context, sessionID, schemaID pgtype.UUID) error {
	return r.q.LinkFunctionSchemaToSession(ctx, db.LinkFunctionSchemaToSessionParams{
		SessionID:        sessionID,
		FunctionSchemaID: schemaID,
	})
}

// GetSchemasBySession retrieves all function schemas for a session
func (r *FunctionSchemasRepo) GetSchemasBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.FunctionSchema, error) {
	return r.q.ListFunctionSchemasBySession(ctx, sessionID)
}
