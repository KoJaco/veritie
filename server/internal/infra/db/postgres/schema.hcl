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
  column "api_key" {
    type = text
    null = false
  }
  column "config" {
    type = jsonb
    null = true
  }
  column "llm_config" {
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
  index "apps_api_key_key" {
    unique  = true
    columns = [column.api_key]
  }
  foreign_key "apps_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = CASCADE
  }
}

table "batch_jobs" {
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
  column "status" {
    type = text
    null = false
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
  index "batch_jobs_app_idx" {
    columns = [column.app_id]
  }
  index "batch_jobs_status_idx" {
    columns = [column.status]
  }
  index "batch_jobs_created_at_idx" {
    columns = [column.created_at]
  }
  foreign_key "batch_jobs_app_fk" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = CASCADE
  }
  foreign_key "batch_jobs_account_fk" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
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
    type = text
    null = false
  }
  column "message" {
    type = text
    null = false
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
  index "job_events_job_idx" {
    columns = [column.job_id]
  }
  index "job_events_created_at_idx" {
    columns = [column.created_at]
  }
  foreign_key "job_events_job_fk" {
    columns     = [column.job_id]
    ref_columns = [table.batch_jobs.column.id]
    on_delete   = CASCADE
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
  column "type" {
    type = text
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
}
