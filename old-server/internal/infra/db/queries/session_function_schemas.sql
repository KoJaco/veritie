-- name: LinkFunctionSchemaToSession :exec
INSERT INTO session_function_schemas (session_id, function_schema_id)
VALUES (@session_id, @function_schema_id)
ON CONFLICT DO NOTHING;
