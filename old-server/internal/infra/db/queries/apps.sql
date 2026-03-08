-- name: GetAppByAPIKey :one
SELECT * FROM apps WHERE api_key = $1;

-- name: GetAppByID :one
SELECT * FROM apps WHERE id = $1;

-- name: ListAppsByAccount :many
SELECT * FROM apps WHERE account_id = $1 ORDER BY created_at DESC;

-- name: GetAppIDAndRequiredConfig :one
SELECT id, account_id, config FROM apps WHERE api_key = $1 LIMIT 1;
