package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db_domain "schma.ai/internal/domain/db"
	db "schma.ai/internal/infra/db/generated"
)

// compile-time check
var _ db_domain.FunctionCallsRepo = (*FunctionCallsRepo)(nil)

type FunctionCallsRepo struct {
	q *db.Queries

}

func NewFunctionCallsRepo(pool *pgxpool.Pool) *FunctionCallsRepo {
	return &FunctionCallsRepo{
		q: db.New(pool),
	}
}

// StoreFunctionCall stores a function call in the database
func (r *FunctionCallsRepo) StoreFunctionCall(ctx context.Context, sessionID pgtype.UUID, name string, args map[string]interface{}) (db.FunctionCall, error) {
	// Marshal args to JSON
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return db.FunctionCall{}, err
	}

	return r.q.AddFunctionCall(ctx, db.AddFunctionCallParams{
		SessionID: sessionID,
		Name:      name,
		Args:      argsBytes,
		CreatedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
	})
}

// GetFunctionCallsBySession retrieves all function calls for a session
func (r *FunctionCallsRepo) GetFunctionCallsBySession(ctx context.Context, sessionID pgtype.UUID) ([]db.FunctionCall, error) {
	return r.q.ListFunctionCallsBySession(ctx, sessionID)
}
