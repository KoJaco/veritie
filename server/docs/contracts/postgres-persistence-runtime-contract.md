# Contract: Postgres Persistence Runtime Contract

## Purpose

Define the internal persistence contract for Veritie Postgres jobs/events writes and reads, including runtime snapshot resolution, ordering guarantees, transaction expectations, and finite-domain enforcement.

## Scope

Included:
- Jobs repository create/rerun/read/update/list behavior.
- Events repository append/list/cursor behavior.
- App-runtime snapshot semantics for job creation.
- Idempotency and deterministic ordering semantics.
- Schema constraints that enforce finite-domain and ownership rules.

Out of scope:
- External HTTP/SDK payload wire format.
- Worker orchestration policies beyond persistence boundary semantics.

## Versioning

- **Current version:** v1
- **Compatibility:** Backward compatible for current refactor runtime expectations
- **Change policy:** Major bump when ordering/idempotency/snapshot semantics change incompatibly; minor bump for additive constraints/examples.

## Definitions

- **Job snapshot:** Config JSON persisted on a job from app runtime (`processing_config`, `runtime_behavior`, `llm_config`) resolved server-side.
- **Scoped read:** Query constrained by `job_id`, `app_id`, and `account_id`.
- **Cursor read:** Ordered event read after `(created_at, id)` for replay continuity.
- **Finite domain enum:** Postgres enum-backed column for bounded state/type fields.

## Contract Shape (Conceptual)

### Required fields

- `jobs.app_id` / `jobs.account_id` — tenant/app ownership scope for job records.
- `jobs.schema_id` / `jobs.schema_version_id` / `jobs.toolset_version_id` — runtime execution references.
- `jobs.status` — enum-backed lifecycle status.
- `jobs.config_snapshot` — persisted server-resolved runtime snapshot.
- `job_events.job_id` / `job_events.type` / `job_events.message` — immutable event append contract.

### Optional fields

- `jobs.idempotency_key` — optional app-scoped dedup key.
- `jobs.rerun_of_job_id` — optional provenance linkage for reruns.
- `job_events.data` / `job_events.progress` — stage metadata and progress payload.
- cursor tuple `(created_at, id)` for event replay pagination.

## Invariants (Must Always Hold)

- Job create/rerun should resolve runtime references from app-owned active versions, not client runtime payloads.
- `jobs.idempotency_key` uniqueness should be enforced per app when present.
- Event replay/listing should be deterministic by `(created_at ASC, id ASC)`.
- Transactional state+event writes should be possible atomically through tx-aware repo methods.
- Finite-domain lifecycle/type fields should remain enum-backed and fail-fast on invalid values.
- Cross-tenant ownership should remain FK/index constrained for app/account-scoped records.

## Error Handling

- Missing required job/event parameters are rejected by repository validation.
- Invalid enum/domain values fail at DB boundary via strict enum typing.
- App runtime resolution failure on create/rerun fails job write before insert.
- Duplicate idempotency key conflicts return DB uniqueness errors for caller mapping.

## Examples

### Minimal valid example

```json
{
  "create_job": {
    "app_id": "f3fbe293-8c2a-46f2-b21c-3a0259e4664f",
    "account_id": "ce8b03f8-57d2-4c2c-b3f7-3910345f3e92",
    "status": "queued",
    "audio_uri": "s3://bucket/audio.wav",
    "audio_size": 1024,
    "audio_duration_ms": 30000,
    "audio_content_type": "audio/wav",
    "idempotency_key": "job-create-001"
  },
  "append_event": {
    "job_id": "3e26c67f-53ad-4ac2-90fd-af38496dfd00",
    "type": "stt_started",
    "message": "transcription started",
    "progress": 0.1
  }
}
```

### Invalid example

```json
{
  "create_job": {
    "app_id": "f3fbe293-8c2a-46f2-b21c-3a0259e4664f",
    "account_id": "ce8b03f8-57d2-4c2c-b3f7-3910345f3e92",
    "status": "queued",
    "audio_uri": "s3://bucket/audio.wav",
    "audio_size": 1024,
    "audio_duration_ms": 30000,
    "audio_content_type": "audio/wav",
    "config_snapshot": {
      "client_override": true
    }
  }
}
```

Expected handling:
- Client-supplied runtime config override is ignored as authoritative input.
- Job write should rely on server-side runtime bundle lookup from app active references.
- Invalid finite-domain values should be rejected by enum constraints at DB boundary.

### Operational notes

- Current DB vocabulary includes `jobs.status` enum values with `succeeded`/`cancelled`.
- Pipeline docs currently use `completed`/`partial_success` terminology.
- This is a temporary cross-branch vocabulary delta and is not a branch-06 blocker.

### References

- Related ADRs: `server/docs/adr/ADR-0002-postgres-persistence-boundary-and-hardening.md`, `server/docs/adr/ADR-0001-server-pipeline-runtime-boundary.md`
- Related architecture: `server/docs/architecture/postgres-persistence-boundary.md`, `server/docs/architecture/server-pipeline-core-flow.md`
- Related decisions: `server/docs/decisions/postgres-branch-06-completion-and-latency-defaults.md`
- Related refactor branch: `server/docs/refactor/06-server-db-postgres-core.md`
- Issue/PR: #
