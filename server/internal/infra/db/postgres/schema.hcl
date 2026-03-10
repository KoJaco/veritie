enum "schema_version_status_enum" {
  schema = schema.public
  values = ["draft", "active", "deprecated"]
}

enum "tool_version_status_enum" {
  schema = schema.public
  values = ["draft", "active", "deprecated"]
}

enum "toolset_version_status_enum" {
  schema = schema.public
  values = ["draft", "active", "deprecated"]
}

enum "job_status_enum" {
  schema = schema.public
  values = ["queued", "running", "succeeded", "failed", "cancelled"]
}

enum "job_step_status_enum" {
  schema = schema.public
  values = ["queued", "running", "succeeded", "failed", "cancelled", "skipped"]
}

enum "media_asset_kind_enum" {
  schema = schema.public
  values = ["source_audio", "normalized_audio", "preview", "thumbnail", "waveform"]
}

enum "transcript_provider_enum" {
  schema = schema.public
  values = ["deepgram", "speechmatics", "manual"]
}

enum "transcript_status_enum" {
  schema = schema.public
  values = ["queued", "running", "succeeded", "failed"]
}

enum "extraction_run_status_enum" {
  schema = schema.public
  values = ["queued", "running", "succeeded", "failed"]
}

enum "tool_suggestion_run_status_enum" {
  schema = schema.public
  values = ["queued", "running", "succeeded", "failed"]
}

enum "tool_suggestion_status_enum" {
  schema = schema.public
  values = ["pending", "suggested", "accepted", "rejected", "executed", "failed"]
}

enum "citation_target_type_enum" {
  schema = schema.public
  values = ["extracted_item", "tool_suggestion"]
}

enum "job_event_type_enum" {
  schema = schema.public
  values = [
    "accepted",
    "ingest_started",
    "ingest_completed",
    "transcription_started",
    "transcription_completed",
    "extraction_started",
    "extraction_completed",
    "tool_suggestion_started",
    "tool_suggestion_completed",
    "completed",
    "failed",
    "cancelled",
    "progress",
    "stt_started",
  ]
}

enum "job_event_level_enum" {
  schema = schema.public
  values = ["debug", "info", "warn", "error"]
}

enum "webhook_event_type_enum" {
  schema = schema.public
  values = ["job_progress", "job_completed", "job_failed"]
}

enum "webhook_delivery_status_enum" {
  schema = schema.public
  values = ["pending", "retrying", "succeeded", "failed"]
}

enum "usage_event_type_enum" {
  schema = schema.public
  values = ["stt_audio_ms", "llm_tokens", "job_processed", "webhook_attempt"]
}

table "accounts" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "name" {
    type = text
    null = false
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
}

table "users" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "account_id" {
    type = uuid
    null = false
  }
  column "email" {
    type = text
    null = false
  }
  column "display_name" {
    type = text
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "users_account_email_unique_idx" {
    unique  = true
    columns = [column.account_id, column.email]
  }
  foreign_key "users_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
}

table "apps" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "account_id" {
    type = uuid
    null = false
  }
  column "name" {
    type = text
    null = false
  }
  column "schema_id" {
    type = uuid
    null = false
  }
  column "active_schema_version_id" {
    type = uuid
    null = false
  }
  column "active_toolset_version_id" {
    type = uuid
    null = false
  }
  column "processing_config" {
    type = jsonb
    null = false
    default = sql("'{}'::jsonb")
  }
  column "runtime_behavior" {
    type = jsonb
    null = false
    default = sql("'{}'::jsonb")
  }
  column "llm_config" {
    type = jsonb
    null = false
    default = sql("'{}'::jsonb")
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "apps_account_name_unique_idx" {
    unique  = true
    columns = [column.account_id, column.name]
  }
  index "apps_id_account_unique_idx" {
    unique  = true
    columns = [column.id, column.account_id]
  }
  foreign_key "apps_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
  foreign_key "apps_schema_account_fk" {
    columns     = [column.schema_id, column.account_id]
    ref_columns = [table.schemas.column.id, table.schemas.column.account_id]
    on_delete   = RESTRICT
  }
  foreign_key "apps_active_schema_version_fk" {
    columns     = [column.active_schema_version_id, column.schema_id]
    ref_columns = [table.schema_versions.column.id, table.schema_versions.column.schema_id]
    on_delete   = RESTRICT
  }
  foreign_key "apps_active_toolset_version_fk" {
    columns     = [column.active_toolset_version_id, column.id]
    ref_columns = [table.toolset_versions.column.id, table.toolset_versions.column.app_id]
    on_delete   = RESTRICT
  }
  index "apps_schema_id_account_id_idx" {
    columns = [column.schema_id, column.account_id]
  }
  index "apps_active_schema_version_id_idx" {
    columns = [column.active_schema_version_id]
  }
  index "apps_active_toolset_version_id_idx" {
    columns = [column.active_toolset_version_id]
  }
}

