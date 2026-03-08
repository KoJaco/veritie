-- Create enum type "subscription_status_enum"
CREATE TYPE "public"."subscription_status_enum" AS ENUM ('incomplete', 'incomplete_expired', 'trialing', 'active', 'past_due', 'canceled', 'unpaid', 'paused');
-- Create "accounts" table
CREATE TABLE "public"."accounts" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" text NOT NULL,
  "subscription_status" "public"."subscription_status_enum" NULL,
  "stripe_subscription_id" text NULL,
  "stripe_customer_id" text NULL,
  "trial_ends_at" timestamp NULL,
  "plan" text NOT NULL DEFAULT 'free',
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "stripe_customer_idx" to table: "accounts"
CREATE INDEX "stripe_customer_idx" ON "public"."accounts" ("stripe_customer_id");
-- Create enum type "action_enum"
CREATE TYPE "public"."action_enum" AS ENUM ('create', 'retrieve', 'update', 'delete');
-- Create enum type "entity_enum"
CREATE TYPE "public"."entity_enum" AS ENUM ('account', 'users', 'roles', 'permissions', 'forms', 'subscriptions', 'billing', 'usage_metrics');
-- Create enum type "notification_type_enum"
CREATE TYPE "public"."notification_type_enum" AS ENUM ('info', 'success', 'warning', 'error', 'critical');
-- Create enum type "batch_job_status_enum"
CREATE TYPE "public"."batch_job_status_enum" AS ENUM ('queued', 'processing', 'completed', 'failed');
-- Create enum type "user_role_enum"
CREATE TYPE "public"."user_role_enum" AS ENUM ('owner', 'admin', 'user');
-- Create "apps" table
CREATE TABLE "public"."apps" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL,
  "api_key" text NOT NULL,
  "config" json NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "app_account_fk" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "account_app_idx" to table: "apps"
CREATE INDEX "account_app_idx" ON "public"."apps" ("account_id");
-- Create "batch_jobs" table
CREATE TABLE "public"."batch_jobs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "app_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "status" "public"."batch_job_status_enum" NOT NULL,
  "file_path" text NOT NULL,
  "file_size" bigint NOT NULL,
  "config" jsonb NOT NULL,
  "result" jsonb NULL,
  "error_message" text NULL,
  "started_at" timestamp NULL,
  "completed_at" timestamp NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "batch_jobs_account_fk" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "batch_jobs_app_fk" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "batch_jobs_app_idx" to table: "batch_jobs"
CREATE INDEX "batch_jobs_app_idx" ON "public"."batch_jobs" ("app_id");
-- Create index "batch_jobs_created_at_idx" to table: "batch_jobs"
CREATE INDEX "batch_jobs_created_at_idx" ON "public"."batch_jobs" ("created_at");
-- Create index "batch_jobs_status_idx" to table: "batch_jobs"
CREATE INDEX "batch_jobs_status_idx" ON "public"."batch_jobs" ("status");
-- Create "sessions" table
CREATE TABLE "public"."sessions" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "app_id" uuid NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "closed_at" timestamp NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "session_app_fk" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "draft_function_aggs" table
CREATE TABLE "public"."draft_function_aggs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "session_id" uuid NOT NULL,
  "app_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "function_name" text NOT NULL,
  "total_detections" bigint NOT NULL DEFAULT 0,
  "highest_score" double precision NOT NULL DEFAULT 0,
  "avg_score" double precision NOT NULL DEFAULT 0,
  "first_detected" timestamp NOT NULL DEFAULT now(),
  "last_detected" timestamp NOT NULL DEFAULT now(),
  "sample_args" jsonb NULL,
  "version_count" bigint NOT NULL DEFAULT 1,
  "final_call_count" bigint NOT NULL DEFAULT 0,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "unique_session_function" UNIQUE ("session_id", "function_name"),
  CONSTRAINT "draft_aggs_account_fk" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "draft_aggs_app_fk" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "draft_aggs_session_fk" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "draft_aggs_app_function_idx" to table: "draft_function_aggs"
