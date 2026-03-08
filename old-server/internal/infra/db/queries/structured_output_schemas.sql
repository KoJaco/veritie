-- name: InsertStructuredOutputSchemaIfNotExists :one
INSERT INTO structured_output_schemas (
  app_id,
  session_id,
  name,
  description,
  schema,
  parsing_guide,
  update_ms,
  parsing_strategy,
  checksum,
  created_at
)
VALUES (
  @app_id,
  @session_id,
  @name,
  @description,
  @schema,
  @parsing_guide,
  @update_ms,
  @parsing_strategy,
  @checksum,
  @created_at
)
ON CONFLICT (app_id, checksum) DO NOTHING
RETURNING id;

-- name: GetStructuredOutputSchemaIDByChecksum :one
SELECT id
FROM structured_output_schemas
WHERE app_id = @app_id AND checksum = @checksum;

-- name: ListStructuredOutputSchemasBySession :many
SELECT sos.* FROM structured_output_schemas sos
JOIN session_structured_output_schemas ssos ON sos.id = ssos.structured_output_schema_id
WHERE ssos.session_id = $1 ORDER BY sos.created_at ASC;

-- name: AttachStructuredOutputSchemaToSession :exec
INSERT INTO session_structured_output_schemas (session_id, structured_output_schema_id)
VALUES ($1, $2)
ON CONFLICT (session_id, structured_output_schema_id) DO NOTHING;
