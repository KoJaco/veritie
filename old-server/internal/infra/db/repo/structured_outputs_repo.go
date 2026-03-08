package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db_domain "schma.ai/internal/domain/db"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
)

var _ db_domain.StructuredOutputsRepo = (*StructuredOutputsRepo)(nil)

type StructuredOutputsRepo struct {
	q *db.Queries
}

func NewStructuredOutputsRepo(pool *pgxpool.Pool) *StructuredOutputsRepo {
	return &StructuredOutputsRepo{
		q: db.New(pool),
	}
}

func (r *StructuredOutputsRepo) StoreStructuredOutput(ctx context.Context, sessionID pgtype.UUID, output speech.StructuredOutputUpdate) (db.StructuredOutput, error) {
    // 1) Resolve latest structured output schema linked to this session
    schemas, err := r.q.ListStructuredOutputSchemasBySession(ctx, sessionID)
    if err != nil {
        return db.StructuredOutput{}, err
    }
    if len(schemas) == 0 {
        return db.StructuredOutput{}, errors.New("no structured output schema linked to session")
    }
    // List is ordered ASC by created_at; pick the last as latest
    latestSchema := schemas[len(schemas)-1]

    // 4) Build output JSON payload
    payload := map[string]any{
        "rev": output.Rev,
    }
    if output.Final != nil {
        payload["final"] = output.Final
    }
    if output.Delta != nil {
        payload["delta"] = output.Delta
    }
    blob, err := json.Marshal(payload)
    if err != nil {
        return db.StructuredOutput{}, err
    }

    // 5) is_final flag
    isFinal := output.Final != nil

    // 6) Insert row
    row, err := r.q.AddStructuredOutput(ctx, db.AddStructuredOutputParams{
        SessionID:                sessionID,
        StructuredOutputSchemaID: latestSchema.ID,
        Output:                   blob,
        IsFinal:                  isFinal,
        CreatedAt:                pgtype.Timestamp{Time: time.Now(), Valid: true},
        FinalizedAt:              pgtype.Timestamp{Time: time.Now(), Valid: true},
    })
    if err != nil {
        return db.StructuredOutput{}, err
    }
    return row, nil
}

func (r *StructuredOutputsRepo) GetStructuredOutputsBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.StructuredOutput, error) {
    return r.q.ListStructuredOutputsBySession(ctx, sessionID)
}

// StoreStructuredOutputWithSchema persists a structured output using an explicit schema ID.
// This bypasses session->schema discovery and should be used when caller knows the schema used for the session.
func (r *StructuredOutputsRepo) StoreStructuredOutputWithSchema(ctx context.Context, sessionID, schemaID pgtype.UUID, output speech.StructuredOutputUpdate) (db.StructuredOutput, error) {
    payload := map[string]any{"rev": output.Rev}
    if output.Final != nil {
        payload["final"] = output.Final
    }
    if output.Delta != nil {
        payload["delta"] = output.Delta
    }
    blob, err := json.Marshal(payload)
    if err != nil {
        return db.StructuredOutput{}, err
    }
    isFinal := output.Final != nil

    return r.q.AddStructuredOutput(ctx, db.AddStructuredOutputParams{
        SessionID:                sessionID,
        StructuredOutputSchemaID: schemaID,
        Output:                   blob,
        IsFinal:                  isFinal,
        CreatedAt:                pgtype.Timestamp{Time: time.Now(), Valid: true},
        FinalizedAt:              pgtype.Timestamp{Time: time.Now(), Valid: true},
    })
}





