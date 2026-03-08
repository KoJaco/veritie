package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"schma.ai/internal/domain/usage"
	db "schma.ai/internal/infra/db/generated"
)

// TODO: Define port for this repo with compile time check.


type MeterRepo struct {
	q *db.Queries
}

func NewMeterRepo(pool *pgxpool.Pool) *MeterRepo {
	return &MeterRepo{
		q: db.New(pool),
	}
}

var _ usage.UsageMeterRepo = (*MeterRepo)(nil)

// TODO: adjust metering to use appropriate IDs pertaining to session and account (post auth integration)

func (r *MeterRepo) Save(ctx context.Context, m usage.Meter, cost usage.Cost, savedPromptTokens int64, savedPromptCost float64) (db.SessionUsageTotal, error) {
    return r.q.AddSessionUsageTotal(ctx, db.AddSessionUsageTotalParams{
        SessionID:         m.SessionID,
        AccountID:         m.AccountID,
        AppID:             m.AppID,
        AudioSeconds:      m.AudioSeconds,
        PromptTokens:      m.PromptTokens,
        CompletionTokens:  m.CompletionTokens,
        SavedPromptTokens: savedPromptTokens,
        CpuActiveSeconds:  m.CPUActiveSeconds,
        CpuIdleSeconds:    m.CPUIdleSeconds,
        PromptCost:        cost.LLMInCost,
        CompletionCost:    cost.LLMOutCost,
        SavedPromptCost:   savedPromptCost,
        AudioCost:         cost.AudioCost,
        CpuCost:           cost.CPUCost,
        TotalCost:         cost.TotalCost,
    })
}
