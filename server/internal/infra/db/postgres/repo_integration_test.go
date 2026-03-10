//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"veritie.io/internal/config"
	"veritie.io/internal/infra/db/postgres/dbgen"
)

func TestJobsAndEventsRepoIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, config.DatabaseConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	accountID, appID := seedAccountAndApp(t, ctx, pool)
	jobs := NewJobsRepo(pool)
	events := NewEventsRepo(pool)

	idKey := "it-key-1"
	created, err := jobs.CreateJob(ctx, dbgen.CreateJobParams{
		AppID:            appID,
		AccountID:        accountID,
		Status:           "queued",
		IdempotencyKey:   &idKey,
		RerunOfJobID:     pgtype.UUID{},
		AudioUri:         "s3://bucket/audio.wav",
		AudioSize:        1024,
		AudioDurationMs:  30000,
		AudioContentType: "audio/wav",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	fetched, err := jobs.GetJobByIDScoped(ctx, created.ID, appID, accountID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected same job id")
	}

	_, err = jobs.GetJobByIdempotencyKey(ctx, appID, idKey)
	if err != nil {
		t.Fatalf("get by idempotency key: %v", err)
	}

	startedAt := pgtype.Timestamp{Time: time.Now(), Valid: true}
	updated, err := jobs.UpdateJobStatus(ctx, dbgen.UpdateJobStatusParams{
		ID:        created.ID,
		Status:    "running",
		StartedAt: startedAt,
	})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != "running" {
		t.Fatalf("expected running status")
	}

	_, err = events.Append(ctx, dbgen.AppendJobEventParams{
		JobID:    created.ID,
		Type:     "stt_started",
		Message:  "started",
		Progress: 0.1,
	})
	if err != nil {
		t.Fatalf("append event: %v", err)
	}

	listedEvents, err := events.ListByJobID(ctx, created.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(listedEvents) == 0 {
		t.Fatalf("expected at least one event")
	}

	listedJobs, err := jobs.ListJobsByAppAccount(ctx, appID, accountID, 10)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(listedJobs) == 0 {
		t.Fatalf("expected at least one job")
	}

	rerunRef := pgtype.UUID{Bytes: created.ID, Valid: true}
	rerunKey := "it-key-rerun"
	rerun, err := jobs.CreateRerunJob(ctx, dbgen.CreateRerunJobParams{
		AppID:            appID,
		AccountID:        accountID,
		Status:           "queued",
		IdempotencyKey:   &rerunKey,
		RerunOfJobID:     rerunRef,
		AudioUri:         created.AudioUri,
		AudioSize:        created.AudioSize,
		AudioDurationMs:  created.AudioDurationMs,
		AudioContentType: created.AudioContentType,
	})
	if err != nil {
		t.Fatalf("create rerun: %v", err)
	}
	if !rerun.RerunOfJobID.Valid || rerun.RerunOfJobID.Bytes != created.ID {
		t.Fatalf("expected rerun_of_job_id to reference source job")
	}
}

func seedAccountAndApp(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()

	accountID := uuid.New()
	appID := uuid.New()
	schemaID := uuid.New()
	schemaVersionID := uuid.New()
	toolsetID := uuid.New()
	toolsetVersionID := uuid.New()
	apiKeyPrefix := "vt_test"
	apiKeyHash := "hash-" + uuid.NewString()

	_, err := pool.Exec(ctx, `
		INSERT INTO accounts (id, name) VALUES ($1, $2)
	`, accountID, "acct-test")
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO schemas (id, account_id, name) VALUES ($1, $2, $3)
	`, schemaID, accountID, "default-schema")
	if err != nil {
		t.Fatalf("seed schema: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO schema_versions (id, schema_id, version, status, definition) VALUES ($1, $2, $3, $4, $5)
	`, schemaVersionID, schemaID, 1, "active", []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatalf("seed schema version: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx for app/toolset seed: %v", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO apps (
			id,
			account_id,
			name,
			schema_id,
			active_schema_version_id,
			active_toolset_version_id,
			processing_config,
			runtime_behavior,
			llm_config
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, appID, accountID, "app-test", schemaID, schemaVersionID, toolsetVersionID, []byte(`{"stt":"deepgram"}`), []byte(`{"stream":"sse"}`), []byte(`{"provider":"gemini"}`))
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO toolsets (id, app_id, name) VALUES ($1, $2, $3)
	`, toolsetID, appID, "default-toolset")
	if err != nil {
		t.Fatalf("seed toolset: %v", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO toolset_versions (id, toolset_id, app_id, version, status, definition)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, toolsetVersionID, toolsetID, appID, 1, "active", []byte(`{"tools":[]}`))
	if err != nil {
		t.Fatalf("seed toolset version: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit app/toolset seed: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (id, app_id, account_id, name, key_hash, key_prefix)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), appID, accountID, "default", apiKeyHash, apiKeyPrefix)
	if err != nil {
		t.Fatalf("seed api key: %v", err)
	}

	return accountID, appID
}
