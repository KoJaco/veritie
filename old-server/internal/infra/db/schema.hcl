schema "public" {}
enum "subscription_status_enum" {
  schema = schema.public
  values = [
    "incomplete",
    "incomplete_expired",
    "trialing",
    "active",
    "past_due",
    "canceled",
    "unpaid",
    "paused"
  ]
}

enum "user_role_enum" {
  schema = schema.public
  values = ["owner", "admin", "user"]
}

enum "session_type_enum" {
  schema = schema.public
  values = ["structured_output", "functions", "enhanced_text", "markdown"]
}

enum "session_kind_enum" {
  schema = schema.public
  values = ["stream", "batch"]
}

enum "schema_parsing_strategy_enum" {
  schema = schema.public
  values = ["auto", "update-ms", "after-silence", "end-of-session"]
}

enum "notification_type_enum" {
  schema = schema.public
  values = ["info", "success", "warning", "error", "critical"]
}

enum "entity_enum" {
  schema = schema.public
  values = [
    "account",
    "users",
    "roles",
    "permissions",
    "forms",
    "subscriptions",
    "billing",
    "usage_metrics",
    "apps",
    "sessions",
    "schemas"
  ]
}

enum "action_enum" {
  schema = schema.public
  values = ["create", "retrieve", "update", "delete"]
}

enum "batch_job_status_enum" {
  schema = schema.public
  values = ["queued", "processing", "completed", "failed"]
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
    column "subscription_status" {
        type = enum.subscription_status_enum
        null = true
    }
    column "stripe_subscription_id" {
        type = text
        null = true
    }
    column "stripe_customer_id" {
        type = text
        null = true
    }
    column "trial_ends_at" {
        type = timestamp
        null = true
    }
    column "plan" {
        type = text
        null = false
        default = sql("'free'")
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
    index "stripe_customer_idx" {
        columns = [column.stripe_customer_id]
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

    column "email_verified" {
        type = boolean
        default = "false" 
    }

    column "provider" {
        type = text
        null = false
    }

    column "provider_id"{
        type = text
        null = false
    }

    column "role" {
        type = enum.user_role_enum
        null = false
    }

    column "last_login_at" {
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

    index "account_user_idx" {
        columns = [column.account_id]
    }

    index "provider_idx" {
      columns = [column.provider]
    }
    
    foreign_key "user_account_fk" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }
}

table "user_invitations" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
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

  column "role_id" {
    type = uuid
    null = false
  }

  column "token" {
    type = text
    null = false
  }

  column "invited_by" {
    type = uuid
    null = false
  }

  column "expires_at" {
    type = timestamptz
    null = false
  }

  column "status" {
    type    = text
    null    = false
    default = "pending"
    comment = "Valid values: pending, accepted, cancelled"
  }

  column "created_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  column "updated_at" {
    type    = timestamptz
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_user_invitations_account" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_user_invitations_role" {
    columns     = [column.role_id]
    ref_columns = [table.roles.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_user_invitations_invited_by" {
    columns     = [column.invited_by]
    ref_columns = [table.users.column.id]
    on_delete   = "CASCADE"
  }

  index "account_email_idx" {
    columns = [column.account_id, column.email]
  }

  unique "user_invitations_token_unique" {
    columns = [column.token]
  }

  check "user_invitations_status_check" {
    expr = "status IN ('pending','accepted','cancelled')"
  }
}

table "user_preferences" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "settings" {
    type    = jsonb
    null    = false
    default = sql("'{}'::jsonb")
    comment = "Arbitrary per-user preference map"
  }

  column "user_id" {
    type = uuid
    null = false
  }

  column "created_at" {
    type    = timestamptz
    default = sql("now()")
  }

  column "updated_at" {
    type    = timestamptz
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_user_preferences_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = "CASCADE"
  }

  unique "user_preferences_user_unique" {
    columns = [column.user_id]
  }
}

table "user_profiles" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "full_name" {
    type = text
  }

  column "avatarBlob" {
    type = text
    comment = "Arbitrary avatar blob/URL/string as per app usage"
  }

  column "bio" {
    type = text
  }

