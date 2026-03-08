-- name: AddUsageLog :one
INSERT INTO usage_logs (session_id, app_id, account_id, type, metric, logged_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListUsageLogsBySession :many
SELECT * FROM usage_logs WHERE session_id = $1 ORDER BY logged_at ASC;
