# Architecture — Server Pipeline Core Flow

## Purpose

Define the canonical server-side flow for Veritie jobs from app runtime precondition through final client result fetch, including where progress is streamed and where durable checkpoints are written.

## Scope

Covers the end-to-end 0-19 pipeline stages, runtime snapshot boundaries, checkpointed persistence boundaries, and SSE progress boundaries.

Does not cover websocket transport, real-time chunked audio streaming, or deep indexing as a critical-path requirement.

## Components

- App runtime precondition and resolution (auth + principal + active config references).
- Job bootstrap and snapshot (job creation + immutable runtime snapshot for execution).
- Upload lifecycle (target issuance, direct client upload, finalize/verify).
- Worker lifecycle (queue dispatch, pickup, staged processing).
- Artifact pipelines (transcription, extraction, tool suggestion, optional indexing).
- Persistence checkpoints (job, artifacts, stage boundaries, usage).
- SSE lifecycle stream (lifecycle, stage, and artifact readiness events).
- Canonical read path (`GET /v1/jobs/{job_id}` and reconnect semantics).

## Boundaries

- App is the workflow boundary: clients should not negotiate schema/toolset runtime per job in MVP.
- Job creation is the runtime freeze point: downstream execution should use the job snapshot, not mutable live app configuration.
- SSE is a progress channel only; persisted job/artifact reads are canonical truth.
- Persistence checkpoints are stage-stable and should not be forced to match SSE event granularity.
- Tool suggestion should execute after extraction in MVP to keep orchestration and grounding predictable.

## Invariants

- Job bootstrap should resolve principal-scoped runtime context before stage execution starts.
- Jobs should carry immutable references/snapshots required for replayability.
- Upload should remain direct-to-storage for short audio MVP with server-side finalize verification.
- Stage progression should be ordered and checkpointed at stable artifact boundaries.
- Optional stages should support non-fatal failure semantics (`partial_success`) when product policy allows.
- Reconnect/recovery should resolve from persisted state first, then resume live stream behavior.

## Non-Goals

- Replacing `GET /v1/jobs/{job_id}` with SSE-only state recovery.
- Capturing every runtime delta as durable DB writes.
- Parallelizing extraction and tool suggestion in MVP.
- Making indexing mandatory for initial completion semantics.
