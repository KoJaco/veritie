-- name: AddFunctionCall :one
INSERT INTO function_calls (session_id, name, args, created_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListFunctionCallsBySession :many
SELECT * FROM function_calls WHERE session_id = $1 ORDER BY created_at ASC;