table "api_keys" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "account_id" {
    type = uuid
    null = false
  }
  column "name" {
    type = text
    null = false
  }
  column "key_hash" {
    type = text
    null = false
  }
  column "key_prefix" {
    type = text
    null = false
  }
  column "last_used_at" {
    type = timestamp
    null = true
  }
  column "expires_at" {
    type = timestamp
    null = true
  }
  column "revoked_at" {
    type = timestamp
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "api_keys_hash_unique_idx" {
    unique  = true
    columns = [column.key_hash]
  }
  index "api_keys_prefix_unique_idx" {
    unique  = true
    columns = [column.key_prefix]
  }
  index "api_keys_app_active_idx" {
    columns = [column.app_id, column.revoked_at]
  }
  index "api_keys_app_account_idx" {
    columns = [column.app_id, column.account_id]
  }
  index "api_keys_account_id_idx" {
    columns = [column.account_id]
  }
  foreign_key "api_keys_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
  foreign_key "api_keys_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
  foreign_key "api_keys_app_account_fk" {
    columns     = [column.app_id, column.account_id]
    ref_columns = [table.apps.column.id, table.apps.column.account_id]
    on_delete   = CASCADE
  }
}

table "schemas" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "account_id" {
    type = uuid
    null = false
  }
  column "name" {
    type = text
    null = false
  }
  column "description" {
    type = text
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "schemas_account_name_unique_idx" {
    unique  = true
    columns = [column.account_id, column.name]
  }
  index "schemas_id_account_unique_idx" {
    unique  = true
    columns = [column.id, column.account_id]
  }
  foreign_key "schemas_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
}

table "schema_versions" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "schema_id" {
    type = uuid
    null = false
  }
  column "version" {
    type = int
    null = false
  }
  column "status" {
    type = enum.schema_version_status_enum
    null = false
  }
  column "definition" {
    type = jsonb
    null = false
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "schema_versions_schema_version_unique_idx" {
    unique  = true
    columns = [column.schema_id, column.version]
  }
  index "schema_versions_id_schema_unique_idx" {
    unique  = true
    columns = [column.id, column.schema_id]
  }
  foreign_key "schema_versions_schema_fk" {
    columns     = [column.schema_id]
    ref_columns = [table.schemas.column.id]
    on_delete   = CASCADE
  }
}

table "tools" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "name" {
    type = text
    null = false
  }
  column "description" {
    type = text
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "tools_app_name_unique_idx" {
    unique  = true
    columns = [column.app_id, column.name]
  }
  foreign_key "tools_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
}

table "tool_versions" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "tool_id" {
    type = uuid
    null = false
  }
  column "version" {
    type = int
    null = false
  }
  column "status" {
    type = enum.tool_version_status_enum
    null = false
  }
  column "definition" {
    type = jsonb
    null = false
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "tool_versions_tool_version_unique_idx" {
    unique  = true
    columns = [column.tool_id, column.version]
  }
  foreign_key "tool_versions_tool_fk" {
    columns     = [column.tool_id]
    ref_columns = [table.tools.column.id]
    on_delete   = CASCADE
  }
}

table "toolsets" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "name" {
    type = text
    null = false
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "toolsets_app_unique_idx" {
    unique  = true
    columns = [column.app_id]
  }
  index "toolsets_id_app_unique_idx" {
    unique  = true
    columns = [column.id, column.app_id]
  }
  foreign_key "toolsets_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
}

