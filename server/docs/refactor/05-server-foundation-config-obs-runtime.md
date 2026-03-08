# 05 Server Foundation Config Obs Runtime

## Objective
Implement base runtime plumbing for configuration, logging, metrics/tracing abstractions, and build metadata so downstream feature branches can compose on stable primitives.

## Why This Branch Exists
All server branches depend on reliable startup behavior, configuration validation, and observability scaffolding. Implementing this early avoids repeated ad hoc boot logic.

## In Scope
- `server/internal/config`
- `server/internal/obs`
- `server/internal/runtime`
- Wiring in `server/cmd/api/main.go` and `server/cmd/worker/main.go`

## Out of Scope
- Business-domain job orchestration logic
- Provider integrations (STT/LLM)
- DB repository behavior

## Implementation Plan
1. Define config model in `internal/config/config.go`:
   - app mode, ports, log level
   - database DSN/config
   - provider credentials and toggles
   - observability toggles/endpoints
2. Implement `internal/config/validate.go` with explicit startup-fail validation.
3. Implement `internal/obs` foundation:
   - structured logger wrapper
   - metrics interface and no-op/default implementation
   - tracing initialization hooks and shutdown function
4. Expand `internal/runtime/buildinfo.go` with version, commit, build time fields.
5. Wire both command entrypoints:
   - load and validate config
   - initialize logger/metrics/tracing
   - emit startup log with build info
   - register graceful shutdown cleanup for observability resources

## Deliverables
- Config model and validation rules
- Logger/metrics/tracing initialization contract
- Runtime build metadata structure and startup emission
- Updated API and worker entrypoint wiring

## Dependencies
- 02 Refactor Spec Normalization
- 03 Server Go Baseline CI

## Risks and Mitigations
- Risk: over-engineered abstractions before real usage.
- Mitigation: keep interfaces minimal and additive.
- Risk: config sprawl with unclear ownership.
- Mitigation: group by subsystem and document required vs optional fields.
- Risk: tracing initialization failures blocking local development.
- Mitigation: explicit no-op fallback mode for disabled observability.

## Verification
- Unit tests for config validation edge cases.
- Startup smoke test for both API and worker binaries.
- Log output contains build info and selected runtime mode.

## Acceptance Gates
- API and worker startup fails fast on invalid required config.
- Baseline structured logs are emitted during startup and shutdown.
- Metrics/tracing hooks are initialized and available for later instrumentation.
- Build metadata is visible in startup logs and runtime info path (if exposed).
