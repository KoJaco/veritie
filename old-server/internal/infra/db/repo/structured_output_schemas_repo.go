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


var _ db_domain.StructuredOutputSchemasRepo = (*StructuredOutputSchemasRepo)(nil)

type StructuredOutputSchemasRepo struct {
	q *db.Queries
}


func NewStructuredOutputSchemasRepo(pool *pgxpool.Pool) *StructuredOutputSchemasRepo {
	return &StructuredOutputSchemasRepo{
		q: db.New(pool),
	}
}

// helper: calculateChecksum generates a SHA256 checksum for the parameters JSON
func (r * StructuredOutputSchemasRepo) calculateChecksum(parameters []byte) string {
	hash := sha256.Sum256(parameters)
	return hex.EncodeToString(hash[:])
}

func (r * StructuredOutputSchemasRepo) StoreOrGetSchema(ctx context.Context, appID, sessionID pgtype.UUID, config speech.StructuredOutputConfig) (pgtype.UUID, error) {
    // Marshal parameters to JSON for storage and checksum
    schemaBytes, err := json.Marshal(config.Schema)
	if err != nil {
		return pgtype.UUID{}, err
	}

	// Calculate checksum for deduplication
	checksum := r.calculateChecksum(schemaBytes)

    existingID, err := r.q.GetStructuredOutputSchemaIDByChecksum(ctx, db.GetStructuredOutputSchemaIDByChecksumParams{
		AppID:    appID,
		Checksum: checksum,
	})
    if err == nil {
        // Schema already exists, return its ID
        return existingID, nil
    }

	// Schema doesn't exist, try to insert new one
	description := pgtype.Text{String: config.Schema.Description, Valid: config.Schema.Description != ""}
    name := pgtype.Text{String: config.Schema.Name, Valid: config.Schema.Name != ""}
    parsingGuide := pgtype.Text{String: config.ParsingGuide, Valid: config.ParsingGuide != ""}
    updateMS := pgtype.Int4{Int32: int32(config.UpdateMs), Valid: config.UpdateMs > 0}

    schemaID, err := r.q.InsertStructuredOutputSchemaIfNotExists(ctx, db.InsertStructuredOutputSchemaIfNotExistsParams{
		AppID:         appID,
		SessionID:     sessionID,
		Name:          name,
		Description:   description,
		Schema:        schemaBytes,
		ParsingGuide:  parsingGuide,
		UpdateMs:      updateMS,
		ParsingStrategy: db.SchemaParsingStrategyEnum(config.ParsingConfig.ParsingStrategy),
		Checksum:      checksum,
		CreatedAt:     pgtype.Timestamp{Time: time.Now(), Valid: true},
	})

	if err != nil {
		// Insert failed, might be due to race condition (another process inserted same schema)
		// Try to get existing schema again
		existingID, getErr := r.q.GetStructuredOutputSchemaIDByChecksum(ctx, db.GetStructuredOutputSchemaIDByChecksumParams{
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

func (r * StructuredOutputSchemasRepo) LinkSchemaToSession(ctx context.Context, sessionID, schemaID pgtype.UUID) error {
	return r.q.LinkStructuredOutputSchemaToSession(ctx, db.LinkStructuredOutputSchemaToSessionParams{
		SessionID:        sessionID,
		StructuredOutputSchemaID: schemaID,
	})
}

func (r * StructuredOutputSchemasRepo) GetSchemasBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.StructuredOutputSchema, error) {
	return r.q.ListStructuredOutputSchemasBySession(ctx, sessionID)
}


