# Architecture — Postgres Persistence Boundary

## Purpose

Define the Postgres persistence architecture for Veritie jobs and events, including transaction boundaries, runtime snapshot behavior, and durable ordering guarantees.

## Scope

Covers:
- Pool and health-check behavior.
- Transaction helper boundary for atomic writes.
- Jobs/events repository responsibilities and ordering contract.
- Schema and migration hardening strategy (Atlas + SQL migrations + enum domains).

Does not cover:
- HTTP handler behavior.
- Worker orchestration policy beyond persisted write boundaries.
- Provider-specific STT/LLM behavior.

## Components

- Pool layer: pgx pool construction and health checks.
- Transaction layer: rollback-on-error/panic safety and retry classification hooks.
- Repository layer: jobs and job events persistence operations.
- Schema layer: typed enum domains, FK integrity, and indexes.
- Migration layer: ordered SQL migrations with Atlas validation workflow.

## Boundaries

- Runtime config is resolved from app-owned active references at job create/rerun time and persisted as a job snapshot.
- Jobs and job_events are persisted as durable checkpoints, not per runtime progress emission.
- Event ordering for replay/read paths is deterministic by `(created_at, id)`.
- Atomic status+event transitions are supported through tx-aware repository methods.
- Migration/schema source of truth is the `server/internal/infra/db/postgres` schema+migrations with Atlas checks.

## Invariants

- Job writes should not accept client-authoritative runtime config payloads.
- Finite domain columns should stay enum-backed (fail-fast) rather than free-text.
- Cross-tenant and app/account ownership constraints should be enforced at FK/index level.
- Idempotency uniqueness should remain app-scoped for job creation.
- Repository methods should preserve deterministic list ordering for jobs/events.

## Non-Goals

- Replacing application transition validation with DB-only triggers.
- Storing every transient progress signal as a durable event.
- Using migrationless schema drift as a deployment model.
- Blocking branch-06 completion on later status vocabulary alignment (`succeeded` vs `completed`).