table "toolset_versions" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "toolset_id" {
    type = uuid
    null = false
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "version" {
    type = int
    null = false
  }
  column "status" {
    type = enum.toolset_version_status_enum
    null = false
  }
  column "definition" {
    type = jsonb
    null = false
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "toolset_versions_toolset_version_unique_idx" {
    unique  = true
    columns = [column.toolset_id, column.version]
  }
  index "toolset_versions_id_app_unique_idx" {
    unique  = true
    columns = [column.id, column.app_id]
  }
  index "toolset_versions_id_toolset_unique_idx" {
    unique  = true
    columns = [column.id, column.toolset_id]
  }
  foreign_key "toolset_versions_toolset_app_fk" {
    columns     = [column.toolset_id, column.app_id]
    ref_columns = [table.toolsets.column.id, table.toolsets.column.app_id]
    on_delete   = CASCADE
  }
}

table "jobs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "account_id" {
    type = uuid
    null = false
  }
  column "schema_id" {
    type = uuid
    null = false
  }
  column "schema_version_id" {
    type = uuid
    null = false
  }
  column "toolset_version_id" {
    type = uuid
    null = false
  }
  column "status" {
    type = enum.job_status_enum
    null = false
  }
  column "idempotency_key" {
    type = text
    null = true
  }
  column "rerun_of_job_id" {
    type = uuid
    null = true
  }
  column "audio_uri" {
    type = text
    null = false
  }
  column "audio_size" {
    type = bigint
    null = false
  }
  column "audio_duration_ms" {
    type = bigint
    null = false
  }
  column "audio_content_type" {
    type = text
    null = false
  }
  column "config_snapshot" {
    type = jsonb
    null = false
  }
  column "llm_config" {
    type = jsonb
    null = true
  }
  column "error_message" {
    type = text
    null = true
  }
  column "started_at" {
    type = timestamp
    null = true
  }
  column "completed_at" {
    type = timestamp
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "jobs_app_created_at_idx" {
    columns = [column.app_id, column.created_at]
  }
  index "jobs_status_idx" {
    columns = [column.status]
  }
  index "jobs_created_at_idx" {
    columns = [column.created_at]
  }
  index "jobs_rerun_of_job_id_idx" {
    columns = [column.rerun_of_job_id]
  }
  index "jobs_app_idempotency_unique_idx" {
    unique  = true
    columns = [column.app_id, column.idempotency_key]
    where   = "idempotency_key IS NOT NULL"
  }
  index "jobs_app_account_created_id_idx" {
    columns = [column.app_id, column.account_id, column.created_at, column.id]
  }
  index "jobs_schema_version_schema_idx" {
    columns = [column.schema_version_id, column.schema_id]
  }
  index "jobs_schema_account_idx" {
    columns = [column.schema_id, column.account_id]
  }
  foreign_key "jobs_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
  foreign_key "jobs_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
  foreign_key "jobs_app_account_fk" {
    columns     = [column.app_id, column.account_id]
    ref_columns = [table.apps.column.id, table.apps.column.account_id]
    on_delete   = CASCADE
  }
  foreign_key "jobs_schema_version_fk" {
    columns     = [column.schema_version_id]
    ref_columns = [table.schema_versions.column.id]
    on_delete   = RESTRICT
  }
  foreign_key "jobs_schema_version_schema_fk" {
    columns     = [column.schema_version_id, column.schema_id]
    ref_columns = [table.schema_versions.column.id, table.schema_versions.column.schema_id]
    on_delete   = RESTRICT
  }
  foreign_key "jobs_schema_account_fk" {
    columns     = [column.schema_id, column.account_id]
    ref_columns = [table.schemas.column.id, table.schemas.column.account_id]
    on_delete   = RESTRICT
  }
  foreign_key "jobs_toolset_version_app_fk" {
    columns     = [column.toolset_version_id, column.app_id]
    ref_columns = [table.toolset_versions.column.id, table.toolset_versions.column.app_id]
    on_delete   = RESTRICT
  }
  foreign_key "jobs_rerun_of_job_fk" {
    columns     = [column.rerun_of_job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = SET_NULL
  }
}

table "job_steps" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "job_id" {
    type = uuid
    null = false
  }
  column "step" {
    type = text
    null = false
  }
  column "status" {
    type = enum.job_step_status_enum
    null = false
  }
  column "message" {
    type = text
    null = true
  }
  column "error_message" {
    type = text
    null = true
  }
  column "started_at" {
    type = timestamp
    null = true
  }
  column "completed_at" {
    type = timestamp
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "job_steps_job_step_unique_idx" {
    unique  = true
    columns = [column.job_id, column.step]
  }
  foreign_key "job_steps_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = CASCADE
  }
}

