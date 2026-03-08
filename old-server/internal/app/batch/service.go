package batch

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/batch"
)

// Service provides high-level batch operations
type Service struct {
	repo batch.JobRepo
}

func NewService(repo batch.JobRepo) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) GetJob(ctx context.Context, id string) (batch.Job, error) {
	// Parse UUID from string
	var jobID pgtype.UUID
	if err := jobID.Scan(id); err != nil {
		return batch.Job{}, err
	}

	return s.repo.Get(ctx, jobID)
}
