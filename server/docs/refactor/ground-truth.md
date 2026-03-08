# Veritie Infrastructure Guide + Specification

## 1. Purpose

### 1.1 Product intent

Veritie is a **voice-native structured event system**. It ingests short audio, produces **schema-validated structured objects**, and attaches **traceable evidence** (field → transcript span/timestamp), with full **job-level observability** and rerun capability.

### 1.2 Infrastructure intent

The infrastructure is an **auditable processing engine** that:

- authenticates a principal/app
- loads schemas/config
- runs STT + classification + extraction pipelines
- emits structured outputs with evidence
- persists artifacts and job events
- streams job progress to clients via SSE
- supports reruns with versioned config

This is **not** a notes backend. It is an event processing platform optimized for **trust, traceability, and operational clarity**.

---

## 2. Core Principles

1. **Trust-first outputs**
    - Prefer omission over guessing.
    - Represent unknowns explicitly.
    - Do not silently resolve ambiguous references.

2. **Auditability**
    - Every structured field can be traced back to transcript/audio evidence.
    - Every job is replayable (same inputs + config version).

3. **Observability by default**
    - Jobs are event-sourced (or event-log-driven).
    - Clients can watch progress live.

4. **Two-phase contract**
    - **Capture**: deterministic extraction only.
    - **Interpret**: optional post-capture enrichment/suggestions.

5. **Constrained relationships**
    - Relationships depth-1 only (object → related entity).
    - More complex structure is represented via tags and proposals.

---

## 3. Scope

### 3.1 In scope (MVP infra)

- Short audio batch ingestion (upload or signed URL)
- STT abstraction and transcript normalization
- Schema retrieval per principal (OpenAPI/JSON schema semantics)
- Optional classifier to select applicable schemas
- Extraction agent(s) to produce structured objects
- Evidence indexing: field → transcript span mapping
- Job lifecycle persistence + SSE streaming
- Daily review model is a UI concern; backend supports “pending” state
- Reruns with different schemas/config; retain prior outputs
- Tags (m–m) and depth-1 relationships with explicit CRUD tool list

### 3.2 Out of scope (initially)

- Deep multi-hop graph inference and relationship chaining
- Fully general “memory layer” used during extraction
- Automatic resolution of “same as yesterday” without user confirmation
- Real-time streaming capture pipeline (unless already built separately)
- Multi-system integrations beyond export surfaces (e.g., CSV)

---

## 4. System Overview

### 4.1 High-level pipeline

1. **Ingest** audio + metadata
2. **Create job** + begin emitting job events
3. **STT** → transcript with timestamps
4. **Evidence index** built on transcript (token/segment spans)
5. **Classify** (optional) → choose schema(s)
6. **Extract** structured object(s) per chosen schema
7. **Validate** objects (runtime schema validation)
8. **Trace** each field to transcript span (or mark unknown/unresolved)
9. **Persist outputs** + emit final job state
10. **Rerun** supported by replay with alternate schema/config

### 4.2 Trust model

- Output is always accompanied by evidence pointers or “unknown/unresolved”
- Suggested/enrichment outputs are stored separately from accepted facts

---

## 5. Domain Model

### 5.1 Key concepts

- **Principal / App**: authenticated caller; owns schemas and config
- **Schema**: versioned definition(s) of extractable object types
- **Job**: one processing run over an audio input with a specific config snapshot
- **Artifact**: transcript, audio references, evidence index, structured outputs
- **Job Event**: immutable event log entries (status updates, stage outputs, errors)
- **Object**: extracted structured instance (e.g., Expense, WorkLog, Todo)
- **Relationship (depth-1)**: object references another entity (e.g., WorkLog → Project)
- **Tag**: flexible multi-label metadata for pseudo-hierarchy and grouping
- **Proposal**: non-final enrichment suggestions (links, merges, inferred tags)

---

## 6. Architecture

### 6.1 Recommended service boundaries (single service, modularized)

- `transport/http` (REST endpoints for jobs and control surfaces)
- `transport/sse` (event streaming endpoint and stream lifecycle handling)
- `app` (orchestration / use-cases)
- `domain` (entities + ports)
- `infra/stt` (Speechmatics, others)
- `infra/llm` (agent runtime, model adapters)
- `infra/db` (repos, migrations)
- `infra/obs` (logging/metrics/tracing)
- `infra/storage` (audio/transcript storage pointers)
- `pkg/schema` (schema parsing, validation, normalization)
- `pkg/evidence` (span index + mapping utilities)

