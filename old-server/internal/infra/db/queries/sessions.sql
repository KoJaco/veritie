-- name: CreateSession :one
INSERT INTO sessions (app_id, is_test, kind, created_at, closed_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = @id;

-- name: UpdateSessionClosedAt :exec
UPDATE sessions SET closed_at = $1 WHERE id = $2;

-- name: ListSessionsByApp :many
SELECT * FROM sessions WHERE app_id = $1 ORDER BY created_at DESC;
