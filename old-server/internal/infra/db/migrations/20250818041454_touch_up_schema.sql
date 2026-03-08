-- Modify "function_schemas" table
ALTER TABLE "public"."function_schemas" ALTER COLUMN "name" DROP NOT NULL, DROP COLUMN "parameters", ADD COLUMN "parsing_guide" text NULL, ADD COLUMN "update_ms" integer NULL, ADD COLUMN "declarations" json NOT NULL;
-- Set comment to column: "name" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."name" IS 'Optional label for the function config (client-defined)';
-- Set comment to column: "checksum" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."checksum" IS 'Hash of the entire function config for deduplication';
-- Set comment to column: "parsing_guide" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."parsing_guide" IS 'Free-text LLM instructions associated with this function config';
-- Set comment to column: "update_ms" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."update_ms" IS 'Update frequency in milliseconds for real-time parsing';
-- Set comment to column: "declarations" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."declarations" IS 'The complete function declarations array for this config';
-- Modify "structured_output_schemas" table
ALTER TABLE "public"."structured_output_schemas" ADD COLUMN "update_ms" integer NULL;
-- Set comment to column: "update_ms" on table: "structured_output_schemas"
COMMENT ON COLUMN "public"."structured_output_schemas"."update_ms" IS 'Update frequency in milliseconds for real-time parsing';
-- Modify "transcripts" table
ALTER TABLE "public"."transcripts" DROP COLUMN "content", ADD COLUMN "text" text NOT NULL, ADD COLUMN "is_final" boolean NOT NULL DEFAULT false, ADD COLUMN "confidence" real NULL, ADD COLUMN "stability" real NULL, ADD COLUMN "chunk_dur_sec" double precision NULL, ADD COLUMN "channel" integer NULL, ADD COLUMN "words" jsonb NULL, ADD COLUMN "turns" jsonb NULL;
-- Create index "transcripts_confidence_idx" to table: "transcripts"
CREATE INDEX "transcripts_confidence_idx" ON "public"."transcripts" ("confidence");
-- Create index "transcripts_session_final_idx" to table: "transcripts"
CREATE INDEX "transcripts_session_final_idx" ON "public"."transcripts" ("session_id", "is_final");
-- Set comment to column: "text" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."text" IS 'The transcript text content';
-- Set comment to column: "is_final" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."is_final" IS 'Whether this is a final transcript or interim';
-- Set comment to column: "confidence" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."confidence" IS 'Overall confidence score for the transcript (0.0-1.0)';
-- Set comment to column: "stability" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."stability" IS 'Stability score for streaming transcripts (0.0-1.0)';
-- Set comment to column: "chunk_dur_sec" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."chunk_dur_sec" IS 'Duration of the audio chunk in seconds';
-- Set comment to column: "channel" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."channel" IS 'Audio channel number for multi-channel audio';
-- Set comment to column: "words" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."words" IS 'Word-level data with timing, confidence, and speaker info';
-- Set comment to column: "turns" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."turns" IS 'Speaker diarization data with turn information';
-- Drop "session_prompts" table
DROP TABLE "public"."session_prompts";
-- Drop "prompts" table
DROP TABLE "public"."prompts";