table "media_assets" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "job_id" {
    type = uuid
    null = false
  }
  column "kind" {
    type = enum.media_asset_kind_enum
    null = false
  }
  column "uri" {
    type = text
    null = false
  }
  column "content_type" {
    type = text
    null = false
  }
  column "size_bytes" {
    type = bigint
    null = true
  }
  column "duration_ms" {
    type = bigint
    null = true
  }
  column "metadata" {
    type = jsonb
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "media_assets_job_idx" {
    columns = [column.job_id]
  }
  foreign_key "media_assets_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = CASCADE
  }
}

table "transcripts" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "job_id" {
    type = uuid
    null = false
  }
  column "provider" {
    type = enum.transcript_provider_enum
    null = false
  }
  column "status" {
    type = enum.transcript_status_enum
    null = false
  }
  column "language_code" {
    type = text
    null = true
  }
  column "full_text" {
    type = text
    null = true
  }
  column "metadata" {
    type = jsonb
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "transcripts_job_idx" {
    columns = [column.job_id]
  }
  foreign_key "transcripts_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = CASCADE
  }
}

table "transcript_segments" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "transcript_id" {
    type = uuid
    null = false
  }
  column "segment_index" {
    type = bigint
    null = false
  }
  column "speaker_label" {
    type = text
    null = true
  }
  column "start_ms" {
    type = bigint
    null = false
  }
  column "end_ms" {
    type = bigint
    null = false
  }
  column "text" {
    type = text
    null = false
  }
  column "confidence" {
    type = float8
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "transcript_segments_transcript_order_unique_idx" {
    unique  = true
    columns = [column.transcript_id, column.segment_index]
  }
  foreign_key "transcript_segments_transcript_fk" {
    columns     = [column.transcript_id]
    ref_columns = [table.transcripts.column.id]
    on_delete   = CASCADE
  }
}

table "extraction_runs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "job_id" {
    type = uuid
    null = false
  }
  column "schema_version_id" {
    type = uuid
    null = false
  }
  column "status" {
    type = enum.extraction_run_status_enum
    null = false
  }
  column "error_message" {
    type = text
    null = true
  }
  column "started_at" {
    type = timestamp
    null = true
  }
  column "completed_at" {
    type = timestamp
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "extraction_runs_job_idx" {
    columns = [column.job_id]
  }
  foreign_key "extraction_runs_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = CASCADE
  }
  foreign_key "extraction_runs_schema_version_fk" {
    columns     = [column.schema_version_id]
    ref_columns = [table.schema_versions.column.id]
    on_delete   = RESTRICT
  }
}

table "extracted_items" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "extraction_run_id" {
    type = uuid
    null = false
  }
  column "item_key" {
    type = text
    null = false
  }
  column "item_type" {
    type = text
    null = false
  }
  column "payload" {
    type = jsonb
    null = false
  }
  column "confidence" {
    type = float8
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "extracted_items_extraction_run_idx" {
    columns = [column.extraction_run_id]
  }
  foreign_key "extracted_items_extraction_run_fk" {
    columns     = [column.extraction_run_id]
    ref_columns = [table.extraction_runs.column.id]
    on_delete   = CASCADE
  }
}

table "tool_suggestion_runs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "job_id" {
    type = uuid
    null = false
  }
  column "status" {
    type = enum.tool_suggestion_run_status_enum
    null = false
  }
  column "error_message" {
    type = text
    null = true
  }
  column "started_at" {
    type = timestamp
    null = true
  }
  column "completed_at" {
    type = timestamp
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "tool_suggestion_runs_job_idx" {
    columns = [column.job_id]
  }
  foreign_key "tool_suggestion_runs_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = CASCADE
  }
}

