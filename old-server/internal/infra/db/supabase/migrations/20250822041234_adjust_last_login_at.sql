-- Modify "users" table
ALTER TABLE "public"."users" ALTER COLUMN "last_login_at" DROP NOT NULL, ALTER COLUMN "last_login_at" DROP DEFAULT;
