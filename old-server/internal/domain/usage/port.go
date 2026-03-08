package usage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "schma.ai/internal/infra/db/generated"
)

// Port exposed for persistence.
type UsageMeterRepo interface {
    // Save persists counts, savings, and calculated cost in one idempotent txn.
    Save(ctx context.Context, meter Meter, c Cost, savedPromptTokens int64, savedPromptCost float64) (db.SessionUsageTotal, error) // upsert

    // We need returning?
    // TODO: determine if we want returning (adjust sql query and re-generate)
}

// UsageEvent represents a single usage event for detailed logging
type UsageEvent struct {
	SessionID pgtype.UUID
	AppID     pgtype.UUID
	AccountID pgtype.UUID
	Type      string      // e.g., 'stt', 'llm', 'function_call'
	Metric    interface{} // Flexible data: tokens, duration, latency, cost, etc.
	LoggedAt  time.Time
}

// UsageEventRepo for detailed event logging (complements session totals)
type UsageEventRepo interface {
	// LogEvent records individual usage events for analytics/debugging
	LogEvent(ctx context.Context, event UsageEvent) error

	// ListEventsBySession retrieves all events for a session
	ListEventsBySession(ctx context.Context, sessionID pgtype.UUID) ([]UsageEvent, error)
}
