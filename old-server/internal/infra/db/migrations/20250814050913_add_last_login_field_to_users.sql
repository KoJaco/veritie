-- Modify "users" table
ALTER TABLE "public"."users" ADD COLUMN "last_login_at" timestamp NOT NULL DEFAULT now();
