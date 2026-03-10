package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"veritie.io/internal/infra/db/postgres/dbgen"
)

func TestJobsRepoValidation(t *testing.T) {
	repo := &JobsRepo{}
	ctx := context.Background()

	if _, err := repo.CreateJob(ctx, dbgen.CreateJobParams{}); err == nil {
		t.Fatalf("expected validation error for empty create params")
	}

	if _, err := repo.GetJobByIdempotencyKey(ctx, uuid.New(), ""); err == nil {
		t.Fatalf("expected validation error for empty idempotency key")
	}

	if _, err := repo.UpdateJobStatus(ctx, dbgen.UpdateJobStatusParams{}); err == nil {
		t.Fatalf("expected validation error for empty update params")
	}

	if _, err := repo.CreateRerunJob(ctx, dbgen.CreateRerunJobParams{}); err == nil {
		t.Fatalf("expected validation error for missing rerun_of_job_id")
	}
}
