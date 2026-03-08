-- name: UpsertConnectionState :one
INSERT INTO connection_states (
  connection_id,
  ws_session_id,
  app_id,
  account_id,
  llm_mode,
  active_session_id,
  connection_status,
  stt_provider,
  function_definitions_count,
  structured_schema_present,
  last_activity,
  ping_latency_ms,
  last_error,
  error_count
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, 0), COALESCE($10, false), $11, $12, $13, COALESCE($14, 0)
)
ON CONFLICT (connection_id) DO UPDATE SET
  ws_session_id = EXCLUDED.ws_session_id,
  app_id = EXCLUDED.app_id,
  account_id = EXCLUDED.account_id,
  llm_mode = EXCLUDED.llm_mode,
  active_session_id = EXCLUDED.active_session_id,
  connection_status = EXCLUDED.connection_status,
  stt_provider = EXCLUDED.stt_provider,
  function_definitions_count = EXCLUDED.function_definitions_count,
  structured_schema_present = EXCLUDED.structured_schema_present,
  last_activity = EXCLUDED.last_activity,
  ping_latency_ms = EXCLUDED.ping_latency_ms,
  last_error = EXCLUDED.last_error,
  error_count = EXCLUDED.error_count,
  updated_at = now()
RETURNING *;

-- name: UpdateConnectionStateOnClose :exec
UPDATE connection_states
SET connection_status = 'closed',
    active_session_id = NULL,
    updated_at = now()
WHERE connection_id = $1;

-- name: GetConnectionStateByConnID :one
SELECT *
FROM connection_states
WHERE connection_id = $1;

-- name: ListActiveConnectionStatesByApp :many
SELECT *
FROM connection_states
WHERE app_id = $1 AND connection_status IN ('active', 'idle')
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountActiveConnectionStatesByApp :one
SELECT COUNT(*) AS active
FROM connection_states
WHERE app_id = $1 AND connection_status IN ('active', 'idle');


