package batch

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID           pgtype.UUID `json:"id"`
	AppID        pgtype.UUID `json:"app_id"`
	AccountID    pgtype.UUID `json:"account_id"`
	SessionID    pgtype.UUID `json:"session_id"`
	Status       JobStatus   `json:"status"`
	FilePath     string      `json:"file_path"`
	FileSize     int64       `json:"file_size"`
	ErrorMessage string      `json:"error_message,omitempty"`
	StartedAt    *time.Time  `json:"started_at,omitempty"`
	CompletedAt  *time.Time  `json:"completed_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type JobRepo interface {
	Create(ctx context.Context, appID, accountID, sessionID pgtype.UUID, filePath string, fileSize int64) (Job, error)
	Get(ctx context.Context, id pgtype.UUID) (Job, error)
	ListQueued(ctx context.Context, limit int) ([]Job, error)
	UpdateStatus(ctx context.Context, id pgtype.UUID, status JobStatus, errorMsg string) error
	ListByApp(ctx context.Context, appID pgtype.UUID, limit, offset int) ([]Job, error)
}
