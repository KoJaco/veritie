package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"veritie.io/internal/infra/db/postgres/dbgen"
)

// EventsRepo wraps sqlc-generated queries for job_events operations.
type EventsRepo struct {
	queries *dbgen.Queries
}

func NewEventsRepo(pool *pgxpool.Pool) *EventsRepo {
	return &EventsRepo{queries: dbgen.New(pool)}
}

func (r *EventsRepo) Append(ctx context.Context, params dbgen.AppendJobEventParams) (dbgen.JobEvent, error) {
	if params.JobID == uuid.Nil {
		return dbgen.JobEvent{}, fmt.Errorf("job_id is required")
	}
	if params.Type == "" {
		return dbgen.JobEvent{}, fmt.Errorf("event type is required")
	}
	if params.Message == "" {
		return dbgen.JobEvent{}, fmt.Errorf("event message is required")
	}
	return r.queries.AppendJobEvent(ctx, params)
}

func (r *EventsRepo) ListByJobID(ctx context.Context, jobID uuid.UUID) ([]dbgen.JobEvent, error) {
	return r.queries.ListJobEventsByJobID(ctx, jobID)
}

func (r *EventsRepo) ListFromCursor(ctx context.Context, jobID uuid.UUID, createdAt time.Time, cursorID uuid.UUID) ([]dbgen.JobEvent, error) {
	return r.queries.ListJobEventsByJobIDFromCursor(ctx, dbgen.ListJobEventsByJobIDFromCursorParams{
		JobID:     jobID,
		CreatedAt: pgtype.Timestamp{Time: createdAt, Valid: true},
		Column3:   cursorID,
	})
}

func (r *EventsRepo) withTx(tx pgx.Tx) *dbgen.Queries {
	return r.queries.WithTx(tx)
}

// AppendInTx allows event writes to stay atomic with job updates.
func (r *EventsRepo) AppendInTx(ctx context.Context, tx pgx.Tx, params dbgen.AppendJobEventParams) (dbgen.JobEvent, error) {
	return r.withTx(tx).AppendJobEvent(ctx, params)
}
