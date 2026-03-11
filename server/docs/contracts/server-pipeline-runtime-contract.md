# Contract: Server Pipeline Runtime Contract

## Purpose

Define the canonical runtime contract for Veritie server pipeline execution, including stage flow, checkpointed persistence behavior, SSE event behavior, and operational latency guidance.

## Scope

Included:
- Stage sequence 0-19 for short-audio MVP flow.
- Stage inputs/outputs, persistence checkpoints, SSE event semantics, and retry posture.
- Lifecycle/stage/artifact event taxonomy.
- Guidance-level invariants and SLA-style targets for operational behavior.

Out of scope:
- Provider-specific low-level tuning details.
- Frontend rendering policy.
- Real-time chunked transport or websocket protocol design.

## Versioning

- **Current version:** v1
- **Compatibility:** Backward compatible (guidance-level normative doc for MVP shape)
- **Change policy:** Bump major when stage identity/event semantics/checkpoint boundaries change incompatibly; bump minor for additive guidance.

## Definitions

- **Job snapshot:** Immutable runtime context captured at job creation from app-scoped active config.
- **Checkpoint:** Durable persistence boundary where state/artifacts are committed.
- **Lifecycle event:** Job-level event (created, queued, started, terminal).
- **Stage event:** Processing stage status event (`stage.started`, `stage.progress`, `stage.completed`, `stage.failed`).
- **Artifact event:** Event indicating durable or preview artifact availability (for example, `transcript.ready`).
- **Canonical read:** `GET /v1/jobs/{job_id}` response as durable source of truth.

## Contract Shape (Conceptual)

### Required fields

- `job_id` — stable identifier for the job lifecycle.
- `stage` — current pipeline stage identity where relevant.
- `event` — lifecycle, stage, or artifact event name.
- `timestamp` — server emission timestamp.
- `status` — current job state (`queued`, `running`, `completed`, `failed`, `partial_success`).

### Optional fields

- `data` — stage-specific metadata.
- `sequence` — monotonic ordering hint for stream consumption.
- `retryable` — transient/permanent failure hint.
- `snapshot_ref` — runtime snapshot/version reference.

## Invariants (Must Always Hold)

- App should be treated as workflow boundary for runtime config selection in MVP.
- Job creation should snapshot active runtime config and downstream stages should use that snapshot.
- SSE should be treated as progress transport, not canonical state storage.
- Persistence should happen at stable checkpoints, not per SSE emission.
- Tool suggestion should run after extraction in MVP.
- Optional stages should not force whole-job failure when policy supports `partial_success`.
- Canonical reconciliation should always be possible through `GET /v1/jobs/{job_id}`.

## Error Handling

- Invalid lifecycle transitions should be rejected as contract violations and emitted as stage/job failures with typed reasons.
- Upload finalize should be idempotent; duplicate calls should not duplicate checkpoints.
- Retryable failures should be bounded by per-stage retry policies; non-retryable failures should transition deterministically.
- Stream disconnects should be recoverable via canonical read and optional `job.snapshot` stream behavior.

## Examples

### Minimal valid example

```json
{
  "job_id": "job_123",
  "event": "stage.completed",
  "stage": "transcription",
  "status": "running",
  "timestamp": "2026-03-11T09:00:00Z"
}
```

### Invalid example

```json
{
  "job_id": "job_123",
  "event": "job.completed",
  "stage": "extraction",
  "status": "running"
}
```

Expected handling: reject as inconsistent terminal lifecycle payload; require canonical persisted terminal transition before emitting `job.completed`.

### Operational notes

#### Stage identity and dependency (canonical)

```text
0  App runtime precondition
1  Client capture/initiation
2  Job bootstrap
3  Awaiting upload
4  Client upload
5  Upload finalize / verify
6  Queue dispatch / worker pickup
7  Media fetch / optional normalization
8  Transcription
9  Transcript persistence
10 Optional classification / pre-analysis
11 Structured extraction
12 Citation resolution (extraction)
13 Extraction persistence checkpoint
14 Tool suggestion
15 Citation resolution (tool suggestion)
16 Tool suggestion persistence checkpoint
17 Optional indexing / retrieval prep
18 Finalization
19 Canonical result fetch / reconnect
```

#### Stage table (0-19)