### 6.2 Concurrency model

Each job executes as a state machine:

- stages run sequentially by default
- some stages may run in parallel where safe (e.g., evidence indexing in parallel with initial transcript normalization; later “interpret” stages separate)
- every stage emits events for SSE + persistence

### 6.3 Idempotency

- job creation should accept a client-supplied idempotency key
- reruns create new jobs referencing original job_id

---

## 7. APIs and Contracts

### 7.1 Auth

- Each request is authenticated as an **App** under a principal.
- Auth returns an **AppConfig snapshot** including:
    - schema set + versions
    - model preferences (none for now, default to provided models)
    - extraction settings (strictness, omit-vs-guess policy)
    - tag policies
    - relationship policies

### 7.2 Batch ingestion endpoints (illustrative)

- `POST /v1/jobs`
  Create a processing job. Accepts:
    - audio upload OR `audio_url` OR storage pointer
    - metadata: locale, device context, optional “current_project_id”
    - idempotency key

Returns:

- `job_id`

- `status_url`

- `stream_url`

- `GET /v1/jobs/{job_id}`
  Returns job summary + outputs references.

- `GET /v1/jobs/{job_id}/stream` (SSE)
  Streams job events in real time.

### 7.3 Rerun endpoint

- `POST /v1/jobs/{job_id}/rerun`
    - select schema set override or config override
    - creates new job: `rerun_of_job_id`

### 7.4 Export endpoints (initial)

- `GET /v1/exports/{type}.csv?from=...&to=...`
- `GET /v1/exports/all.csv?from=...&to=...`

(Exports should reflect accepted facts, not proposals.)

---

## 8. Job Lifecycle Spec

### 8.1 States

- `queued`
- `running`
- `succeeded`
- `failed`
- `cancelled`

### 8.2 Stages (evented)

- `ingest_started`
- `stt_started`
- `stt_completed`
- `evidence_index_started`
- `evidence_index_completed`
- `classification_started` (optional)
- `classification_completed`
- `extraction_started`
- `extraction_completed`
- `validation_started`
- `validation_completed`
- `persist_started`
- `persist_completed`
- `completed`

### 8.3 Event payload conventions

Every event includes:

- `job_id`
- `timestamp`
- `stage`
- `level` (info/warn/error)
- `message`
- `data` (stage-specific)
- `trace_id` / `span_id` (if tracing enabled)
- `config_version_hash` (snapshot identifier)

---

## 9. Schemas

### 9.1 Storage & versioning

- Schema definitions are stored versioned per principal/app.
- Jobs pin the schema versions used via `config snapshot hash`.

### 9.2 Validation

- All extracted objects must validate against the schema.
- Invalid outputs become:
    - either hard failure (strict mode)
    - or partial output with `validation_errors` (lenient mode)

### 9.3 Complexity guidance

- deeply nested schemas degrade extraction quality
- provide internal helper/warnings for schema authors
- encourage normalized types + tags for hierarchy-like needs

---

## 10. Agent Framework Spec

### 10.1 Agent types

- **Classifier Agent** (optional): selects schema(s) to apply
- **Extractor Agent** (per schema or schema-pack): produces object candidates
- **Validator/Repair Agent** (optional): attempts to repair invalid output (strictly bounded; must not invent facts)
- **Interpret Agent** (post-capture, optional): generates proposals only

### 10.2 Output contract: facts vs proposals

Agents must output one of:

- **Fact object(s)**: schema-valid, evidence-backed or explicitly unknown
- **Proposal(s)**: suggested links/tags/merges, never auto-applied unless rules permit

### 10.3 Unresolved references (contextual speech)

For utterances like “same as yesterday,” agents must emit:

- `reference_needed` entries with hints and constraints (e.g., date window)
- `needs_review=true`
- optional candidate query parameters for the UI/service to retrieve likely matches

No automatic linking at capture time.

---

## 11. Traceability and Evidence Index

### 11.1 Evidence requirements

For each extracted object:

- each field may carry:
    - `evidence`: transcript span(s) and/or timestamp span(s)
    - `confidence` (optional)
    - `status`: { asserted | unknown | inferred | unresolved_reference }

“Inferred” should be strongly discouraged in capture; reserve for interpret proposals.

### 11.2 Evidence index design

- Build an index from transcript tokens/segments:
    - token offsets
    - segment IDs
    - start/end times

- Provide stable handles for evidence pointers:
    - `segment_id` + `start_char` + `end_char`
    - and/or `t_start_ms` / `t_end_ms`

