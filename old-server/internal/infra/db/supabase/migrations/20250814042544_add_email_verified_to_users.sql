-- Modify "permissions" table
ALTER TABLE "public"."permissions" ALTER COLUMN "actions" SET NOT NULL;
-- Create index "permissions_entity_action_idx" to table: "permissions"
CREATE INDEX "permissions_entity_action_idx" ON "public"."permissions" ("entity", "actions");
-- Modify "users" table
ALTER TABLE "public"."users" ADD COLUMN "email_verified" boolean NOT NULL DEFAULT false;
