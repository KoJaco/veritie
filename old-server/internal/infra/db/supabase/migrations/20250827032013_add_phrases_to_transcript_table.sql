-- Modify "transcripts" table
ALTER TABLE "public"."transcripts" ADD COLUMN "phrases" jsonb NULL;
-- Set comment to column: "phrases" on table: "transcripts"
COMMENT ON COLUMN "public"."transcripts"."phrases" IS 'Phrases split by final transcript chunks';
