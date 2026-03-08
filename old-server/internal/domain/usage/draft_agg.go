package usage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// DraftAgg represents aggregated draft function call data
type DraftAgg struct {
	SessionID       pgtype.UUID
	AppID           pgtype.UUID
	AccountID       pgtype.UUID
	FunctionName    string
	TotalDetections int64       // Total number of times this function was detected
	HighestScore    float64     // Highest similarity score achieved
	AvgScore        float64     // Average similarity score
	FirstDetected   time.Time   // When first detected
	LastDetected    time.Time   // When last detected
	SampleArgs      interface{} // Sample arguments from highest scoring detection
	VersionCount    int64       // Number of different argument variations
	FinalCallCount  int64       // Number of times this became a final function call
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DraftAggStats provides session-level aggregation statistics
type DraftAggStats struct {
	SessionID           pgtype.UUID
	AppID               pgtype.UUID
	AccountID           pgtype.UUID
	TotalDraftFunctions int64   // Total draft functions detected
	TotalFinalFunctions int64   // Total final functions executed
	DraftToFinalRatio   float64 // Ratio of drafts to finals (higher = more explorative)
	UniqueFunction      int64   // Number of unique functions detected
	AvgDetectionLatency float64 // Average time from draft to final (seconds)
	TopFunction         string  // Most frequently detected function
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// DraftAggRepo interface for draft aggregation persistence
type DraftAggRepo interface {
	// UpsertDraftAgg updates or inserts draft aggregation data
	UpsertDraftAgg(ctx context.Context, agg DraftAgg) error

	// GetDraftAggsBySession retrieves all draft aggregations for a session
	GetDraftAggsBySession(ctx context.Context, sessionID pgtype.UUID) ([]DraftAgg, error)

	// UpsertDraftAggStats updates session-level statistics
	UpsertDraftAggStats(ctx context.Context, stats DraftAggStats) error

	// GetDraftAggStats retrieves session statistics
	GetDraftAggStats(ctx context.Context, sessionID pgtype.UUID) (DraftAggStats, error)
}
