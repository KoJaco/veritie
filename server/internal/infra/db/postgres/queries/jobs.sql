-- name: GetAppRuntimeBundle :one
SELECT
  a.id,
  a.account_id,
  a.schema_id,
  a.active_schema_version_id,
  a.active_toolset_version_id,
  a.processing_config,
  a.runtime_behavior,
  a.llm_config
FROM apps a
JOIN schema_versions sv
  ON sv.id = a.active_schema_version_id
 AND sv.schema_id = a.schema_id
JOIN toolset_versions tsv
  ON tsv.id = a.active_toolset_version_id
 AND tsv.app_id = a.id
WHERE a.id = $1
  AND a.account_id = $2;

-- name: CreateJob :one
INSERT INTO jobs (
  app_id,
  account_id,
  schema_id,
  schema_version_id,
  toolset_version_id,
  status,
  idempotency_key,
  rerun_of_job_id,
  audio_uri,
  audio_size,
  audio_duration_ms,
  audio_content_type,
  config_snapshot,
  llm_config,
  error_message,
  started_at,
  completed_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;

-- name: GetJobByIDScoped :one
SELECT *
FROM jobs
WHERE id = $1
  AND app_id = $2
  AND account_id = $3;

-- name: GetJobByIdempotencyKey :one
SELECT *
FROM jobs
WHERE app_id = $1
  AND idempotency_key = $2;

-- name: UpdateJobStatus :one
UPDATE jobs
SET status = $2,
    error_message = $3,
    started_at = COALESCE($4, started_at),
    completed_at = COALESCE($5, completed_at),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListJobsByAppAccount :many
SELECT *
FROM jobs
WHERE app_id = $1
  AND account_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: ListJobsBeforeCursor :many
SELECT *
FROM jobs
WHERE app_id = $1
  AND account_id = $2
  AND (created_at, id) < ($3, $4::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $5;

-- name: CreateRerunJob :one
INSERT INTO jobs (
  app_id,
  account_id,
  schema_id,
  schema_version_id,
  toolset_version_id,
  status,
  idempotency_key,
  rerun_of_job_id,
  audio_uri,
  audio_size,
  audio_duration_ms,
  audio_content_type,
  config_snapshot,
  llm_config
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;