### 11.3 Storage

- Evidence pointers stored with objects
- Full transcript stored separately (or pointer to storage), including:
    - diarization if present
    - normalized text and raw text
    - per-segment timing metadata

---

## 12. Relationships and Tags

### 12.1 Depth-1 relationships

Supported:

- object contains a single reference to another entity (e.g., `project_id`)
- resolution rules:
    - explicit mention in speech, OR
    - provided “current context” (e.g., `current_project_id`)

### 12.2 Why depth-1 is enforced

- prevents inference chains
- keeps review cost bounded
- keeps evidence mapping meaningful
- preserves trust

### 12.3 Tags

Tags are first-class and multi-assignable:

- used to emulate hierarchy without asserting brittle subtask graphs
- recommend typed tags (namespace style):
    - `area:aft_cabin`
    - `system:generator`
    - `issue:power_drop`

---

## 13. Data Persistence Spec

### 13.1 Minimum tables (conceptual)

- `apps` / `principals` (auth scope)
- `schemas` (versioned)
- `schema_sets` or `app_config` (which schemas apply)
- `jobs`
- `job_events`
- `job_artifacts` (audio ref, transcript ref, evidence index ref)
- `extracted_objects` (facts)
- `object_evidence` (if normalized)
- `object_tags` + `tags`
- `object_relationships` (depth-1 references)
- `proposals` (suggested links/tags/merges)
- `rerun_links` (job lineage)

### 13.2 Rerun semantics

- Rerun creates a new job with:
    - `rerun_of_job_id`
    - new config snapshot
    - outputs stored alongside previous outputs

- Optionally allow “candidate replacement” proposal:
    - “Job B output replaces Job A output if accepted”

---

## 14. Observability

### 14.1 Logging

- structured logs with job_id and stage
- errors include:
    - root cause
    - external request IDs (STT/LLM)
    - retryability classification

### 14.2 Metrics (minimum)

- job latency by stage
- STT latency and failure rate
- extraction success rate per schema
- validation failure counts
- rerun frequency
- “needs_review” rate
- corrections rate (if captured)

### 14.3 Tracing

- trace per job with stage spans
- attach vendor calls as spans (STT, LLM)

### 14.4 SSE

- stream the same job events you persist
- SSE is a view over the event log, not a separate truth

---

## 15. Reliability and Failure Modes

### 15.1 Retry strategy

- STT calls: retry with backoff on transient failures
- LLM calls: retry only when safe; idempotency tokens recommended
- Persist stage must be idempotent

### 15.2 Degraded operation

- if evidence indexing fails, job can still produce transcript + extraction but mark traceability degraded
- if extraction fails, still persist transcript and job events for postmortem

### 15.3 Data retention

- raw audio may be ephemeral; transcript and structured objects durable
- retention policies should be configurable per principal (future)

---

## 16. Security and Privacy

- secure storage pointers for audio/transcripts
- strict separation by principal/app
- careful logging (avoid leaking transcript contents in logs by default)
- explicit opt-in for transcript retention (future-oriented but consistent with your values)
- ensure SSE streams require auth and cannot be guessed

---

## 17. Testing Strategy

### 17.1 Unit tests

- schema validation and normalization
- evidence pointer correctness (span integrity)
- job state machine transitions

### 17.2 Integration tests

- STT adapter contract tests with fixtures
- LLM agent contract tests with golden transcripts
- replay/rerun determinism tests

### 17.3 End-to-end tests

- ingest → SSE events → final outputs
- rerun using alternate schema set
- “unresolved reference” flow emits correct structured signals

---

## 18. Roadmap Extensions That Preserve Trust

1. **Reference resolution (no “memory layer”)**
    - detect reference phrases
    - emit unresolved reference objects
    - UI selects from candidates (retrieval-based suggestions)

2. **Interpret phase proposals**
    - suggested merges, inferred tags, suggested links
    - stored separately from accepted facts

3. **Schema packs**
    - curated object bundles with strong defaults
    - hidden schema by default; inspectable advanced mode

4. **Exports**
    - CSV by type, time window, project
    - later: connectors

5. **Enterprise posture**
    - retention controls
    - deployment options
    - stronger audit reports

---

## 19. Non-Negotiable Constraints to Document

These should be explicit “system laws”:

- Capture does not perform multi-hop inference.
- Capture does not silently resolve ambiguous references.
- Relationships are depth-1 only.
- Facts and proposals are stored separately.
- Any accepted fact must have evidence or be explicitly marked unknown.

---