  column "user_id" {
    type = uuid
    null = false
  }

  column "created_at" {
    type    = timestamptz
    default = sql("now()")
  }

  column "updated_at" {
    type    = timestamptz
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_user_profiles_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = "CASCADE"
  }

  index "user_profiles_user_idx" {
    columns = [column.user_id]
  }
}


table "roles" {
    schema = schema.public

    column "id" {
        type    = uuid
        null    = false
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
        default = ""
    }

    column "access_level" {
        type = int
        null = false
        comment = "Hierarchy level, e.g., 100 = owner, 50 = admin"
    }

    column "created_at" {
        type = timestamp
        default = sql("now()")
        null = false
    }

    column "updated_at" {
        type = timestamp
        default = sql("now()")
        null = false
    }

    primary_key {
        columns = [column.id]
    }

    index "unique_role_name_account" {
        columns = [column.name, column.account_id]
    }

    index "roles_account_id_idx" {
        columns = [column.account_id]
    }

    foreign_key "fk_roles_account" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }
}


table "role_users" {
    schema = schema.public

    column "user_id" {
        type = uuid
        null = false
    }

    column "role_id" {
        type = uuid
        null = false
    }

    column "assigned_by" {
        type = uuid
        null = true
    }

    column "assigned_at" {
        type = timestamp
        default = sql("now()")
    }

    primary_key {
        columns = [column.user_id, column.role_id]
    }

    index "role_users_unique_idx" {
        columns = [column.user_id, column.role_id]
    }

    index "role_users_user_id_idx" {
        columns = [column.user_id]
    }

    foreign_key "fk_role_users_user" {
        columns     = [column.user_id]
        ref_columns = [table.users.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "fk_role_users_role" {
        columns     = [column.role_id]
        ref_columns = [table.roles.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "fk_role_users_assigned_by" {
        columns     = [column.assigned_by]
        ref_columns = [table.users.column.id]
        on_delete   = "SET_NULL"
    }
}


table "permissions" {
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

    column "entity" {
        type = enum.entity_enum
        null = false
    }

     column "actions" {
        type = sql("action_enum[]")
        null = false
    }

    column "description" {
        type = text
        default = ""
    }

    column "is_critical" {
        type = boolean
        default = "false"
    }

    column "is_owner_only" {
        type = boolean
        default = "false"
    }

    column "created_at" {
        type = timestamp
        default = sql("now()")
    }

    column "updated_at" {
        type = timestamp
        default = sql("now()")
    }

    primary_key {
        columns = [column.id]
    }

    index "permissions_account_id_idx" {
        columns = [column.account_id]
    }

    index "permissions_entity_action_idx" {
        columns = [column.entity, column.actions]
    }

    foreign_key "fk_permissions_account" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }
}


table "permission_roles" {
    schema = schema.public

    column "role_id" {
        type = uuid
        null = false
    }

    column "permission_id" {
        type = uuid
        null = false
    }

    column "created_at" {
        type = timestamp
        default = sql("now()")
    }

    column "updated_at" {
        type = timestamp
        default = sql("now()")
    }

    primary_key {
        columns = [column.role_id, column.permission_id]
    }

    index "permission_roles_role_idx" {
        columns = [column.role_id]
    }

    foreign_key "fk_permission_roles_role" {
        columns     = [column.role_id]
        ref_columns = [table.roles.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "fk_permission_roles_permission" {
        columns     = [column.permission_id]
        ref_columns = [table.permissions.column.id]
        on_delete   = "CASCADE"
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
    column "description" {
        type = text
        null = false
    }
    column "api_key" {
        type = text
        null = false
    }
    column "config" {
        type = json
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
    index "account_app_idx" {
        columns = [column.account_id]
    }
    foreign_key "app_account_fk" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }
}


table "sessions" {
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

    column "kind" {
        type = sql("session_kind_enum")
        default = sql("'stream'")
    }

    column "is_test" {
        type = boolean
        default = false
    }

    column "created_at" {
        type = timestamp
        null = false
        default = sql("now()")
    }
    column "closed_at" {
        type = timestamp
        null = true
    }
    primary_key {
        columns = [column.id]
    }

    foreign_key "session_app_fk" {
        columns     = [column.app_id]
        ref_columns = [table.apps.column.id]
        on_delete   = "CASCADE"
    }

    index "session_kind_idx" {
      columns = [column.kind]
    }
}

table "transcripts" {
    schema = schema.public

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }
    column "session_id" {
        type = uuid
        null = false
    }
    column "text" {
        type = text
        null = false
        comment = "The transcript text content"
    }
    column "is_final" {
        type = boolean
        null = false
        default = false
        comment = "Whether this is a final transcript or interim"
    }
    column "confidence" {
        type = float4
        null = true
        comment = "Overall confidence score for the transcript (0.0-1.0)"
    }
    column "stability" {
        type = float4
        null = true
        comment = "Stability score for streaming transcripts (0.0-1.0)"
    }
    column "chunk_dur_sec" {
        type = float8
        null = true
        comment = "Duration of the audio chunk in seconds"
    }
    column "channel" {
        type = int
        null = true
        comment = "Audio channel number for multi-channel audio"
    }
    column "words" {
        type = jsonb
        null = true
        comment = "Word-level data with timing, confidence, and speaker info"
    }
    column "turns" {
        type = jsonb
        null = true
        comment = "Speaker diarization data with turn information"
    }
    column "phrases" {
        type = jsonb
        null = true
        comment = "Phrases split by final transcript chunks"
    }

    column "created_at" {
        type = timestamp
        null = false
        default = sql("now()")
    }

    primary_key {
        columns = [column.id]
    }

    foreign_key "transcript_session_fk" {
        columns     = [column.session_id]
        ref_columns = [table.sessions.column.id]
        on_delete   = "CASCADE"
    }
    index "transcripts_session_final_idx" {
        columns = [column.session_id, column.is_final]
    }
    index "transcripts_confidence_idx" {
        columns = [column.confidence]
    }
}

table "function_calls" {
    schema = schema.public

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }
    column "session_id" {
        type = uuid
        null = false
    }
    column "name" {
        type = text
        null = false
    }
    column "args" {
        type = json
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
    foreign_key "function_session_fk" {
        columns     = [column.session_id]
        ref_columns = [table.sessions.column.id]
        on_delete   = "CASCADE"
    }
}

table "function_schemas" {
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

    column "session_id" {
        type = uuid
        null = true
        comment = "If this schema was used in a specific session"
    }

    column "name" {
        type = text
        null = true
        comment = "Optional label for the function config (client-defined)"
    }

    column "description" {
        type = text
        null = true
    }

    column "parsing_guide" {
        type = text
        null = true
        comment = "Free-text LLM instructions associated with this function config"
    }

    column "update_ms" {
        type = int
        null = true
        comment = "Update frequency in milliseconds for real-time parsing"
    }

    column "parsing_strategy" {
        type = enum.schema_parsing_strategy_enum
        null = false
        default = sql("'auto'")
    }

    column "declarations" {
        type = json
        null = false
        comment = "The complete function declarations array for this config"
    }

    column "checksum" {
        type = text
        null = false
        comment = "Hash of the entire function config for deduplication"
    }

    column "created_at" {
        type = timestamp
        default = sql("now()")
    }

    primary_key {
        columns = [column.id]
    }

    foreign_key "fk_function_schemas_app" {
        columns     = [column.app_id]
        ref_columns = [table.apps.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "fk_function_schemas_session" {
        columns     = [column.session_id]
        ref_columns = [table.sessions.column.id]
        on_delete   = "SET_NULL"
    }

    index "function_schemas_app_name_idx" {
        columns = [column.app_id, column.name]
    }

    index "function_schemas_session_idx" {
        columns = [column.session_id]
    }

    unique "unique_app_checksum" {
        columns = [column.app_id, column.checksum]
    }
}


table "session_function_schemas" {
  schema = schema.public

  column "session_id" {
    type = uuid
    null = false
  }

  column "function_schema_id" {
    type = uuid
    null = false
  }

  primary_key {
    columns = [column.session_id, column.function_schema_id]
  }

  foreign_key "fk_sfs_session" {
    columns     = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_sfs_function_schema" {
    columns     = [column.function_schema_id]
    ref_columns = [table.function_schemas.column.id]
    on_delete   = "CASCADE"
  }
}





table "usage_logs" {
    schema = schema.public

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    column "session_id" {
        type = uuid
        null = false
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
        comment = "e.g., 'stt', 'llm', 'function_call'"
    }

    column "metric" {
        type = json
        null = false
        comment = "Flexible data payload: tokens, duration, latency, cost, etc."
    }

    column "logged_at" {
        type = timestamp
        null = false
        default = sql("now()")
    }

    primary_key {
        columns = [column.id]
    }

    foreign_key "fk_usage_logs_session" {
        columns     = [column.session_id]
        ref_columns = [table.sessions.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "fk_usage_logs_app" {
        columns     = [column.app_id]
        ref_columns = [table.apps.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "fk_usage_logs_account" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }

    index "usage_logs_by_app_type_time" {
        columns = [column.app_id, column.type, column.logged_at]
    }

    index "usage_logs_by_account" {
        columns = [column.account_id]
    }
}


table "session_usage_totals" {
    schema = schema.public

    column "session_id" {
        type = uuid
        null = false
    }

    column "account_id" {
        type = uuid
        null = false
    }

    column "app_id" {
        type = uuid
        null = false
    }

    column "audio_seconds" {
        type = float8
        null = false
        default = 0
    }

    column "prompt_tokens" {
        type = int8
        null = false
        default = 0
    }

    column "completion_tokens" {
        type = int8
        null = false
        default = 0
    }

    column "saved_prompt_tokens" {
        type = int8
        null = false
        default = 0
    }

    column "cpu_active_seconds" {
        type = float8
        null = false
        default = 0
    }

    column "cpu_idle_seconds" {
        type = float8
        null = false
        default = 0
    }

    column "prompt_cost" {
        type = float8
        null = false
        default = 0
    }

    column "completion_cost" {
        type = float8
        null = false
        default = 0
    }

    column "saved_prompt_cost" {
        type = float8
        null = false
        default = 0
    }

    column "audio_cost" {
        type = float8
        null = false
        default = 0
    }

    column "cpu_cost" {
        type = float8
        null = false
        default = 0
    }

    column "total_cost" {
        type = float8
        null = false
        default = 0
    }

    column "updated_at" {
        type = timestamp
        null = false
        default = sql("now()")
    }

    primary_key {
        columns = [column.session_id]
    }

    foreign_key "session_id_fk" {
        columns     = [column.session_id]
        ref_columns = [table.sessions.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "app_id_fk" {
        columns     = [column.app_id]
        ref_columns = [table.apps.column.id]
        on_delete   = "CASCADE"
    }

    foreign_key "account_id_fk" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }

    index "app_idx" {
        columns = [column.app_id]
    }

    index "account_idx" {
        columns = [column.account_id]
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

  column "session_id" {
    type = uuid
    null = false
  }
  
  column "status" {
    type = enum.batch_job_status_enum
    null = false
    default = sql("'queued'")
  }
  
  column "file_path" {
    type = text
    null = false
  }
  
  column "file_size" {
    type = bigint
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
  
  foreign_key "batch_jobs_app_fk" {
    columns = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete = "CASCADE"
  }
  
  foreign_key "batch_jobs_account_fk" {
    columns = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete = "CASCADE"
  }

  foreign_key "batch_jobs_session_fk" {
    columns = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete = "CASCADE"
  }
  
  index "batch_jobs_status_idx" {
    columns = [column.status]
  }
  
  index "batch_jobs_app_idx" {
    columns = [column.app_id]
  }
  
  index "batch_jobs_created_at_idx" {
    columns = [column.created_at]
  }
}

table "draft_function_aggs" {
  schema = schema.public

  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "session_id" {
    type = uuid
    null = false
  }

  column "app_id" {
    type = uuid
    null = false
  }

  column "account_id" {
    type = uuid
    null = false
  }

  column "function_name" {
    type = text
    null = false
  }

  column "total_detections" {
    type = bigint
    null = false
    default = 0
  }

  column "highest_score" {
    type = float8
    null = false
    default = 0.0
  }

  column "avg_score" {
    type = float8
    null = false
    default = 0.0
  }

  column "first_detected" {
    type = timestamp
    null = false
    default = sql("now()")
  }

  column "last_detected" {
    type = timestamp
    null = false
    default = sql("now()")
  }

  column "sample_args" {
    type = jsonb
    null = true
  }

  column "version_count" {
    type = bigint
    null = false
    default = 1
  }

  column "final_call_count" {
    type = bigint
    null = false
    default = 0
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

  unique "unique_session_function" {
    columns = [column.session_id, column.function_name]
  }

  foreign_key "draft_aggs_session_fk" {
    columns = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete = "CASCADE"
  }

  foreign_key "draft_aggs_app_fk" {
    columns = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete = "CASCADE"
  }

  foreign_key "draft_aggs_account_fk" {
    columns = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete = "CASCADE"
  }

  index "draft_aggs_session_idx" {
    columns = [column.session_id]
  }

  index "draft_aggs_app_function_idx" {
    columns = [column.app_id, column.function_name]
  }
}

table "draft_function_stats" {
  schema = schema.public

  column "session_id" {
    type = uuid
    null = false
  }

  column "app_id" {
    type = uuid
    null = false
  }

  column "account_id" {
    type = uuid
    null = false
  }

  column "total_draft_functions" {
    type = bigint
    null = false
    default = 0
  }

  column "total_final_functions" {
    type = bigint
    null = false
    default = 0
  }

  column "draft_to_final_ratio" {
    type = float8
    null = false
    default = 0.0
  }

  column "unique_functions" {
    type = bigint
    null = false
    default = 0
  }

  column "avg_detection_latency" {
    type = float8
    null = false
    default = 0.0
  }

  column "top_function" {
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
    columns = [column.session_id]
  }

  foreign_key "draft_stats_session_fk" {
    columns = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete = "CASCADE"
  }

  foreign_key "draft_stats_app_fk" {
    columns = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete = "CASCADE"
  }

  foreign_key "draft_stats_account_fk" {
    columns = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete = "CASCADE"
  }
}


table "structured_output_schemas" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "app_id" {
    type = uuid
    null = false
  }

  column "session_id" {
    type    = uuid
    null    = true
    comment = "If this schema was first seen/used in a specific session"
  }

  column "name" {
    type = text
    null = true
    comment = "Optional label for the schema (client-defined)"
  }

  column "description" {
    type = text
    null = true
  }

  column "parsing_guide" {
    type    = text
    null    = true
    comment = "Free-text LLM instructions associated with this schema"
  }

  column "update_ms" {
    type    = int
    null    = true
    comment = "Update frequency in milliseconds for real-time parsing"
  }

  column "parsing_strategy" {
    type = enum.schema_parsing_strategy_enum
    null = false
    default = sql("'auto'")
  }

  column "schema" {
    type    = json
    null    = false
    comment = "The JSON Schema object for structured output"
  }

  column "checksum" {
    type    = text
    null    = false
    comment = "Hash of the schema field for deduplication (e.g., SHA-256)"
  }

  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_so_schemas_app" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_so_schemas_session" {
    columns     = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete   = "SET_NULL"
  }

  index "so_schemas_app_name_idx" {
    columns = [column.app_id, column.name]
  }

  index "so_schemas_session_idx" {
    columns = [column.session_id]
  }

  unique "unique_so_app_checksum" {
    columns = [column.app_id, column.checksum]
  }
}

table "session_structured_output_schemas" {
  schema = schema.public

  column "session_id" {
    type = uuid
    null = false
  }

  column "structured_output_schema_id" {
    type = uuid
    null = false
  }

  primary_key {
    columns = [column.session_id, column.structured_output_schema_id]
  }

  foreign_key "fk_ssos_session" {
    columns     = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_ssos_schema" {
    columns     = [column.structured_output_schema_id]
    ref_columns = [table.structured_output_schemas.column.id]
    on_delete   = "CASCADE"
  }
}

table "structured_outputs" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "session_id" {
    type = uuid
    null = false
  }


  column "structured_output_schema_id" {
    type = uuid
    null = false
    comment = "FK to the deduped schema used to produce this output"
  }

  column "output" {
    type    = json
    null    = false
    comment = "Finalized structured output JSON object"
  }

  column "is_final" {
    type    = bool
    null    = false
    default = true
    comment = "Reserved for future use if you store interim versions"
  }

  column "finalized_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_so_session" {
    columns     = [column.session_id]
    ref_columns = [table.sessions.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_so_schema" {
    columns     = [column.structured_output_schema_id]
    ref_columns = [table.structured_output_schemas.column.id]
    on_delete   = "RESTRICT"
  }

  index "so_session_idx" {
    columns = [column.session_id]
  }

  index "so_schema_idx" {
    columns = [column.structured_output_schema_id]
  }
}


table "connection_logs" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "connection_id" {
    type    = text
    null    = false
    comment = "generated connection id (string)"
  }

  column "ws_session_id" {
    type = text
    null = false
  }

  column "app_id" {
    type = uuid
    null = false
  }

  column "account_id" {
    type = uuid
    null = false
  }

  column "remote_addr" {
    type = inet
    null = true
  }

  column "user_agent" {
    type = text
    null = true
  }

  column "subprotocols" {
    type = sql("text[]")
    null = true
  }

  column "event_type" {
    type = text
    null = false
    comment = "connect | disconnect | error | timeout | info"
  }

  column "event_data" {
    type = jsonb
    null = true
  }

  column "started_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  column "ended_at" {
    type = timestamp
    null = true
  }

  column "duration_ms" {
    type = int
    null = true
  }

  column "error_message" {
    type = text
    null = true
  }

  column "error_code" {
    type = text
    null = true
  }

  column "messages_sent" {
    type    = int
    null    = false
    default = 0
  }

  column "messages_received" {
    type    = int
    null    = false
    default = 0
  }

  column "audio_chunks_processed" {
    type    = int
    null    = false
    default = 0
  }

  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  index "conn_logs_app_idx" {
    columns = [column.app_id, column.started_at]
  }

  index "conn_logs_session_idx" {
    columns = [column.ws_session_id]
  }

  index "conn_logs_type_idx" {
    columns = [column.event_type]
  }

  foreign_key "fk_conn_logs_app" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_conn_logs_account" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = "CASCADE"
  }
}

table "connection_states" {
  schema = schema.public

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "connection_id" {
    type = text
    null = false
  }

  column "ws_session_id" {
    type = text
    null = false
  }

  column "app_id" {
    type = uuid
    null = false
  }

  column "account_id" {
    type = uuid
    null = false
  }

  column "llm_mode" {
    type = text
    null = true
    comment = "functions | structured | none"
  }

  column "active_session_id" {
    type = uuid
    null = true
  }

  column "connection_status" {
    type = text
    null = false
    default = sql("'active'")
  }

  column "stt_provider" {
    type = text
    null = true
  }

  column "function_definitions_count" {
    type    = int
    null    = false
    default = 0
  }

  column "structured_schema_present" {
    type    = boolean
    null    = false
    default = false
  }

  column "last_activity" {
    type = timestamp
    null = true
  }

  column "ping_latency_ms" {
    type = int
    null = true
  }

  column "last_error" {
    type = text
    null = true
  }

  column "error_count" {
    type    = int
    null    = false
    default = 0
  }

  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  column "updated_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }

  primary_key {
    columns = [column.id]
  }

  unique "connection_states_connection_id_unique" {
    columns = [column.connection_id]
  }

  index "connection_states_session_idx" {
    columns = [column.ws_session_id]
  }

  foreign_key "fk_conn_states_app" {
    columns     = [column.app_id]
    ref_columns = [table.apps.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_conn_states_account" {
    columns     = [column.account_id]
    ref_columns = [table.accounts.column.id]
    on_delete   = "CASCADE"
  }

  foreign_key "fk_conn_states_active_session" {
    columns     = [column.active_session_id]
    ref_columns = [table.sessions.column.id]
    on_delete   = "SET_NULL"
  }
}