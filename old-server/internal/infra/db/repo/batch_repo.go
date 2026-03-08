package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"schma.ai/internal/domain/batch"
	db "schma.ai/internal/infra/db/generated"
)

// TODO: Define port for this repo with compile time check.

type BatchRepo struct {
	q *db.Queries
}

func NewBatchRepo(pool *pgxpool.Pool) *BatchRepo {
	return &BatchRepo{q: db.New(pool)}
}

var _ batch.JobRepo = (*BatchRepo)(nil)

func (r *BatchRepo) Create(ctx context.Context, appID, accountID, sessionID pgtype.UUID, filePath string, fileSize int64) (batch.Job, error) {
	row, err := r.q.CreateBatchJob(ctx, db.CreateBatchJobParams{
		AppID:     appID,
		AccountID: accountID,
		SessionID: sessionID,
		FilePath:  filePath,
		FileSize:  fileSize,
	})
	if err != nil {
		return batch.Job{}, err
	}

	return mapDBJobToDomain(row), nil
}

func (r *BatchRepo) Get(ctx context.Context, id pgtype.UUID) (batch.Job, error) {
	row, err := r.q.GetBatchJob(ctx, id)
	if err != nil {
		return batch.Job{}, err
	}

	return mapDBJobToDomain(row), nil
}

func (r *BatchRepo) ListQueued(ctx context.Context, limit int) ([]batch.Job, error) {
	rows, err := r.q.ListQueuedJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}

	jobs := make([]batch.Job, len(rows))
	for i, row := range rows {
		jobs[i] = mapDBJobToDomain(row)
	}

	return jobs, nil
}

func (r *BatchRepo) UpdateStatus(ctx context.Context, id pgtype.UUID, status batch.JobStatus, errorMsg string) error {
	return r.q.UpdateBatchJobStatus(ctx, db.UpdateBatchJobStatusParams{
		ID:           id,
		Status:       db.BatchJobStatusEnum(status),
		ErrorMessage: pgtype.Text{String: errorMsg, Valid: errorMsg != ""},
	})
}

func (r *BatchRepo) ListByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]batch.Job, error) {
	rows, err := r.q.ListJobsByApp(ctx, db.ListJobsByAppParams{
		AppID:  appID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]batch.Job, len(rows))
	for i, row := range rows {
		jobs[i] = mapDBJobToDomain(row)
	}

	return jobs, nil
}

func mapDBJobToDomain(row db.BatchJob) batch.Job {
	return batch.Job{
		ID:           row.ID,
		AppID:        row.AppID,
		AccountID:    row.AccountID,
		SessionID:    row.SessionID,
		Status:       batch.JobStatus(row.Status),
		FilePath:     row.FilePath,
		FileSize:     row.FileSize,
		ErrorMessage: row.ErrorMessage.String,
		StartedAt:    timeFromPgTimestamp(row.StartedAt),
		CompletedAt:  timeFromPgTimestamp(row.CompletedAt),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func timeFromPgTimestamp(t pgtype.Timestamp) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
