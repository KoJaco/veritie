-- Modify "users" table
ALTER TABLE "public"."users" ADD COLUMN "provider" text NOT NULL, ADD COLUMN "provider_id" text NOT NULL;
-- Create index "provider_idx" to table: "users"
CREATE INDEX "provider_idx" ON "public"."users" ("provider");
