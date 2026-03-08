package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/auth"
	db "schma.ai/internal/infra/db/generated"
)

// TODO: Define port for this repo with compile time check.

type AppRepo struct {
	q *db.Queries
}

func NewAppRepo(conn *pgx.Conn) *AppRepo {
	return &AppRepo{q: db.New(conn)}
}

var _ auth.AppFetcher = (*AppRepo)(nil)

func (r *AppRepo) FetchAppForAPIKey(ctx context.Context, key string) (auth.AppInfo, error) {
	row, err := r.q.GetAppByAPIKey(ctx, key)
	if err != nil {
		return auth.AppInfo{}, err
	}

	return mapRowToAppInfo(row)
}

func (r *AppRepo) FetchAppByID(ctx context.Context, id pgtype.UUID) (auth.AppInfo, error) {
	row, err := r.q.GetAppByID(ctx, id)
	if err != nil {
		return auth.AppInfo{}, err
	}

	return mapRowToAppInfo(row)
}

func mapRowToAppInfo(row db.App) (auth.AppInfo, error) {
	// If function_schema / structured_schema are JSON blobs in DB:
	return auth.AppInfo{
		AppID:     row.ID,
		AccountID: row.AccountID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,

		// UsageLimits: auth.UsageLimits{
		// 	MaxSessionsPerMinute:  row.MaxSessionsPerMinute,
		// 	MaxLLMTokensPerMin:    row.MaxLLMTokensPerMin,
		// 	MaxAudioSecondsPerMin: row.MaxAudioSecondsPerMin,
		// },
	}, nil
}
