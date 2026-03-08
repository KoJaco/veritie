-- name: CreateConnectionLog :one
INSERT INTO connection_logs (
  connection_id,
  ws_session_id,
  app_id,
  account_id,
  remote_addr,
  user_agent,
  subprotocols,
  event_type,
  event_data,
  started_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE($10, now()))
RETURNING *;

-- name: EndConnectionLog :one
UPDATE connection_logs
SET
  ended_at = COALESCE($2, now()),
  duration_ms = $3,
  error_message = $4,
  error_code = $5,
  messages_sent = COALESCE($6, messages_sent),
  messages_received = COALESCE($7, messages_received),
  audio_chunks_processed = COALESCE($8, audio_chunks_processed)
WHERE id = $1
RETURNING *;

-- name: AppendConnectionEvent :one
INSERT INTO connection_logs (
  connection_id,
  ws_session_id,
  app_id,
  account_id,
  event_type,
  event_data
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetLatestConnectionLogByConnID :one
SELECT *
FROM connection_logs
WHERE connection_id = $1
ORDER BY started_at DESC
LIMIT 1;

-- name: ListConnectionLogsByApp :many
SELECT *
FROM connection_logs
WHERE app_id = $1
  AND started_at >= $2
ORDER BY started_at DESC
LIMIT $3 OFFSET $4;

-- name: ListActiveConnectionsByApp :many
SELECT *
FROM connection_logs
WHERE app_id = $1
  AND ended_at IS NULL
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: CountActiveConnectionsByApp :one
SELECT COUNT(*) AS active
FROM connection_logs
WHERE app_id = $1
  AND ended_at IS NULL;