table "tool_suggestions" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "tool_suggestion_run_id" {
    type = uuid
    null = false
  }
  column "tool_version_id" {
    type = uuid
    null = true
  }
  column "status" {
    type = enum.tool_suggestion_status_enum
    null = false
  }
  column "title" {
    type = text
    null = true
  }
  column "payload" {
    type = jsonb
    null = false
  }
  column "confidence" {
    type = float8
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "tool_suggestions_run_idx" {
    columns = [column.tool_suggestion_run_id]
  }
  foreign_key "tool_suggestions_run_fk" {
    columns     = [column.tool_suggestion_run_id]
    ref_columns = [table.tool_suggestion_runs.column.id]
    on_delete   = CASCADE
  }
  foreign_key "tool_suggestions_tool_version_fk" {
    columns     = [column.tool_version_id]
    ref_columns = [table.tool_versions.column.id]
    on_delete   = SET_NULL
  }
}

table "citations" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "transcript_segment_id" {
    type = uuid
    null = false
  }
  column "target_type" {
    type = enum.citation_target_type_enum
    null = false
  }
  column "target_id" {
    type = uuid
    null = false
  }
  column "start_ms" {
    type = bigint
    null = true
  }
  column "end_ms" {
    type = bigint
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "citations_target_idx" {
    columns = [column.target_type, column.target_id]
  }
  foreign_key "citations_transcript_segment_fk" {
    columns     = [column.transcript_segment_id]
    ref_columns = [table.transcript_segments.column.id]
    on_delete   = CASCADE
  }
}

table "job_events" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "job_id" {
    type = uuid
    null = false
  }
  column "type" {
    type = enum.job_event_type_enum
    null = false
  }
  column "message" {
    type = text
    null = false
  }
  column "level" {
    type = enum.job_event_level_enum
    null = false
    default = sql("'info'::job_event_level_enum")
  }
  column "progress" {
    type = float8
    null = false
    default = 0
  }
  column "data" {
    type = jsonb
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "job_events_job_created_idx" {
    columns = [column.job_id, column.created_at, column.id]
  }
  foreign_key "job_events_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = CASCADE
  }
}

table "webhooks" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "url" {
    type = text
    null = false
  }
  column "secret_hash" {
    type = text
    null = false
  }
  column "enabled" {
    type = boolean
    null = false
    default = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "webhooks_app_idx" {
    columns = [column.app_id]
  }
  foreign_key "webhooks_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
}

table "webhook_deliveries" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "webhook_id" {
    type = uuid
    null = false
  }
  column "job_id" {
    type = uuid
    null = true
  }
  column "event_type" {
    type = enum.webhook_event_type_enum
    null = false
  }
  column "status" {
    type = enum.webhook_delivery_status_enum
    null = false
  }
  column "response_code" {
    type = int
    null = true
  }
  column "error_message" {
    type = text
    null = true
  }
  column "attempted_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  column "delivered_at" {
    type = timestamp
    null = true
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "webhook_deliveries_webhook_idx" {
    columns = [column.webhook_id]
  }
  foreign_key "webhook_deliveries_webhook_fk" {
    columns     = [column.webhook_id]
    ref_columns = [table.webhooks.column.id]
    on_delete   = CASCADE
  }
  foreign_key "webhook_deliveries_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = SET_NULL
  }
}

table "usage_events" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "app_id" {
    type = uuid
    null = false
  }
  column "account_id" {
    type = uuid
    null = false
  }
  column "job_id" {
    type = uuid
    null = true
  }
  column "type" {
    type = enum.usage_event_type_enum
    null = false
  }
  column "metric" {
    type = jsonb
    null = false
  }
  column "logged_at" {
    type = timestamp
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "usage_events_app_logged_idx" {
    columns = [column.app_id, column.logged_at]
  }
  foreign_key "usage_events_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
  foreign_key "usage_events_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
  foreign_key "usage_events_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.jobs.column.id]
    on_delete   = SET_NULL
  }
}
