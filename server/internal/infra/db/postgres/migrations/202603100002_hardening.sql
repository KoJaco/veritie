-- 1) Cross-tenant integrity and additional relational constraints

ALTER TABLE api_keys
  ADD CONSTRAINT api_keys_app_account_fk
  FOREIGN KEY (app_id, account_id)
  REFERENCES apps(id, account_id)
  ON DELETE CASCADE;

ALTER TABLE jobs
  ADD COLUMN schema_id UUID;

UPDATE jobs j
SET schema_id = a.schema_id
FROM apps a
WHERE a.id = j.app_id;

ALTER TABLE jobs
  ALTER COLUMN schema_id SET NOT NULL;

ALTER TABLE jobs
  ADD CONSTRAINT jobs_schema_version_schema_fk
  FOREIGN KEY (schema_version_id, schema_id)
  REFERENCES schema_versions(id, schema_id)
  ON DELETE RESTRICT;

ALTER TABLE jobs
  ADD CONSTRAINT jobs_schema_account_fk
  FOREIGN KEY (schema_id, account_id)
  REFERENCES schemas(id, account_id)
  ON DELETE RESTRICT;

-- 2) Backfill app runtime completeness, then enforce NOT NULL

DO $$
DECLARE
  app_rec RECORD;
  toolset_id_v UUID;
  toolset_version_id_v UUID;
BEGIN
  FOR app_rec IN
    SELECT id
    FROM apps
    WHERE active_toolset_version_id IS NULL
  LOOP
    toolset_id_v := gen_random_uuid();
    toolset_version_id_v := gen_random_uuid();

    INSERT INTO toolsets (id, app_id, name)
    VALUES (toolset_id_v, app_rec.id, 'default-toolset');

    INSERT INTO toolset_versions (id, toolset_id, app_id, version, status, definition)
    VALUES (toolset_version_id_v, toolset_id_v, app_rec.id, 1, 'active', '{"tools":[]}'::jsonb);

    UPDATE apps
    SET active_toolset_version_id = toolset_version_id_v
    WHERE id = app_rec.id;
  END LOOP;
END
$$;

ALTER TABLE apps
  DROP CONSTRAINT apps_active_toolset_version_fk;

ALTER TABLE apps
  ADD CONSTRAINT apps_active_toolset_version_fk
  FOREIGN KEY (active_toolset_version_id, id)
  REFERENCES toolset_versions(id, app_id)
  ON DELETE RESTRICT
  DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE apps
  ALTER COLUMN active_toolset_version_id SET NOT NULL;

-- 3) Enum type definitions

CREATE TYPE schema_version_status_enum AS ENUM ('draft', 'active', 'deprecated');
CREATE TYPE tool_version_status_enum AS ENUM ('draft', 'active', 'deprecated');
CREATE TYPE toolset_version_status_enum AS ENUM ('draft', 'active', 'deprecated');
CREATE TYPE job_status_enum AS ENUM ('queued', 'running', 'succeeded', 'failed', 'cancelled');
CREATE TYPE job_step_status_enum AS ENUM ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'skipped');
CREATE TYPE media_asset_kind_enum AS ENUM ('source_audio', 'normalized_audio', 'preview', 'thumbnail', 'waveform');
CREATE TYPE transcript_provider_enum AS ENUM ('deepgram', 'speechmatics', 'manual');
CREATE TYPE transcript_status_enum AS ENUM ('queued', 'running', 'succeeded', 'failed');
CREATE TYPE extraction_run_status_enum AS ENUM ('queued', 'running', 'succeeded', 'failed');
CREATE TYPE tool_suggestion_run_status_enum AS ENUM ('queued', 'running', 'succeeded', 'failed');
CREATE TYPE tool_suggestion_status_enum AS ENUM ('pending', 'suggested', 'accepted', 'rejected', 'executed', 'failed');
CREATE TYPE citation_target_type_enum AS ENUM ('extracted_item', 'tool_suggestion');
CREATE TYPE job_event_type_enum AS ENUM (
  'accepted',
  'ingest_started',
  'ingest_completed',
  'transcription_started',
  'transcription_completed',
  'extraction_started',
  'extraction_completed',
  'tool_suggestion_started',
  'tool_suggestion_completed',
  'completed',
  'failed',
  'cancelled',
  'progress',
  'stt_started'
);
CREATE TYPE job_event_level_enum AS ENUM ('debug', 'info', 'warn', 'error');
CREATE TYPE webhook_event_type_enum AS ENUM ('job_progress', 'job_completed', 'job_failed');
CREATE TYPE webhook_delivery_status_enum AS ENUM ('pending', 'retrying', 'succeeded', 'failed');
CREATE TYPE usage_event_type_enum AS ENUM ('stt_audio_ms', 'llm_tokens', 'job_processed', 'webhook_attempt');

