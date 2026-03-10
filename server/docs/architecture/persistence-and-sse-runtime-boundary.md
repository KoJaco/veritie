# Architecture — Persistence and SSE Runtime Boundary

## Purpose

Define the separation between high-frequency runtime delivery (SSE) and durable persistence checkpoints so event granularity does not force database write granularity.

## Scope

Covers server-side runtime event emission, durable checkpoint persistence, replay ordering guarantees, and recovery boundaries for jobs.

Does not cover client rendering behavior, webhook delivery policy, or provider-specific retry algorithms.

## Components

- Runtime stream layer: in-process stage progress emissions for SSE subscribers.
- Persistence checkpoint layer: durable writes to `jobs`, `job_events`, and stage artifacts at stable boundaries.
- Replay layer: reads persisted `job_events` in deterministic order for reconnect.

## Boundaries

- SSE is a transport for near-real-time UX updates and may emit events that are not persisted.
- Persistence stores coarse, operationally meaningful lifecycle checkpoints and stable artifacts.
- Replay on reconnect starts from persisted events and then transitions to live runtime emissions.

## Invariants

- Do not model persistence 1:1 with SSE message volume.
- Persist at stable boundaries: accepted, stage finalized, completed, failed.
- Persisted `job_events` remain append-only and are ordered by `(created_at, id)`.
- Job state changes and persisted event checkpoints should be written atomically when they describe the same boundary transition.

## Non-Goals

- Capturing every transient runtime progress delta in the database.
- Using SSE as the source of truth for durable job reconstruction.
