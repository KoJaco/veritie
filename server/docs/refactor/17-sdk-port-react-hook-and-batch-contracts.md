# 17 SDK Port React Hook and Batch Contracts

## Objective
Port the React hook layer to Veritie naming and complete SDK contract alignment for batch/auth helper APIs, while preserving controlled backward compatibility.

## Why This Branch Exists
After core client migration, integrators need ergonomic React usage and clear endpoint contract alignment. This branch completes consumer-facing migration paths with minimal breakage.

## In Scope
- Rename `useSchma` to `useVeritie`
- Preserve temporary hook wrappers for compatibility
- Align batch helper paths/config with `/v1/jobs*` server contract
- Align token/auth helper behavior with HTTP+SSE model
- Update SDK examples/snippets and migration notes

## Out of Scope
- Final removal of deprecated schma aliases
- Frontend product integration work outside SDK package
- Server contract changes

## Split Decision
No split required. Hook migration, endpoint alignment, and compatibility wrappers should ship together to avoid mixed integration guidance and consumer confusion.

## Implementation Plan
1. Port React hook implementation:
   - move `useSchma` logic to `useVeritie`
   - preserve state/event semantics from legacy behavior where still valid
   - expose typed return contract aligned with `VeritieSDK`
2. Implement compatibility wrappers:
   - keep `useSchma` as wrapper alias over `useVeritie`
   - document deprecation timeline and migration steps
3. Align batch/auth helper contracts:
   - update default batch paths to `/v1/jobs` contract family
   - ensure status/list/rerun helpers map to current server endpoints
   - validate auth header/token helper flows for HTTP and SSE access patterns
4. Update examples/snippets/docs:
   - replace schma-first snippets with Veritie-first snippets
   - include compatibility examples for transition period
   - document migration checklist for existing integrators
5. Add tests:
   - hook behavior parity tests (connection state, transcript updates, batch helpers)
   - compatibility wrapper tests (`useSchma` parity)
   - endpoint path/config resolution tests

## Deliverables
- `useVeritie` hook implementation and exports
- Backward-compatible `useSchma` wrapper with deprecation guidance
- Batch/auth helper alignment with `/v1/jobs*` and SSE-compatible flows
- Updated examples/snippets for integrators
- Hook and compatibility test coverage

## Dependencies
- 16 SDK Port Core Client
- 11 Server HTTP Jobs Contract
- 12 Server SSE Stream Contract

## Risks and Mitigations
- Risk: hook migration changes runtime behavior unexpectedly.
- Mitigation: parity-focused tests against known legacy scenarios.
- Risk: stale snippet/docs continue promoting schma naming.
- Mitigation: replace defaults with Veritie naming and clearly label compatibility mode.
- Risk: auth helper assumptions drift from server behavior.
- Mitigation: contract tests against current server auth and stream access expectations.

## Verification
- SDK test suite covers hook state transitions and batch/auth helpers.
- Example app smoke test validates `useVeritie` happy path.
- Compatibility fixture validates existing `useSchma` usage still runs.

## Acceptance Gates
- `useVeritie` is the canonical React hook export.
- `useSchma` wrapper remains functional and explicitly deprecated.
- Batch helper APIs resolve to `/v1/jobs*` contracts with tested behavior.
- Updated docs/snippets prioritize Veritie naming and migration clarity.
