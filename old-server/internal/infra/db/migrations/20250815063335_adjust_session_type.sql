-- Create enum type "session_type_enum"
CREATE TYPE "public"."session_type_enum" AS ENUM ('structured_output', 'functions', 'enhanced_text', 'markdown');
-- Modify "sessions" table
ALTER TABLE "public"."sessions" ADD COLUMN "type" "public"."session_type_enum" NOT NULL;
-- Create index "session_type_idx" to table: "sessions"
CREATE INDEX "session_type_idx" ON "public"."sessions" ("type");