CREATE INDEX "draft_aggs_app_function_idx" ON "public"."draft_function_aggs" ("app_id", "function_name");
-- Create index "draft_aggs_session_idx" to table: "draft_function_aggs"
CREATE INDEX "draft_aggs_session_idx" ON "public"."draft_function_aggs" ("session_id");
-- Create "draft_function_stats" table
CREATE TABLE "public"."draft_function_stats" (
  "session_id" uuid NOT NULL,
  "app_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "total_draft_functions" bigint NOT NULL DEFAULT 0,
  "total_final_functions" bigint NOT NULL DEFAULT 0,
  "draft_to_final_ratio" double precision NOT NULL DEFAULT 0,
  "unique_functions" bigint NOT NULL DEFAULT 0,
  "avg_detection_latency" double precision NOT NULL DEFAULT 0,
  "top_function" text NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("session_id"),
  CONSTRAINT "draft_stats_account_fk" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "draft_stats_app_fk" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "draft_stats_session_fk" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "function_calls" table
CREATE TABLE "public"."function_calls" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "session_id" uuid NOT NULL,
  "name" text NOT NULL,
  "args" json NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "function_session_fk" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "function_schemas" table
CREATE TABLE "public"."function_schemas" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "app_id" uuid NOT NULL,
  "session_id" uuid NULL,
  "name" text NOT NULL,
  "description" text NULL,
  "parameters" json NOT NULL,
  "checksum" text NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "unique_app_checksum" UNIQUE ("app_id", "checksum"),
  CONSTRAINT "fk_function_schemas_app" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_function_schemas_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "function_schemas_app_name_idx" to table: "function_schemas"
CREATE INDEX "function_schemas_app_name_idx" ON "public"."function_schemas" ("app_id", "name");
-- Create index "function_schemas_session_idx" to table: "function_schemas"
CREATE INDEX "function_schemas_session_idx" ON "public"."function_schemas" ("session_id");
-- Set comment to column: "session_id" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."session_id" IS 'If this schema was used in a specific session';
-- Set comment to column: "parameters" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."parameters" IS 'The JSON Schema for the function''s parameters';
-- Set comment to column: "checksum" on table: "function_schemas"
COMMENT ON COLUMN "public"."function_schemas"."checksum" IS 'Hash of the parameters field for deduplication';
-- Create "permissions" table
CREATE TABLE "public"."permissions" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id" uuid NOT NULL,
  "entity" "public"."entity_enum" NOT NULL,
  "actions" "public"."action_enum" NOT NULL,
  "description" text NOT NULL DEFAULT '',
  "is_critical" boolean NOT NULL DEFAULT false,
  "is_owner_only" boolean NOT NULL DEFAULT false,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_permissions_account" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "permissions_account_id_idx" to table: "permissions"
CREATE INDEX "permissions_account_id_idx" ON "public"."permissions" ("account_id");
-- Create index "permissions_entity_action_idx" to table: "permissions"
CREATE INDEX "permissions_entity_action_idx" ON "public"."permissions" ("entity", "actions");
-- Create "roles" table
CREATE TABLE "public"."roles" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT '',
  "access_level" integer NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_roles_account" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "roles_account_id_idx" to table: "roles"
CREATE INDEX "roles_account_id_idx" ON "public"."roles" ("account_id");
-- Create index "unique_role_name_account" to table: "roles"
CREATE INDEX "unique_role_name_account" ON "public"."roles" ("name", "account_id");
-- Set comment to column: "access_level" on table: "roles"
COMMENT ON COLUMN "public"."roles"."access_level" IS 'Hierarchy level, e.g., 100 = owner, 50 = admin';
-- Create "permission_roles" table
CREATE TABLE "public"."permission_roles" (
  "role_id" uuid NOT NULL,
  "permission_id" uuid NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("role_id", "permission_id"),
  CONSTRAINT "fk_permission_roles_permission" FOREIGN KEY ("permission_id") REFERENCES "public"."permissions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_permission_roles_role" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "permission_roles_role_idx" to table: "permission_roles"
CREATE INDEX "permission_roles_role_idx" ON "public"."permission_roles" ("role_id");
-- Create "prompts" table
CREATE TABLE "public"."prompts" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "app_id" uuid NOT NULL,
  "session_id" uuid NOT NULL,
  "content" text NOT NULL,
  "checksum" text NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "unique_app_prompt_checksum" UNIQUE ("app_id", "checksum"),
  CONSTRAINT "fk_prompts_app" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_prompts_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Set comment to column: "session_id" on table: "prompts"
COMMENT ON COLUMN "public"."prompts"."session_id" IS 'If this prompt was created during a specific session';
-- Set comment to column: "checksum" on table: "prompts"
COMMENT ON COLUMN "public"."prompts"."checksum" IS 'Hash of the content for deduplication';
-- Create "users" table
CREATE TABLE "public"."users" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id" uuid NOT NULL,
  "email" text NOT NULL,
  "role" "public"."user_role_enum" NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "user_account_fk" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "account_user_idx" to table: "users"
CREATE INDEX "account_user_idx" ON "public"."users" ("account_id");
-- Create "role_users" table
CREATE TABLE "public"."role_users" (
  "user_id" uuid NOT NULL,
  "role_id" uuid NOT NULL,
  "assigned_by" uuid NULL,
  "assigned_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("user_id", "role_id"),
  CONSTRAINT "fk_role_users_assigned_by" FOREIGN KEY ("assigned_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "fk_role_users_role" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_role_users_user" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "role_users_unique_idx" to table: "role_users"
CREATE INDEX "role_users_unique_idx" ON "public"."role_users" ("user_id", "role_id");
-- Create index "role_users_user_id_idx" to table: "role_users"
CREATE INDEX "role_users_user_id_idx" ON "public"."role_users" ("user_id");
-- Create "session_function_schemas" table
CREATE TABLE "public"."session_function_schemas" (
  "session_id" uuid NOT NULL,
  "function_schema_id" uuid NOT NULL,
  PRIMARY KEY ("session_id", "function_schema_id"),
  CONSTRAINT "fk_sfs_function_schema" FOREIGN KEY ("function_schema_id") REFERENCES "public"."function_schemas" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_sfs_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "session_prompts" table
CREATE TABLE "public"."session_prompts" (
  "session_id" uuid NOT NULL,
  "prompt_id" uuid NOT NULL,
  PRIMARY KEY ("session_id", "prompt_id"),
  CONSTRAINT "fk_sp_prompt" FOREIGN KEY ("prompt_id") REFERENCES "public"."prompts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_sp_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "structured_output_schemas" table
CREATE TABLE "public"."structured_output_schemas" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "app_id" uuid NOT NULL,
  "session_id" uuid NULL,
  "name" text NULL,
  "description" text NULL,
  "schema" json NOT NULL,
  "parsing_guide" text NULL,
  "checksum" text NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "unique_so_app_checksum" UNIQUE ("app_id", "checksum"),
  CONSTRAINT "fk_so_schemas_app" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_so_schemas_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "so_schemas_app_name_idx" to table: "structured_output_schemas"
CREATE INDEX "so_schemas_app_name_idx" ON "public"."structured_output_schemas" ("app_id", "name");
-- Create index "so_schemas_session_idx" to table: "structured_output_schemas"
CREATE INDEX "so_schemas_session_idx" ON "public"."structured_output_schemas" ("session_id");
-- Set comment to column: "session_id" on table: "structured_output_schemas"
COMMENT ON COLUMN "public"."structured_output_schemas"."session_id" IS 'If this schema was first seen/used in a specific session';
-- Set comment to column: "name" on table: "structured_output_schemas"
COMMENT ON COLUMN "public"."structured_output_schemas"."name" IS 'Optional label for the schema (client-defined)';
-- Set comment to column: "schema" on table: "structured_output_schemas"
COMMENT ON COLUMN "public"."structured_output_schemas"."schema" IS 'The JSON Schema object for structured output';
-- Set comment to column: "parsing_guide" on table: "structured_output_schemas"
COMMENT ON COLUMN "public"."structured_output_schemas"."parsing_guide" IS 'Free-text LLM instructions associated with this schema';
-- Set comment to column: "checksum" on table: "structured_output_schemas"
COMMENT ON COLUMN "public"."structured_output_schemas"."checksum" IS 'Hash of the schema field for deduplication (e.g., SHA-256)';
-- Create "session_structured_output_schemas" table
CREATE TABLE "public"."session_structured_output_schemas" (
  "session_id" uuid NOT NULL,
  "structured_output_schema_id" uuid NOT NULL,
  PRIMARY KEY ("session_id", "structured_output_schema_id"),
  CONSTRAINT "fk_ssos_schema" FOREIGN KEY ("structured_output_schema_id") REFERENCES "public"."structured_output_schemas" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_ssos_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "session_usage_totals" table
CREATE TABLE "public"."session_usage_totals" (
  "session_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "app_id" uuid NOT NULL,
  "audio_seconds" double precision NOT NULL DEFAULT 0,
  "prompt_tokens" bigint NOT NULL DEFAULT 0,
  "completion_tokens" bigint NOT NULL DEFAULT 0,
  "saved_prompt_tokens" bigint NOT NULL DEFAULT 0,
  "cpu_active_seconds" double precision NOT NULL DEFAULT 0,
  "cpu_idle_seconds" double precision NOT NULL DEFAULT 0,
  "prompt_cost" double precision NOT NULL DEFAULT 0,
  "completion_cost" double precision NOT NULL DEFAULT 0,
  "saved_prompt_cost" double precision NOT NULL DEFAULT 0,
  "audio_cost" double precision NOT NULL DEFAULT 0,
  "cpu_cost" double precision NOT NULL DEFAULT 0,
  "total_cost" double precision NOT NULL DEFAULT 0,
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("session_id"),
  CONSTRAINT "account_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "app_id_fk" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "session_id_fk" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "account_idx" to table: "session_usage_totals"
CREATE INDEX "account_idx" ON "public"."session_usage_totals" ("account_id");
-- Create index "app_idx" to table: "session_usage_totals"
CREATE INDEX "app_idx" ON "public"."session_usage_totals" ("app_id");
-- Create "structured_outputs" table
CREATE TABLE "public"."structured_outputs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "session_id" uuid NOT NULL,
  "structured_output_schema_id" uuid NOT NULL,
  "output" json NOT NULL,
  "is_final" boolean NOT NULL DEFAULT true,
  "finalized_at" timestamp NOT NULL DEFAULT now(),
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_so_schema" FOREIGN KEY ("structured_output_schema_id") REFERENCES "public"."structured_output_schemas" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "fk_so_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "so_schema_idx" to table: "structured_outputs"
CREATE INDEX "so_schema_idx" ON "public"."structured_outputs" ("structured_output_schema_id");
-- Create index "so_session_idx" to table: "structured_outputs"
CREATE INDEX "so_session_idx" ON "public"."structured_outputs" ("session_id");
-- Set comment to column: "structured_output_schema_id" on table: "structured_outputs"
COMMENT ON COLUMN "public"."structured_outputs"."structured_output_schema_id" IS 'FK to the deduped schema used to produce this output';
-- Set comment to column: "output" on table: "structured_outputs"
COMMENT ON COLUMN "public"."structured_outputs"."output" IS 'Finalized structured output JSON object';
-- Set comment to column: "is_final" on table: "structured_outputs"
COMMENT ON COLUMN "public"."structured_outputs"."is_final" IS 'Reserved for future use if you store interim versions';
-- Create "transcripts" table
CREATE TABLE "public"."transcripts" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "session_id" uuid NOT NULL,
  "content" json NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "transcript_session_fk" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "usage_logs" table
CREATE TABLE "public"."usage_logs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "session_id" uuid NOT NULL,
  "app_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "type" text NOT NULL,
  "metric" json NOT NULL,
  "logged_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_usage_logs_account" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_usage_logs_app" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_usage_logs_session" FOREIGN KEY ("session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "usage_logs_by_account" to table: "usage_logs"
CREATE INDEX "usage_logs_by_account" ON "public"."usage_logs" ("account_id");
-- Create index "usage_logs_by_app_type_time" to table: "usage_logs"
CREATE INDEX "usage_logs_by_app_type_time" ON "public"."usage_logs" ("app_id", "type", "logged_at");
-- Set comment to column: "type" on table: "usage_logs"
COMMENT ON COLUMN "public"."usage_logs"."type" IS 'e.g., ''stt'', ''llm'', ''function_call''';
-- Set comment to column: "metric" on table: "usage_logs"
COMMENT ON COLUMN "public"."usage_logs"."metric" IS 'Flexible data payload: tokens, duration, latency, cost, etc.';
-- Create "user_invitations" table
CREATE TABLE "public"."user_invitations" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id" uuid NOT NULL,
  "email" text NOT NULL,
  "role_id" uuid NOT NULL,
  "token" text NOT NULL,
  "invited_by" uuid NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "user_invitations_token_unique" UNIQUE ("token"),
  CONSTRAINT "fk_user_invitations_account" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_user_invitations_invited_by" FOREIGN KEY ("invited_by") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_user_invitations_role" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "user_invitations_status_check" CHECK (status = ANY (ARRAY['pending'::text, 'accepted'::text, 'cancelled'::text]))
);
-- Create index "account_email_idx" to table: "user_invitations"
CREATE INDEX "account_email_idx" ON "public"."user_invitations" ("account_id", "email");
-- Set comment to column: "status" on table: "user_invitations"
COMMENT ON COLUMN "public"."user_invitations"."status" IS 'Valid values: pending, accepted, cancelled';
-- Create "user_preferences" table
CREATE TABLE "public"."user_preferences" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "settings" jsonb NOT NULL DEFAULT '{}',
  "user_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "user_preferences_user_unique" UNIQUE ("user_id"),
  CONSTRAINT "fk_user_preferences_user" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Set comment to column: "settings" on table: "user_preferences"
COMMENT ON COLUMN "public"."user_preferences"."settings" IS 'Arbitrary per-user preference map';
-- Create "user_profiles" table
CREATE TABLE "public"."user_profiles" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "full_name" text NOT NULL,
  "avatarBlob" text NOT NULL,
  "bio" text NOT NULL,
  "user_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_user_profiles_user" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "user_profiles_user_idx" to table: "user_profiles"
CREATE INDEX "user_profiles_user_idx" ON "public"."user_profiles" ("user_id");
-- Set comment to column: "avatarBlob" on table: "user_profiles"
COMMENT ON COLUMN "public"."user_profiles"."avatarBlob" IS 'Arbitrary avatar blob/URL/string as per app usage';
