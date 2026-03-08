-- name: CreateBatchJob :one
INSERT INTO batch_jobs (
  app_id, account_id, session_id, file_path, file_size
) VALUES (
  $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetBatchJob :one
SELECT * FROM batch_jobs WHERE id = $1;

-- name: GetBatchJobWithSession :one
SELECT b.*, s.kind, s.created_at AS session_created_at, s.closed_at AS session_closed_at
FROM batch_jobs b
JOIN sessions s ON s.id = b.session_id
WHERE b.id = $1;

-- name: ListQueuedJobs :many
SELECT * FROM batch_jobs 
WHERE status = 'queued'::batch_job_status_enum
ORDER BY created_at ASC 
LIMIT $1;

-- name: UpdateBatchJobStatus :exec
UPDATE batch_jobs 
SET 
  status = @status::batch_job_status_enum,
  started_at = CASE WHEN @status::batch_job_status_enum = 'processing'::batch_job_status_enum THEN now() ELSE started_at END,
  completed_at = CASE WHEN @status::batch_job_status_enum IN ('completed'::batch_job_status_enum, 'failed'::batch_job_status_enum) THEN now() ELSE completed_at END,
  error_message = @error_message,
  updated_at = now()
WHERE id = @id;

-- name: ListJobsByApp :many
SELECT * FROM batch_jobs 
WHERE app_id = $1 
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;