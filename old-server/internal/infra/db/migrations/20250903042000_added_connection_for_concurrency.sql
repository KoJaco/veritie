-- Create "connection_logs" table
CREATE TABLE "public"."connection_logs" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "connection_id" text NOT NULL,
  "ws_session_id" text NOT NULL,
  "app_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "remote_addr" inet NULL,
  "user_agent" text NULL,
  "subprotocols" text[] NULL,
  "event_type" text NOT NULL,
  "event_data" jsonb NULL,
  "started_at" timestamp NOT NULL DEFAULT now(),
  "ended_at" timestamp NULL,
  "duration_ms" integer NULL,
  "error_message" text NULL,
  "error_code" text NULL,
  "messages_sent" integer NOT NULL DEFAULT 0,
  "messages_received" integer NOT NULL DEFAULT 0,
  "audio_chunks_processed" integer NOT NULL DEFAULT 0,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_conn_logs_account" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_conn_logs_app" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "conn_logs_app_idx" to table: "connection_logs"
CREATE INDEX "conn_logs_app_idx" ON "public"."connection_logs" ("app_id", "started_at");
-- Create index "conn_logs_session_idx" to table: "connection_logs"
CREATE INDEX "conn_logs_session_idx" ON "public"."connection_logs" ("ws_session_id");
-- Create index "conn_logs_type_idx" to table: "connection_logs"
CREATE INDEX "conn_logs_type_idx" ON "public"."connection_logs" ("event_type");
-- Set comment to column: "connection_id" on table: "connection_logs"
COMMENT ON COLUMN "public"."connection_logs"."connection_id" IS 'generated connection id (string)';
-- Set comment to column: "event_type" on table: "connection_logs"
COMMENT ON COLUMN "public"."connection_logs"."event_type" IS 'connect | disconnect | error | timeout | info';
-- Create "connection_states" table
CREATE TABLE "public"."connection_states" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "connection_id" text NOT NULL,
  "ws_session_id" text NOT NULL,
  "app_id" uuid NOT NULL,
  "account_id" uuid NOT NULL,
  "llm_mode" text NULL,
  "active_session_id" uuid NULL,
  "connection_status" text NOT NULL DEFAULT 'active',
  "stt_provider" text NULL,
  "function_definitions_count" integer NOT NULL DEFAULT 0,
  "structured_schema_present" boolean NOT NULL DEFAULT false,
  "last_activity" timestamp NULL,
  "ping_latency_ms" integer NULL,
  "last_error" text NULL,
  "error_count" integer NOT NULL DEFAULT 0,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "connection_states_connection_id_unique" UNIQUE ("connection_id"),
  CONSTRAINT "fk_conn_states_account" FOREIGN KEY ("account_id") REFERENCES "public"."accounts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "fk_conn_states_active_session" FOREIGN KEY ("active_session_id") REFERENCES "public"."sessions" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "fk_conn_states_app" FOREIGN KEY ("app_id") REFERENCES "public"."apps" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "connection_states_session_idx" to table: "connection_states"
CREATE INDEX "connection_states_session_idx" ON "public"."connection_states" ("ws_session_id");
-- Set comment to column: "llm_mode" on table: "connection_states"
COMMENT ON COLUMN "public"."connection_states"."llm_mode" IS 'functions | structured | none';
