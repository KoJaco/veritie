# ADR-0003: Server Foundation Runtime and CI Guardrails

## Status

Accepted

## Date

2026-03-11

## Context

Refactor branches 02-05 establish the baseline needed before higher-level server behavior can be implemented safely:
- path/scope normalization in refactor docs
- baseline Go CI checks
- Atlas schema/migration CI checks
- runtime config/observability/build-info bootstrap in API and worker entrypoints

Without these guardrails, later branches risk inconsistent path conventions, startup misconfiguration drift, and reduced CI confidence.

## Decision

1. Keep refactor path/scope normalization as canonical in `server/docs/refactor`.
2. Enforce baseline server Go CI checks on server changes.
3. Enforce Atlas schema/migration dry-run checks on DB path changes.
4. Require fail-fast runtime config validation before API/worker startup.
5. Standardize startup observability hooks and build metadata logging in both entrypoints.

## Alternatives Considered

- **Delay CI guardrails until later branches** — rejected due to compounding regression risk.
- **Only run tests in baseline CI (skip vet/race/coverage)** — rejected due to lower signal for refactor safety.
- **Allow best-effort startup with invalid config** — rejected due to hidden runtime failure modes.
- **Keep legacy path aliases in normalized docs** — rejected due to implementation ambiguity.

## Consequences

- **Pros**
- Consistent baseline quality signals for all downstream server branches.
- Deterministic startup behavior and clearer operational diagnostics.
- Reduced schema drift risk through Atlas dry-run checks.
- Clear canonical mapping between legacy and refactor paths.
- **Cons**
- Slower CI feedback loop due to stricter checks.
- Higher initial setup burden for local/dev env configuration.
- Requires ongoing maintenance of workflow/tooling versions.
- **Follow-ups / TODOs** (optional)
- Periodically reassess CI runtime/cost and split jobs if needed.
- Keep docs refreshed in branch 18 when foundational behavior evolves.

## References

- Related docs/contracts: `server/docs/contracts/server-foundation-runtime-and-ci-contract.md`, `server/docs/architecture/server-foundation-runtime-and-ci-guardrails.md`
- Related decision note: `server/docs/decisions/refactor-02-05-foundation-completion-summary.md`
- Related refactor docs: `server/docs/refactor/02-refactor-spec-normalization.md`, `server/docs/refactor/03-server-go-baseline-ci.md`, `server/docs/refactor/04-server-db-atlas-checks.md`, `server/docs/refactor/05-server-foundation-config-obs-runtime.md`
- Issue: #
- PR: #
