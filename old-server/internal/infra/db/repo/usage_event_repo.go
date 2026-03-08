package repo

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"schma.ai/internal/domain/usage"
	db "schma.ai/internal/infra/db/generated"
)

// TODO: Define port for this repo with compile time check.


type UsageEventRepo struct {
	q *db.Queries
}

func NewUsageEventRepo(pool *pgxpool.Pool) *UsageEventRepo {
	return &UsageEventRepo{
		q: db.New(pool),
	}
}

var _ usage.UsageEventRepo = (*UsageEventRepo)(nil)

func (r *UsageEventRepo) LogEvent(ctx context.Context, event usage.UsageEvent) error {
	// Marshal the metric to JSON bytes for storage
	metricBytes, err := json.Marshal(event.Metric)
	if err != nil {
		return err
	}

	_, err = r.q.AddUsageLog(ctx, db.AddUsageLogParams{
		SessionID: event.SessionID,
		AppID:     event.AppID,
		AccountID: event.AccountID,
		Type:      event.Type,
		Metric:    metricBytes,
		LoggedAt:  pgtype.Timestamp{Time: event.LoggedAt, Valid: true},
	})
	return err
}

func (r *UsageEventRepo) ListEventsBySession(ctx context.Context, sessionID pgtype.UUID) ([]usage.UsageEvent, error) {
	dbEvents, err := r.q.ListUsageLogsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	events := make([]usage.UsageEvent, len(dbEvents))
	for i, dbEvent := range dbEvents {
		var metric interface{}
		if err := json.Unmarshal(dbEvent.Metric, &metric); err != nil {
			return nil, err
		}

		events[i] = usage.UsageEvent{
			SessionID: dbEvent.SessionID,
			AppID:     dbEvent.AppID,
			AccountID: dbEvent.AccountID,
			Type:      dbEvent.Type,
			Metric:    metric,
			LoggedAt:  dbEvent.LoggedAt.Time,
		}
	}

	return events, nil
}
