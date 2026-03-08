# 16 SDK Port Core Client

## Objective
Port core client logic from `old-sdk/index.ts` into a modular `sdk` architecture, establishing `VeritieSDK` as the primary public client while preserving temporary compatibility aliases.

## Why This Branch Exists
The legacy SDK is monolithic and schma-branded. This branch creates a stable Veritie core client foundation before React-hook and richer integration layers are migrated.

## In Scope
- Core SDK client migration into modular files (`transport`, `models`, `types`, `errors`, `utils`)
- Rename public primary class from `SchmaSDK` to `VeritieSDK`
- Compatibility aliases for legacy names with deprecation annotations
- Alignment with locked server contracts (`/v1/jobs*`, SSE stream behavior)
- Migration notes for existing integrators

## Out of Scope
- React hook migration (`useSchma` -> `useVeritie`) (branch 17)
- Full snippet/examples overhaul (branch 17 + docs refresh branch 18)
- Hard removal of schma compatibility aliases (final cutover branch)

## Split Decision
No split required. Class rename, module decomposition, and compatibility exports should land atomically to avoid broken consumers and transient API fragmentation.

## Implementation Plan
1. Define SDK module boundaries:
   - `sdk/src/client` (core SDK class)
   - `sdk/src/transport` (HTTP + SSE clients)
   - `sdk/src/types` (public API types)
   - `sdk/src/models`/`sdk/src/parsers` (message and transcript handling)
   - `sdk/src/compat` (legacy alias exports)
2. Port and refactor core runtime logic from `old-sdk/index.ts`:
   - connection/session orchestration
   - transcript/event handling
   - batch jobs API helpers
   - error normalization
3. Rename and expose primary API surface:
   - `VeritieSDK` as canonical export
   - `SchmaSDK` alias retained with deprecation marker/docs
4. Align contract paths and payload assumptions:
   - `/v1/jobs`, `/v1/jobs/{job_id}`, `/v1/jobs/{job_id}/rerun`, `/v1/jobs/{job_id}/stream`
   - remove legacy schma endpoint defaults from primary path config
5. Add compatibility/deprecation policy:
   - explicit deprecation timeline in docs/comments
   - runtime warning policy (if enabled) scoped to dev mode only
6. Add tests:
   - core client unit tests for request building and response parsing
   - compatibility alias tests (`SchmaSDK` delegates correctly)
   - SSE stream parse/dispatch tests

## Deliverables
- Modularized SDK core implementation in `sdk/src/*`
- Canonical `VeritieSDK` export with documented API surface
- Temporary compatibility alias exports and deprecation annotations
- Migration guidance for existing SDK consumers
- Core client test coverage

## Dependencies
- 15 SDK Bootstrap Veritie SDK
- 11 Server HTTP Jobs Contract
- 12 Server SSE Stream Contract

## Risks and Mitigations
- Risk: modular refactor introduces subtle behavior regressions.
- Mitigation: preserve behavior through fixture-based tests and staged alias compatibility checks.
- Risk: consumers rely on undocumented legacy defaults.
- Mitigation: document changed defaults and keep compatibility wrappers during transition.
- Risk: API naming churn continues across branches.
- Mitigation: lock `VeritieSDK` as canonical now and confine remaining renames to branch 17 wrappers/docs.

## Verification
- Run SDK build/typecheck/test pipeline locally and in CI.
- Validate compatibility imports still work in a smoke test fixture.
- Contract tests confirm core methods target `/v1/jobs*` endpoints.

## Acceptance Gates
- `VeritieSDK` is the documented primary client export.
- Legacy `SchmaSDK` alias remains functional with deprecation notice.
- Core client functionality is modularized and test-covered.
- Endpoint/path behavior aligns with current server contract.
