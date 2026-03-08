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


type DraftAggRepo struct {
	q *db.Queries
}

func NewDraftAggRepo(pool *pgxpool.Pool) *DraftAggRepo {
	return &DraftAggRepo{
		q: db.New(pool),
	}
}

var _ usage.DraftAggRepo = (*DraftAggRepo)(nil)

func (r *DraftAggRepo) UpsertDraftAgg(ctx context.Context, agg usage.DraftAgg) error {
	// Marshal sample args to JSON
	var sampleArgsBytes []byte
	if agg.SampleArgs != nil {
		var err error
		sampleArgsBytes, err = json.Marshal(agg.SampleArgs)
		if err != nil {
			return err
		}
	}

	_, err := r.q.UpsertDraftFunctionAgg(ctx, db.UpsertDraftFunctionAggParams{
		SessionID:       agg.SessionID,
		AppID:           agg.AppID,
		AccountID:       agg.AccountID,
		FunctionName:    agg.FunctionName,
		TotalDetections: agg.TotalDetections,
		HighestScore:    agg.HighestScore,
		AvgScore:        agg.AvgScore,
		FirstDetected:   pgtype.Timestamp{Time: agg.FirstDetected, Valid: true},
		LastDetected:    pgtype.Timestamp{Time: agg.LastDetected, Valid: true},
		SampleArgs:      sampleArgsBytes,
		VersionCount:    agg.VersionCount,
		FinalCallCount:  agg.FinalCallCount,
	})
	return err
}

func (r *DraftAggRepo) GetDraftAggsBySession(ctx context.Context, sessionID pgtype.UUID) ([]usage.DraftAgg, error) {
	dbAggs, err := r.q.GetDraftFunctionAggsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	aggs := make([]usage.DraftAgg, len(dbAggs))
	for i, dbAgg := range dbAggs {
		agg, err := r.mapDBToDraftAgg(dbAgg)
		if err != nil {
			return nil, err
		}
		aggs[i] = agg
	}

	return aggs, nil
}

func (r *DraftAggRepo) UpsertDraftAggStats(ctx context.Context, stats usage.DraftAggStats) error {
	_, err := r.q.UpsertDraftFunctionStats(ctx, db.UpsertDraftFunctionStatsParams{
		SessionID:           stats.SessionID,
		AppID:               stats.AppID,
		AccountID:           stats.AccountID,
		TotalDraftFunctions: stats.TotalDraftFunctions,
		TotalFinalFunctions: stats.TotalFinalFunctions,
		DraftToFinalRatio:   stats.DraftToFinalRatio,
		UniqueFunctions:     stats.UniqueFunction,
		AvgDetectionLatency: stats.AvgDetectionLatency,
		TopFunction:         pgtype.Text{String: stats.TopFunction, Valid: stats.TopFunction != ""},
	})
	return err
}

func (r *DraftAggRepo) GetDraftAggStats(ctx context.Context, sessionID pgtype.UUID) (usage.DraftAggStats, error) {
	dbStats, err := r.q.GetDraftFunctionStats(ctx, sessionID)
	if err != nil {
		return usage.DraftAggStats{}, err
	}

	return usage.DraftAggStats{
		SessionID:           dbStats.SessionID,
		AppID:               dbStats.AppID,
		AccountID:           dbStats.AccountID,
		TotalDraftFunctions: dbStats.TotalDraftFunctions,
		TotalFinalFunctions: dbStats.TotalFinalFunctions,
		DraftToFinalRatio:   dbStats.DraftToFinalRatio,
		UniqueFunction:      dbStats.UniqueFunctions,
		AvgDetectionLatency: dbStats.AvgDetectionLatency,
		TopFunction:         dbStats.TopFunction.String,
		CreatedAt:           dbStats.CreatedAt.Time,
		UpdatedAt:           dbStats.UpdatedAt.Time,
	}, nil
}

func (r *DraftAggRepo) mapDBToDraftAgg(dbAgg db.DraftFunctionAgg) (usage.DraftAgg, error) {
	var sampleArgs interface{}
	if len(dbAgg.SampleArgs) > 0 {
		if err := json.Unmarshal(dbAgg.SampleArgs, &sampleArgs); err != nil {
			return usage.DraftAgg{}, err
		}
	}

	return usage.DraftAgg{
		SessionID:       dbAgg.SessionID,
		AppID:           dbAgg.AppID,
		AccountID:       dbAgg.AccountID,
		FunctionName:    dbAgg.FunctionName,
		TotalDetections: dbAgg.TotalDetections,
		HighestScore:    dbAgg.HighestScore,
		AvgScore:        dbAgg.AvgScore,
		FirstDetected:   dbAgg.FirstDetected.Time,
		LastDetected:    dbAgg.LastDetected.Time,
		SampleArgs:      sampleArgs,
		VersionCount:    dbAgg.VersionCount,
		FinalCallCount:  dbAgg.FinalCallCount,
		CreatedAt:       dbAgg.CreatedAt.Time,
		UpdatedAt:       dbAgg.UpdatedAt.Time,
	}, nil
}
