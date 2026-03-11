# Architecture — Server Foundation Runtime and CI Guardrails

## Purpose

Define the foundational architecture introduced across refactor branches 02-05: normalized repo-spec boundaries, baseline server CI guardrails, DB Atlas drift checks, and runtime config/observability bootstrap contracts.

## Scope

Covers:
- Refactor-document path/scope normalization for `/server`.
- Server Go baseline CI expectations.
- Atlas schema/migration CI checks.
- Runtime bootstrap layers (`config`, `obs`, `runtime`) used by API and worker entrypoints.

Does not cover:
- Jobs domain lifecycle/state machine behavior.
- Auth principal resolution semantics.
- Provider/runtime pipeline execution stages.

## Components

- Spec normalization layer: `refactor-map.md`, `structure.md`, `ground-truth.md`.
- Go baseline CI workflow: compile/test/vet/race/coverage + module hygiene checks.
- DB Atlas CI workflow: schema dry-run and conditional migration replay dry-run.
- Runtime foundation packages:
  - `internal/config` for typed env config and validation.
  - `internal/obs` for logger/metrics/tracing hooks.
  - `internal/runtime` for build metadata.
- Entrypoint bootstrap: `cmd/api/main.go`, `cmd/worker/main.go`.

## Boundaries

- CI guardrails are branch-wide quality gates and should fail fast on drift.
- Runtime entrypoints should only start after config validation succeeds.
- Observability initialization should be available in both API and worker processes, with safe no-op behavior where backend wiring is intentionally deferred.
- Refactor docs remain canonical for path/scope intent and should avoid stale legacy path aliases.

## Invariants

- Server CI should enforce `go test`, `go vet`, race checks, and coverage profile generation.
- DB CI should enforce schema-file presence and Atlas dry-run checks.
- Runtime config should reject invalid modes/providers/ports and missing required secrets (subject to explicit local-dev bypass controls).
- Startup logs should include build metadata (`version`, `commit`, `build_time`) and process identity.

## Non-Goals

- Shipping provider-specific runtime integrations in foundation branches.
- Replacing later branch contract docs for jobs/SSE/auth.
- Conflating normalization docs with implementation behavior contracts.

