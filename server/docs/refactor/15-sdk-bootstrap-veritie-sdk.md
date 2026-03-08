# 15 SDK Bootstrap Veritie SDK

## Objective
Create the new `sdk/` package foundation for Veritie with TypeScript build/test/release scaffolding and a stable initial public surface for upcoming client port work.

## Why This Branch Exists
SDK migration cannot proceed safely without a clean package baseline. This branch establishes tooling, packaging, CI, and naming conventions before porting legacy client logic.

## In Scope
- `sdk/` package scaffold and directory layout
- TypeScript compiler config and build scripts
- Test runner and lint/format baseline for SDK package
- Initial public exports using Veritie naming conventions
- SDK CI workflow integration
- Minimal migration notes for existing schma SDK consumers

## Out of Scope
- Full client/runtime port from `old-sdk` (branch 16)
- React hook migration (`useSchma` -> `useVeritie`) (branch 17)
- Production release publishing to npm registry (can be staged after baseline validation)

## Split Decision
No split required. Package scaffolding, scripts, and CI must land together so subsequent SDK branches have a stable and enforceable baseline.

## Implementation Plan
1. Initialize SDK package structure:
   - `sdk/src` for source
   - `sdk/test` (or co-located tests) for unit coverage
   - `sdk/package.json` with scripts and metadata
2. Define TypeScript and packaging config:
   - `tsconfig.json` with strict settings
   - module output strategy (ESM/CJS policy as chosen for consumers)
   - declaration file generation
3. Set up quality tooling:
   - test framework (for example, Vitest/Jest) with baseline config
   - lint/typecheck scripts
   - formatting/check script alignment
4. Create initial export surface:
   - placeholder `VeritieSDK` entrypoint skeleton
   - public type exports for future expansion
   - explicit deprecation policy note for schma aliases (to be implemented in 16/17)
5. Add SDK CI workflow:
   - trigger on `sdk/**`
   - install deps, run typecheck/tests/build
   - upload relevant artifacts/logs if needed
6. Add consumer/bootstrap docs:
   - basic install/build usage
   - package goals and migration context

## Deliverables
- `sdk/package.json` and baseline project configs
- TS build/typecheck/test pipeline
- Initial Veritie-named public exports
- SDK CI workflow for validation
- Bootstrap documentation for contributors/integrators

## Dependencies
- 01 Project Setup
- 03 Server Go Baseline CI (pattern reference)

## Risks and Mitigations
- Risk: module format mismatch for downstream consumers.
- Mitigation: document and test selected module strategy early.
- Risk: premature API exposure creates compatibility churn.
- Mitigation: keep export surface minimal and explicitly mark unstable areas.
- Risk: CI inconsistency between local and pipeline environments.
- Mitigation: pin toolchain versions and mirror commands in package scripts.

## Verification
- Local install + `build`, `typecheck`, and `test` script execution.
- CI run on `sdk/**` changes validates baseline pipeline.
- Smoke import test confirms public entrypoint resolves correctly.

## Acceptance Gates
- SDK package installs and builds cleanly with strict TypeScript settings.
- Typecheck and unit tests pass in CI.
- Public Veritie-named entrypoint exists and is documented.
- Baseline tooling/scripts are ready for branch 16+ migration work.
