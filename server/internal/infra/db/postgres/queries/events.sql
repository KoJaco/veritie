-- name: AppendJobEvent :one
INSERT INTO job_events (
  job_id,
  type,
  message,
  progress,
  data
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;

-- name: ListJobEventsByJobID :many
SELECT *
FROM job_events
WHERE job_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListJobEventsByJobIDFromCursor :many
SELECT *
FROM job_events
WHERE job_id = $1
  AND (created_at, id) > ($2, $3::uuid)
ORDER BY created_at ASC, id ASC;
