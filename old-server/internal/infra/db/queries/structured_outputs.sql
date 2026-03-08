
-- name: AddStructuredOutput :one
INSERT INTO structured_outputs (
  session_id,
  structured_output_schema_id,
  output,
  is_final,
  finalized_at,
  created_at
)
VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListStructuredOutputsBySession :many
SELECT *
FROM structured_outputs
WHERE session_id = $1
ORDER BY finalized_at ASC, created_at ASC;

-- name: GetLatestStructuredOutputForSessionAndSchema :one
SELECT *
FROM structured_outputs
WHERE session_id = $1
  AND structured_output_schema_id = $2
ORDER BY finalized_at DESC, created_at DESC
LIMIT 1;

-- name: GetStructuredOutputByID :one
SELECT *
FROM structured_outputs
WHERE id = $1;