-- 4) Strict pre-validation (fail-fast)

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM schema_versions WHERE status NOT IN ('draft', 'active', 'deprecated')) THEN
    RAISE EXCEPTION 'invalid schema_versions.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM tool_versions WHERE status NOT IN ('draft', 'active', 'deprecated')) THEN
    RAISE EXCEPTION 'invalid tool_versions.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM toolset_versions WHERE status NOT IN ('draft', 'active', 'deprecated')) THEN
    RAISE EXCEPTION 'invalid toolset_versions.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM jobs WHERE status NOT IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')) THEN
    RAISE EXCEPTION 'invalid jobs.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM job_steps WHERE status NOT IN ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'skipped')) THEN
    RAISE EXCEPTION 'invalid job_steps.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM media_assets WHERE kind NOT IN ('source_audio', 'normalized_audio', 'preview', 'thumbnail', 'waveform')) THEN
    RAISE EXCEPTION 'invalid media_assets.kind values present';
  END IF;

  IF EXISTS (SELECT 1 FROM transcripts WHERE provider NOT IN ('deepgram', 'speechmatics', 'manual')) THEN
    RAISE EXCEPTION 'invalid transcripts.provider values present';
  END IF;

  IF EXISTS (SELECT 1 FROM transcripts WHERE status NOT IN ('queued', 'running', 'succeeded', 'failed')) THEN
    RAISE EXCEPTION 'invalid transcripts.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM extraction_runs WHERE status NOT IN ('queued', 'running', 'succeeded', 'failed')) THEN
    RAISE EXCEPTION 'invalid extraction_runs.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM tool_suggestion_runs WHERE status NOT IN ('queued', 'running', 'succeeded', 'failed')) THEN
    RAISE EXCEPTION 'invalid tool_suggestion_runs.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM tool_suggestions WHERE status NOT IN ('pending', 'suggested', 'accepted', 'rejected', 'executed', 'failed')) THEN
    RAISE EXCEPTION 'invalid tool_suggestions.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM citations WHERE target_type NOT IN ('extracted_item', 'tool_suggestion')) THEN
    RAISE EXCEPTION 'invalid citations.target_type values present';
  END IF;

  IF EXISTS (SELECT 1 FROM job_events WHERE type NOT IN (
    'accepted', 'ingest_started', 'ingest_completed',
    'transcription_started', 'transcription_completed',
    'extraction_started', 'extraction_completed',
    'tool_suggestion_started', 'tool_suggestion_completed',
    'completed', 'failed', 'cancelled', 'progress', 'stt_started'
  )) THEN
    RAISE EXCEPTION 'invalid job_events.type values present';
  END IF;

  IF EXISTS (SELECT 1 FROM job_events WHERE level NOT IN ('debug', 'info', 'warn', 'error')) THEN
    RAISE EXCEPTION 'invalid job_events.level values present';
  END IF;

  IF EXISTS (SELECT 1 FROM webhook_deliveries WHERE event_type NOT IN ('job_progress', 'job_completed', 'job_failed')) THEN
    RAISE EXCEPTION 'invalid webhook_deliveries.event_type values present';
  END IF;

  IF EXISTS (SELECT 1 FROM webhook_deliveries WHERE status NOT IN ('pending', 'retrying', 'succeeded', 'failed')) THEN
    RAISE EXCEPTION 'invalid webhook_deliveries.status values present';
  END IF;

  IF EXISTS (SELECT 1 FROM usage_events WHERE type NOT IN ('stt_audio_ms', 'llm_tokens', 'job_processed', 'webhook_attempt')) THEN
    RAISE EXCEPTION 'invalid usage_events.type values present';
  END IF;