| Stage | Purpose | Inputs | Outputs | Persisted writes | SSE events | Latency/performance risks | Mitigations | Retry policy |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 0. App runtime precondition | Resolve executable app runtime before job creation | App, active refs, auth config | Runnable app context | None in request path | None | Multi-lookup resolution, late invalid config | Keep active refs on app, cache runtime, validate on publish/update | Fail fast at job create |
| 1. Client capture/initiation | Start capture/file flow and prepare job create | User action, local media metadata | Capture session or file selected | None | Optional local UI only | Record-end dead time, late file rejection | Parallel create during recording, early client validation, explicit preparing state | Local user retry |
| 2. Job bootstrap | Authenticate, resolve runtime, create job, return upload instructions | Auth credential, idempotency key, metadata | `job_id`, upload instructions, optional stream endpoint, job snapshot | `jobs` (+ optional lightweight placeholders/events/steps) | `job.created`, optional `job.snapshot` | DB round-trips, cold start, slow upload-target generation | Indexed key lookup, runtime cache, minimal writes, regional colocation | Idempotent retry with idempotency key |
| 3. Awaiting upload | Place job into upload-wait state | `job_id`, upload target | Upload-ready job | Optional job/step status update | Optional `upload.ready` | Expired upload target, abandoned awaiting jobs | Generous TTL, stale sweeper, refresh support | Refresh target or recreate upload session |
| 4. Client upload | Transfer bytes to object storage | File/blob, upload target, storage key | Object stored | No authoritative server write during transfer | Usually client-local progress UI | Uplink speed, region mismatch, single-part full retry | Short-audio size caps, colocated storage, compressed formats, local progress | Client retries; refresh target if expired |
| 5. Upload finalize / verify | Verify uploaded object and queue processing | `job_id`, storage metadata, expected object info | Verified media and ingest complete | `media_assets`, `jobs`, ingest step/event updates | `upload.verified`, ingest `stage.completed`, `processing.queued` | Blind trust, duplicate finalize, stranded uploads | Idempotent finalize, minimal verification, uploaded-not-finalized sweeper | Safe idempotent finalize retries |
| 6. Queue dispatch / worker pickup | Move job into processing execution | Verified job + snapshot + media refs | Assigned worker or processing started | Step/event queued/started updates | `processing.queued`, `processing.started`, transcription `stage.started` | Queue backlog, cold workers, duplicate pickups | Separate queued vs started, hot workers if possible, idempotent claims | Retry queue publish; idempotent pickup |
| 7. Media fetch / optional normalization | Prepare media for STT | Media ref, storage object, runtime config | STT-ready input | Usually none or step progress only | Transcription `stage.progress` | Storage fetch latency, expensive normalization | Restrict formats, avoid unnecessary normalization, colocation | Retry transient storage read failures |
| 8. Transcription | Produce transcript from audio | STT input, language/runtime hints | Final transcript (+ optional partials) | No final durable write until stable | `stage.started`, `stage.progress`, optional `transcript.partial` | Provider latency/cold starts, retry inflation | Short-audio-friendly STT config, bounded retries, coarse progress | Bounded retry on transient failures |
| 9. Transcript persistence | Persist transcript and segments | Final transcript output | Durable transcript artifacts | `transcripts`, `transcript_segments`, step/event updates | `transcript.ready`, transcription `stage.completed` | Chatty per-segment writes, segmentation overhead | Batch inserts, lean schema, deterministic segmentation | Retry before commit with idempotent write semantics |
| 10. Optional classification / pre-analysis | Lightweight labels/quality/gating | Transcript + runtime config | Optional metadata | Minimal, only when meaningful | Optional classification stage events | Latency without value, duplicated logic | Keep lightweight/fail-soft, fold into extraction when possible | Retry only if critical, else skip |
| 11. Structured extraction | Produce schema-aligned outputs | Transcript, segments, schema snapshot, runtime config | Normalized extracted outputs | No final durable write until stable | Extraction `stage.started`/`stage.progress`, optional preview | Large schema/prompt latency, validation overhead | Compact schemas, prevalidation, early progress, output caps | Bounded retry on transient model failures |
| 12. Citation resolution (extraction) | Ground extraction outputs in transcript evidence | Extraction candidates + segments | Extraction citations | Usually with extraction checkpoint | Usually included in extraction progress | Citation matching cost | Bounded in-memory grounding, segment-anchored matching | Retry as extraction sub-stage |
| 13. Extraction persistence checkpoint | Persist extraction run/items/citations | Stable extraction payloads | Durable extraction artifacts | `extraction_runs`, `extracted_items`, `citations`, step/event updates | `extraction.ready`, extraction `stage.completed` | N+1 writes, oversized transaction | Batch/chunk inserts, generic item model | Retry before commit; idempotent run semantics |
| 14. Tool suggestion | Generate suggested actions from transcript + extraction + toolset | Transcript, extracted items, toolset snapshot | Suggested actions + rationale/citation candidates | No final durable write until stable | Tool suggestion stage started/progress | Duplicated reasoning, oversized tool context | Run after extraction, compact single active toolset, disable when not configured | Bounded retry on transient failures |
| 15. Citation resolution (tool suggestion) | Ground suggested actions to transcript evidence | Suggestions + segments | Suggestion citations | Usually with suggestion checkpoint | Usually embedded in stage progress | Additional citation overhead | Reuse extraction grounding approach | Retry as suggestion sub-stage |
| 16. Tool suggestion persistence checkpoint | Persist suggestion run/suggestions/citations | Stable suggestion payloads | Durable suggestion artifacts | `tool_suggestion_runs`, `tool_suggestions`, `citations`, step/event updates | `tool_suggestions.ready`, tool suggestion `stage.completed` | Batch overhead, optional-stage failure propagation | Batch inserts, allow optional-stage partial semantics | Retry before commit; else mark optional failure |
| 17. Optional indexing / retrieval prep | Build retrieval/index artifacts where needed | Transcript segments, extracted items | Optional index artifacts | Optional future tables + step updates | Optional indexing stage events | Time/cost dominates critical path | Keep off critical path, run background/optional | Background retries; non-blocking |
| 18. Finalization | Commit terminal consistency and enqueue side effects | Stage terminals, job id, usage, delivery config | Terminal status + downstream enqueue | terminal `jobs`, `usage_events`, final steps/events, optional webhook enqueue | Finalization stage + terminal lifecycle event | Finalization dumping ground, synchronous side-effect delay | Keep narrow, enqueue side effects, emit terminal after durable commit | Retry commit before terminal write |
| 19. Canonical result fetch / reconnect | Enable durable fetch and reconnect recovery | `job_id`, auth, optional cursor | Canonical job details + artifacts | Usually none | Optional `job.snapshot` on reconnect | Missed stream state, stale reconnect UI | Use canonical GET as truth, snapshot/replay-lite on reconnect | Read retries are freely retryable |

