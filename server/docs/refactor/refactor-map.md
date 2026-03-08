# Refactor Mapping (Old -> Veritie)

This mapping documents reusable components from `/old-server` and their target homes in `/server`.

## Canonical Paths + Scope

- Legacy source roots: `/old-server`, `/old-sdk`
- Refactor targets: `/server`, `/sdk`
- MVP refactor scope: batch pipeline, jobs API, SSE streaming, DB, auth, providers, observability
- Out of MVP refactor scope: WebSocket transport and real-time chunk streaming

## Reuse + refactor targets

- Logging
    - Source: `/old-server/internal/pkg/logger`
    - Target: `/server/internal/obs/logger.go`
    - Notes: keep lightweight logger + spans; remove realtime-specific text where needed.

- Metrics
    - Source: `/old-server/internal/pkg/metrics`
    - Target: `/server/internal/obs/metrics.go`
    - Notes: keep simple counters/summaries; used by job lifecycle + HTTP middleware.

- Usage collection
    - Source: `/old-server/internal/app/usage`
    - Target: `/server/internal/app/usage`
    - Notes: keep batch-mode path; remove websocket-specific CPU sampling.

- Auth + principal
    - Source: `/old-server/internal/domain/auth`, `/old-server/internal/transport/http/middleware/auth.go`, `/old-server/internal/infra/auth`
    - Target: `/server/internal/app/auth`, `/server/internal/transport/http/middleware`
    - Notes: principal must include LLM classification config; auth against DB and return principal.

- Connection management
    - Source: `/old-server/internal/infra/connection`, `/old-server/internal/infra/db/postgres.go`
    - Target: `/server/internal/infra/db/postgres` and `/server/internal/infra/connection` (if still useful)
    - Notes: keep DB pooling helpers and per-request context wiring. Websocket connection pool not reused.

- STT/LLM providers
    - Source: `/old-server/internal/infra/sttgoogle`, `/old-server/internal/infra/llmgemini`
    - Target: `/server/internal/infra/providers/stt`, `/server/internal/infra/providers/llm`
    - Notes: adapt to batch-only interfaces and short-audio constraints.

- Supabase schema + migrations
    - Source: `/old-server/internal/infra/db/schema.hcl`, `/old-server/internal/infra/db/supabase/migrations`, `/old-server/atlas.hcl`
    - Target: `/server/internal/infra/db/postgres` + root configs
    - Notes: keep same tooling approach; update schema for batch jobs, principal config, and usage.

- Test utilities
    - Source: `/old-server/internal/testutil`
    - Target: `/server/internal/testutil`
    - Notes: reuse fixtures/helpers as appropriate.

## Excluded (realtime)

- WebSocket handlers, message loops, connection pool / session manager, streaming pipeline.
- Any real-time audio chunking or incremental parsing logic.

## Batch-only additions

- Job lifecycle + orchestration in `/server/internal/app/jobs`.
- SSE broadcaster and event model in `/server/internal/app/sse` and `/server/internal/transport/http/handlers`.
