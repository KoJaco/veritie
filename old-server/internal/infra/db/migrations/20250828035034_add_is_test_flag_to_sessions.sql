-- Modify "sessions" table
ALTER TABLE "public"."sessions" ADD COLUMN "is_test" boolean NOT NULL DEFAULT false;