#### Persistence checkpoints summary

| Checkpoint | Durable writes |
| --- | --- |
| Job bootstrap | `jobs` (+ optional upload placeholder or initial step/event) |
| Upload verified / ingest complete | `media_assets`, ingest job/step/event updates |
| Transcript ready | `transcripts`, `transcript_segments`, transcription step/event updates |
| Extraction ready | `extraction_runs`, `extracted_items`, `citations`, extraction step/event updates |
| Tool suggestions ready | `tool_suggestion_runs`, `tool_suggestions`, `citations`, suggestion step/event updates |
| Finalization | terminal `jobs` status, `usage_events`, final step/event updates, optional webhook enqueue |

#### SSE event contract summary

Core lifecycle events:
- `job.created`
- `upload.ready`
- `upload.verified`
- `processing.queued`
- `processing.started`
- `job.completed`
- `job.failed`
- `job.partial_success`

Stage events:
- `stage.started`
- `stage.progress`
- `stage.completed`
- `stage.failed`

Stage names:
- `ingest`
- `transcription`
- `classification` (optional)
- `extraction`
- `tool_suggestion`
- `indexing` (optional)
- `finalization`

Artifact events:
- `transcript.partial` (optional)
- `transcript.ready`
- `extraction.ready`
- `tool_suggestions.ready`
- `job.snapshot` (reconnect/recovery)

#### SLA-style targets (guidance)

| Path | Target |
| --- | --- |
| Job bootstrap | `< 300 ms typical` |
| Upload finalize + verify | `< 500 ms typical` |
| Upload-complete to processing-start | `< 1 s typical`, `< 2 s acceptable` |
| Queue pickup | `< 1 s typical` |
| Checkpoint DB writes | batched, bounded, avoid per-item chatter |

### References

- Related ADRs: `server/docs/adr/ADR-0001-server-pipeline-runtime-boundary.md`
- Related architecture: `server/docs/architecture/server-pipeline-core-flow.md`, `server/docs/architecture/persistence-and-sse-runtime-boundary.md`
- Related decision note: `server/docs/decisions/server-pipeline-latency-prioritization.md`
- Related refactor docs: `server/docs/refactor/ground-truth.md`, `server/docs/refactor/08-server-jobs-domain-state-machine.md`, `server/docs/refactor/10-server-worker-runner-orchestration.md`, `server/docs/refactor/11-server-http-jobs-contract.md`, `server/docs/refactor/12-server-sse-stream-contract.md`
