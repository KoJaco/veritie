CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    display_name TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_account_email_unique_idx ON users(account_id, email);

CREATE TABLE schemas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX schemas_account_name_unique_idx ON schemas(account_id, name);
CREATE UNIQUE INDEX schemas_id_account_unique_idx ON schemas(id, account_id);

CREATE TABLE schema_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_id UUID NOT NULL REFERENCES schemas(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status TEXT NOT NULL,
    definition JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX schema_versions_schema_version_unique_idx ON schema_versions(schema_id, version);
CREATE UNIQUE INDEX schema_versions_id_schema_unique_idx ON schema_versions(id, schema_id);

CREATE TABLE apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    schema_id UUID NOT NULL,
    active_schema_version_id UUID NOT NULL,
    active_toolset_version_id UUID,
    processing_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    runtime_behavior JSONB NOT NULL DEFAULT '{}'::jsonb,
    llm_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT apps_schema_account_fk FOREIGN KEY (schema_id, account_id)
        REFERENCES schemas(id, account_id) ON DELETE RESTRICT,
    CONSTRAINT apps_active_schema_version_fk FOREIGN KEY (active_schema_version_id, schema_id)
        REFERENCES schema_versions(id, schema_id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX apps_account_name_unique_idx ON apps(account_id, name);
CREATE UNIQUE INDEX apps_id_account_unique_idx ON apps(id, account_id);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX api_keys_hash_unique_idx ON api_keys(key_hash);
CREATE UNIQUE INDEX api_keys_prefix_unique_idx ON api_keys(key_prefix);
CREATE INDEX api_keys_app_active_idx ON api_keys(app_id, revoked_at);

CREATE TABLE tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tools_app_name_unique_idx ON tools(app_id, name);

CREATE TABLE tool_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status TEXT NOT NULL,
    definition JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tool_versions_tool_version_unique_idx ON tool_versions(tool_id, version);

CREATE TABLE toolsets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX toolsets_app_unique_idx ON toolsets(app_id);
CREATE UNIQUE INDEX toolsets_id_app_unique_idx ON toolsets(id, app_id);

CREATE TABLE toolset_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    toolset_id UUID NOT NULL,
    app_id UUID NOT NULL,
    version INTEGER NOT NULL,
    status TEXT NOT NULL,
    definition JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT toolset_versions_toolset_app_fk FOREIGN KEY (toolset_id, app_id)
        REFERENCES toolsets(id, app_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX toolset_versions_toolset_version_unique_idx ON toolset_versions(toolset_id, version);
CREATE UNIQUE INDEX toolset_versions_id_app_unique_idx ON toolset_versions(id, app_id);
CREATE UNIQUE INDEX toolset_versions_id_toolset_unique_idx ON toolset_versions(id, toolset_id);

ALTER TABLE apps
  ADD CONSTRAINT apps_active_toolset_version_fk
  FOREIGN KEY (active_toolset_version_id, id)
  REFERENCES toolset_versions(id, app_id)
  ON DELETE RESTRICT;

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    schema_version_id UUID NOT NULL REFERENCES schema_versions(id) ON DELETE RESTRICT,
    toolset_version_id UUID NOT NULL,
    status TEXT NOT NULL,
    idempotency_key TEXT,
    rerun_of_job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    audio_uri TEXT NOT NULL,
    audio_size BIGINT NOT NULL,
    audio_duration_ms BIGINT NOT NULL,
    audio_content_type TEXT NOT NULL,
    config_snapshot JSONB NOT NULL,
    llm_config JSONB,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT jobs_app_account_fk FOREIGN KEY (app_id, account_id)
        REFERENCES apps(id, account_id) ON DELETE CASCADE,
    CONSTRAINT jobs_toolset_version_app_fk FOREIGN KEY (toolset_version_id, app_id)
        REFERENCES toolset_versions(id, app_id) ON DELETE RESTRICT
);
CREATE INDEX jobs_app_created_at_idx ON jobs(app_id, created_at);
CREATE INDEX jobs_status_idx ON jobs(status);
CREATE INDEX jobs_created_at_idx ON jobs(created_at);
CREATE INDEX jobs_rerun_of_job_id_idx ON jobs(rerun_of_job_id);
CREATE UNIQUE INDEX jobs_app_idempotency_unique_idx ON jobs(app_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE job_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    step TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX job_steps_job_step_unique_idx ON job_steps(job_id, step);

CREATE TABLE media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    uri TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT,
    duration_ms BIGINT,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX media_assets_job_idx ON media_assets(job_id);

CREATE TABLE transcripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    language_code TEXT,
    full_text TEXT,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX transcripts_job_idx ON transcripts(job_id);

CREATE TABLE transcript_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transcript_id UUID NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
    segment_index BIGINT NOT NULL,
    speaker_label TEXT,
    start_ms BIGINT NOT NULL,
    end_ms BIGINT NOT NULL,
    text TEXT NOT NULL,
    confidence DOUBLE PRECISION,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX transcript_segments_transcript_order_unique_idx ON transcript_segments(transcript_id, segment_index);

CREATE TABLE extraction_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    schema_version_id UUID NOT NULL REFERENCES schema_versions(id) ON DELETE RESTRICT,
    status TEXT NOT NULL,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX extraction_runs_job_idx ON extraction_runs(job_id);

CREATE TABLE extracted_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    extraction_run_id UUID NOT NULL REFERENCES extraction_runs(id) ON DELETE CASCADE,
    item_key TEXT NOT NULL,
    item_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    confidence DOUBLE PRECISION,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX extracted_items_extraction_run_idx ON extracted_items(extraction_run_id);

CREATE TABLE tool_suggestion_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX tool_suggestion_runs_job_idx ON tool_suggestion_runs(job_id);

CREATE TABLE tool_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_suggestion_run_id UUID NOT NULL REFERENCES tool_suggestion_runs(id) ON DELETE CASCADE,
    tool_version_id UUID REFERENCES tool_versions(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    title TEXT,
    payload JSONB NOT NULL,
    confidence DOUBLE PRECISION,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX tool_suggestions_run_idx ON tool_suggestions(tool_suggestion_run_id);

CREATE TABLE citations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transcript_segment_id UUID NOT NULL REFERENCES transcript_segments(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    start_ms BIGINT,
    end_ms BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX citations_target_idx ON citations(target_type, target_id);

CREATE TABLE job_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    message TEXT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    progress DOUBLE PRECISION NOT NULL DEFAULT 0,
    data JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX job_events_job_created_idx ON job_events(job_id, created_at, id);

CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret_hash TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX webhooks_app_idx ON webhooks(app_id);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    response_code INTEGER,
    error_message TEXT,
    attempted_at TIMESTAMP NOT NULL DEFAULT now(),
    delivered_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX webhook_deliveries_webhook_idx ON webhook_deliveries(webhook_id);

CREATE TABLE usage_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    metric JSONB NOT NULL,
    logged_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX usage_events_app_logged_idx ON usage_events(app_id, logged_at);
