-- name: LinkStructuredOutputSchemaToSession :exec
INSERT INTO session_structured_output_schemas (session_id, structured_output_schema_id)
VALUES (@session_id, @structured_output_schema_id)
ON CONFLICT DO NOTHING;
