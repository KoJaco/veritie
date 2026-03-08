-- name: ListFunctionSchemasBySession :many
SELECT fs.* FROM function_schemas fs
JOIN session_function_schemas sfs ON fs.id = sfs.function_schema_id
WHERE sfs.session_id = $1 ORDER BY fs.created_at ASC;

-- name: InsertFunctionSchemaIfNotExists :one
INSERT INTO function_schemas (
    app_id,
    session_id,
    name,
    description,
    parsing_guide,
    update_ms,
    parsing_strategy,
    declarations,
    checksum,
    created_at
)
VALUES (
    @app_id,
    @session_id,
    @name,
    @description,
    @parsing_guide,
    @update_ms,
    @parsing_strategy,
    @declarations,
    @checksum,
    @created_at
)
ON CONFLICT (app_id, checksum) DO NOTHING
RETURNING id;

-- name: GetFunctionSchemaIDByChecksum :one
SELECT id FROM function_schemas
WHERE app_id = @app_id AND checksum = @checksum;
