package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"veritie.io/internal/infra/db/postgres/dbgen"
)

// JobsRepo wraps sqlc-generated queries for jobs table operations.
type JobsRepo struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewJobsRepo(pool *pgxpool.Pool) *JobsRepo {
	return &JobsRepo{
		pool:    pool,
		queries: dbgen.New(pool),
	}
}

func (r *JobsRepo) CreateJob(ctx context.Context, params dbgen.CreateJobParams) (dbgen.Job, error) {
	if params.AppID == uuid.Nil || params.AccountID == uuid.Nil {
		return dbgen.Job{}, fmt.Errorf("app_id and account_id are required")
	}
	if params.AudioUri == "" || params.AudioContentType == "" {
		return dbgen.Job{}, fmt.Errorf("audio_uri and audio_content_type are required")
	}
	if params.Status == "" {
		return dbgen.Job{}, fmt.Errorf("status is required")
	}
	return WithTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) (dbgen.Job, error) {
		queries := r.withTx(tx)
		runtime, err := queries.GetAppRuntimeBundle(ctx, dbgen.GetAppRuntimeBundleParams{
			ID:        params.AppID,
			AccountID: params.AccountID,
		})
		if err != nil {
			return dbgen.Job{}, fmt.Errorf("resolve app runtime bundle: %w", err)
		}

		snapshot, err := buildConfigSnapshot(runtime)
		if err != nil {
			return dbgen.Job{}, err
		}

		params.SchemaVersionID = runtime.ActiveSchemaVersionID
		params.SchemaID = runtime.SchemaID
		params.ToolsetVersionID = runtime.ActiveToolsetVersionID
		params.ConfigSnapshot = snapshot
		params.LlmConfig = runtime.LlmConfig

		return queries.CreateJob(ctx, params)
	})
}

func (r *JobsRepo) CreateRerunJob(ctx context.Context, params dbgen.CreateRerunJobParams) (dbgen.Job, error) {
	if !params.RerunOfJobID.Valid {
		return dbgen.Job{}, fmt.Errorf("rerun_of_job_id is required for rerun job")
	}
	return WithTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) (dbgen.Job, error) {
		queries := r.withTx(tx)
		runtime, err := queries.GetAppRuntimeBundle(ctx, dbgen.GetAppRuntimeBundleParams{
			ID:        params.AppID,
			AccountID: params.AccountID,
		})
		if err != nil {
			return dbgen.Job{}, fmt.Errorf("resolve app runtime bundle: %w", err)
		}

		snapshot, err := buildConfigSnapshot(runtime)
		if err != nil {
			return dbgen.Job{}, err
		}

		params.SchemaVersionID = runtime.ActiveSchemaVersionID
		params.SchemaID = runtime.SchemaID
		params.ToolsetVersionID = runtime.ActiveToolsetVersionID
		params.ConfigSnapshot = snapshot
		params.LlmConfig = runtime.LlmConfig

		return queries.CreateRerunJob(ctx, params)
	})
}

func (r *JobsRepo) GetJobByIDScoped(ctx context.Context, id, appID, accountID uuid.UUID) (dbgen.Job, error) {
	return r.queries.GetJobByIDScoped(ctx, dbgen.GetJobByIDScopedParams{
		ID:        id,
		AppID:     appID,
		AccountID: accountID,
	})
}

func (r *JobsRepo) GetJobByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (dbgen.Job, error) {
	if key == "" {
		return dbgen.Job{}, fmt.Errorf("idempotency key is required")
	}
	k := key
	return r.queries.GetJobByIdempotencyKey(ctx, dbgen.GetJobByIdempotencyKeyParams{
		AppID:          appID,
		IdempotencyKey: &k,
	})
}

func (r *JobsRepo) UpdateJobStatus(ctx context.Context, params dbgen.UpdateJobStatusParams) (dbgen.Job, error) {
	if params.ID == uuid.Nil {
		return dbgen.Job{}, fmt.Errorf("job id is required")
	}
	if params.Status == "" {
		return dbgen.Job{}, fmt.Errorf("status is required")
	}
	return r.queries.UpdateJobStatus(ctx, params)
}

func (r *JobsRepo) ListJobsByAppAccount(ctx context.Context, appID, accountID uuid.UUID, limit int32) ([]dbgen.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	return r.queries.ListJobsByAppAccount(ctx, dbgen.ListJobsByAppAccountParams{
		AppID:     appID,
		AccountID: accountID,
		Limit:     limit,
	})
}

func (r *JobsRepo) ListJobsBeforeCursor(ctx context.Context, appID, accountID uuid.UUID, cursorCreatedAt time.Time, cursorID uuid.UUID, limit int32) ([]dbgen.Job, error) {
	if limit <= 0 {
		limit = 50
	}
	return r.queries.ListJobsBeforeCursor(ctx, dbgen.ListJobsBeforeCursorParams{
		AppID:     appID,
		AccountID: accountID,
		CreatedAt: pgtype.Timestamp{Time: cursorCreatedAt, Valid: true},
		Column4:   cursorID,
		Limit:     limit,
	})
}

func (r *JobsRepo) withTx(tx pgx.Tx) *dbgen.Queries {
	return r.queries.WithTx(tx)
}

// UpdateJobStatusInTx allows state+event writes in one transaction.
func (r *JobsRepo) UpdateJobStatusInTx(ctx context.Context, tx pgx.Tx, params dbgen.UpdateJobStatusParams) (dbgen.Job, error) {
	return r.withTx(tx).UpdateJobStatus(ctx, params)
}

func buildConfigSnapshot(runtime dbgen.GetAppRuntimeBundleRow) ([]byte, error) {
	raw := map[string]json.RawMessage{
		"processing_config": runtime.ProcessingConfig,
		"runtime_behavior":  runtime.RuntimeBehavior,
		"llm_config":        runtime.LlmConfig,
	}
	snapshot, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal config snapshot: %w", err)
	}
	return snapshot, nil
}
