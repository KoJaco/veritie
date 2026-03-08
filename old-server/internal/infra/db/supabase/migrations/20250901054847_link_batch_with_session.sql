-- Create enum type "session_kind_enum"
CREATE TYPE "public"."session_kind_enum" AS ENUM ('stream', 'batch');
-- Modify "sessions" table
ALTER TABLE "public"."sessions" ADD COLUMN "kind" "public"."session_kind_enum" NOT NULL DEFAULT 'stream';
-- Create index "session_kind_idx" to table: "sessions"
CREATE INDEX "session_kind_idx" ON "public"."sessions" ("kind");
-- Modify "batch_jobs" table
ALTER TABLE "public"."batch_jobs" ADD COLUMN "session_id" uuid NOT NULL, ADD CONSTRAINT "batch_jobs_session_fk" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