END
$$;

-- 5) Convert free-text finite domains to strict enums

ALTER TABLE schema_versions
  ALTER COLUMN status TYPE schema_version_status_enum
  USING status::schema_version_status_enum;

ALTER TABLE tool_versions
  ALTER COLUMN status TYPE tool_version_status_enum
  USING status::tool_version_status_enum;

ALTER TABLE toolset_versions
  ALTER COLUMN status TYPE toolset_version_status_enum
  USING status::toolset_version_status_enum;

ALTER TABLE jobs
  ALTER COLUMN status TYPE job_status_enum
  USING status::job_status_enum;

ALTER TABLE job_steps
  ALTER COLUMN status TYPE job_step_status_enum
  USING status::job_step_status_enum;

ALTER TABLE media_assets
  ALTER COLUMN kind TYPE media_asset_kind_enum
  USING kind::media_asset_kind_enum;

ALTER TABLE transcripts
  ALTER COLUMN provider TYPE transcript_provider_enum
  USING provider::transcript_provider_enum,
  ALTER COLUMN status TYPE transcript_status_enum
  USING status::transcript_status_enum;

ALTER TABLE extraction_runs
  ALTER COLUMN status TYPE extraction_run_status_enum
  USING status::extraction_run_status_enum;

ALTER TABLE tool_suggestion_runs
  ALTER COLUMN status TYPE tool_suggestion_run_status_enum
  USING status::tool_suggestion_run_status_enum;

ALTER TABLE tool_suggestions
  ALTER COLUMN status TYPE tool_suggestion_status_enum
  USING status::tool_suggestion_status_enum;

ALTER TABLE citations
  ALTER COLUMN target_type TYPE citation_target_type_enum
  USING target_type::citation_target_type_enum;

ALTER TABLE job_events
  ALTER COLUMN type TYPE job_event_type_enum
  USING type::job_event_type_enum,
  ALTER COLUMN level TYPE job_event_level_enum
  USING level::job_event_level_enum;

ALTER TABLE webhook_deliveries
  ALTER COLUMN event_type TYPE webhook_event_type_enum
  USING event_type::webhook_event_type_enum,
  ALTER COLUMN status TYPE webhook_delivery_status_enum
  USING status::webhook_delivery_status_enum;

ALTER TABLE usage_events
  ALTER COLUMN type TYPE usage_event_type_enum
  USING type::usage_event_type_enum;

-- 6) Performance indexes

CREATE INDEX jobs_app_account_created_id_idx
  ON jobs(app_id, account_id, created_at DESC, id DESC);

CREATE INDEX api_keys_app_account_idx ON api_keys(app_id, account_id);
CREATE INDEX api_keys_account_id_idx ON api_keys(account_id);
CREATE INDEX apps_schema_id_account_id_idx ON apps(schema_id, account_id);
CREATE INDEX apps_active_schema_version_id_idx ON apps(active_schema_version_id);
CREATE INDEX apps_active_toolset_version_id_idx ON apps(active_toolset_version_id);
CREATE INDEX jobs_schema_version_schema_idx ON jobs(schema_version_id, schema_id);
CREATE INDEX jobs_schema_account_idx ON jobs(schema_id, account_id);
