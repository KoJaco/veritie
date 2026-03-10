package postgres

import (
	"context"
	"testing"

	"veritie.io/internal/infra/db/postgres/dbgen"
)

func TestEventsRepoValidation(t *testing.T) {
	repo := &EventsRepo{}
	ctx := context.Background()

	if _, err := repo.Append(ctx, dbgen.AppendJobEventParams{}); err == nil {
		t.Fatalf("expected validation error for empty append params")
	}

	if _, err := repo.Append(ctx, dbgen.AppendJobEventParams{Type: "ingest_started"}); err == nil {
		t.Fatalf("expected validation error for missing job id/message")
	}
}
