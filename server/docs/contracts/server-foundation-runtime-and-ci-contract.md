# Contract: Server Foundation Runtime and CI Contract

## Purpose

Define the baseline contract established by refactor branches 02-05 for server runtime bootstrap and CI safety checks.

## Scope

Included:
- Runtime bootstrap contract for config validation, logging/tracing/metrics initialization, and build-info startup emission.
- Baseline Go CI and Atlas CI expectations.
- Spec-normalization guardrails for canonical path/scope terminology.

Out of scope:
- Jobs/SSE transport payload contracts.
- Auth principal and runtime snapshot execution semantics.
- Provider orchestration and worker stage policies.

## Versioning

- **Current version:** v1
- **Compatibility:** Backward compatible (internal engineering contract)
- **Change policy:** Major bump if required startup checks or mandatory CI gates change incompatibly.

## Definitions

- **Foundation bootstrap:** Process startup path before domain handlers run.
- **Fail-fast validation:** Startup rejection on invalid required runtime configuration.
- **Baseline CI gate:** Mandatory quality checks for server module changes.
- **Atlas drift check:** DB schema/migration dry-run validation in CI.

## Contract Shape (Conceptual)

### Required fields

- Runtime config must include:
  - `service`, `app.mode`, `app.http_port`, `app.worker_concurrency`
  - `database.dsn`
  - selected provider identifiers and corresponding credentials
  - observability log level and tracing endpoint when tracing is enabled
- CI workflows must include:
  - server baseline checks (`test`, `vet`, `race`, coverage, module tidiness)
  - DB Atlas checks (schema dry-run; conditional migration replay dry-run)

### Optional fields

- Provider key validation bypass in non-production modes for local scaffolding.
- SQL migration replay step (conditionally skipped when migration files are absent).

## Invariants (Must Always Hold)

- API and worker entrypoints should fail startup when required config is invalid.
- Startup logs should include build metadata and runtime mode/process identity.
- Baseline Go CI should run on `server/**` changes and fail on module/test/vet/race/coverage regressions.
- Atlas DB CI should run on DB path changes and fail on schema dry-run regressions.
- Canonical refactor docs should avoid stale `/old/server`, `/veritie`, and `/new` path references in normalized target docs.

## Error Handling

- Invalid config values return deterministic validation errors at startup.
- Unsupported log level/provider selections are rejected during validation.
- CI failures block merges and must be corrected in code/workflow config.
- Missing migration SQL files are handled explicitly via skip messaging in Atlas replay step.

## Examples

### Minimal valid example

```json
{
  "runtime_startup": {
    "service": "api",
    "mode": "development",
    "database_dsn_set": true,
    "stt_provider": "deepgram",
    "llm_provider": "gemini",
    "log_level": "info",
    "build_info": {
      "version": "dev",
      "commit": "unknown",
      "build_time": "unknown"
    }
  },
  "ci": {
    "server_go_baseline": "pass",
    "server_db_atlas_checks": "pass"
  }
}
```

### Invalid example

```json
{
  "runtime_startup": {
    "service": "worker",
    "mode": "production",
    "database_dsn_set": false,
    "stt_provider": "deepgram",
    "llm_provider": "gemini",
    "disable_provider_key_checks": true
  }
}
```

Expected handling:
- Startup fails due to missing required DB DSN.
- Startup fails because provider-key-check bypass is disallowed in production mode.

### Operational notes

- `server-go-baseline.yml` also enforces sqlc generated-code drift checks for Postgres dbgen output.
- Atlas workflow uses Postgres service container and validates both schema and migration replay compatibility.

### References

- Related ADRs: `server/docs/adr/ADR-0003-server-foundation-runtime-and-ci-guardrails.md`
- Related architecture: `server/docs/architecture/server-foundation-runtime-and-ci-guardrails.md`
- Related decisions: `server/docs/decisions/refactor-02-05-foundation-completion-summary.md`
- Related refactor docs: `server/docs/refactor/02-refactor-spec-normalization.md`, `server/docs/refactor/03-server-go-baseline-ci.md`, `server/docs/refactor/04-server-db-atlas-checks.md`, `server/docs/refactor/05-server-foundation-config-obs-runtime.md`
- Issue/PR: #
