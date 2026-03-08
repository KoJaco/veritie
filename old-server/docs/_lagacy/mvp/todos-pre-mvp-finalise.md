# Todos

1. Update SDK and Server, security fix (no API key in browser).

-   Provide snippets for common frameworks for server-server requests
-   update SDK to new approach
-   update Server to new approach
-   Update dashboard to use new approach (remix snippet)
-   Test imple.

2. We need to allow users of the app to be able to test our schemas. Schema creation and making use of these schemas also hinges on our checksum generation.

-   Get checksums working correctly. Schemas should not be saved twice into a database.
-   Adjust database schema. Sessions can have a test tag. This functionality must be pushed all the way through, from SDK -> transport/message -> domain/speech -> repos. This allows a user to test particular schemas... will require CRUD in dashboard for schemas and a testing view. Sandbox mode for testing, send test flag but also check origin in server (testing must come from within the schma site).

-   I also want a session_type column (enum = ["structured_output", "function_calling", "enhanced_text", "basic", "mixed"])

3. Pricing transparency

-   Config ping endpoint to provide back (given the supplied config) the price per minute (should be estimate + padding). isSchemaValid, Estimate token usage (prompt/completion), estimate savings (tokenise and check if caching should be applied),
-   during-session cost updates. Push updates to the client SDK, display current usage for the session and allow them to do whatever with that.

4. PII and PHI redaction in-place

-   Using distilBERT for PII redaction
-   Using ClinicalBERT for PHI redaction
-   both redactors should fulfill the same domain.. we're redacting transcripts, function call arguments, and structured output values.
-   redaction is default unless specified in the config params. Redaction should be specified per config (read in WS). Update SDK and transport/message to reflect this, then handler must read in config

5. Refactor storage patterns. Should be using Supabase Storage for large JSON

-- CORE
create table sessions (
id uuid primary key default gen_random_uuid(),
account_id uuid not null,
started_at timestamptz not null default now(),
ended_at timestamptz,
duration_ms int,
stt_provider text,
llm_model text,
status text check (status in ('processing','complete','error')) default 'processing',

-- small, scalar metrics for fast dashboards
turns_count int default 0,
words_count int default 0,
tokens_in int default 0,
tokens_out int default 0,
tokens_saved int default 0,
cost_usd numeric(10,5) default 0,

-- storage pointers (null if not uploaded)
turns_url text,
words_url text,
metrics_url text,

-- RLS joins
created_by uuid
);

create index on sessions (account_id, started_at desc);
create index on sessions (status);

-- Optional: light turn table for search/UX; keep ONLY plain text + a few fields
create table transcript_turns (
session_id uuid not null references sessions(id) on delete cascade,
idx int not null, -- 0-based turn index
speaker text, -- optional
start_ms int,
end_ms int,
text text not null, -- plain text (no heavy JSON)
primary key (session_id, idx)
);

-- Full-text search index on turns.text (choose config to taste)
alter table transcript_turns
add column tsv tsvector generated always as (to_tsvector('simple', coalesce(text, ''))) stored;
create index transcript_turns_tsv_idx on transcript_turns using gin (tsv);

-- Structured output schema (dedup by checksum)
create table so_schemas (
checksum text primary key, -- e.g., sha256 of the schema JSON
name text,
version text,
schema_json jsonb not null,
created_at timestamptz default now()
);

-- Structured output instances (choose one of the two storage strategies below)
create table structured_outputs (
id uuid primary key default gen_random_uuid(),
session_id uuid not null references sessions(id) on delete cascade,
schema_checksum text references so_schemas(checksum),
-- Strategy A (small): inline
output_json jsonb, -- NULL if using storage_url
-- Strategy B (large or versioned): pointer
storage_url text, -- NULL if using output_json
created_at timestamptz default now(),
is_final boolean default true -- mark drafts vs final
);

create index on structured_outputs (session_id, created_at desc);
create index on structured_outputs (schema_checksum);

6. Apps need a way to access their data. Need a rest api so businesses can read all their data.

-   HMMMM
