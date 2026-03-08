# 18 Docs ADR Architecture Contracts Refresh

## Objective
Refresh and lock architecture, ADR, and contract documentation so it accurately reflects implemented server and SDK behavior and serves as the canonical operational reference.

## Why This Branch Exists
After core implementation branches, undocumented or stale decisions become the main source of onboarding friction and integration mistakes. This branch turns implementation knowledge into durable, auditable docs.

## In Scope
- ADR updates/additions for irreversible technical decisions
- Architecture document updates for runtime boundaries and data flow
- API contract docs for jobs + SSE
- SDK contract docs for `VeritieSDK`/`useVeritie` and deprecation policy
- Cross-linking and template consistency across docs tree

## Out of Scope
- New feature implementation
- Major API redesign beyond documenting shipped behavior
- Historical ADR rewrites except where explicitly required for correction

## Split Decision
No split required. ADR, architecture, and contracts must be published together to avoid inconsistent reference states across teams.

## Implementation Plan
1. Build documentation delta checklist from implemented branches:
   - jobs lifecycle/state-machine behavior
   - auth principal + server-side config snapshot model
   - Deepgram-default STT and adapter abstraction strategy
   - HTTP jobs contract and SSE stream contract
   - observability and usage event model
   - SDK naming migration and compatibility wrappers
2. Update ADRs under `server/docs/adr`:
   - add ADR(s) for major irreversible decisions not already captured
   - include context, decision, tradeoffs, and consequences
3. Update architecture docs under `server/docs/architecture`:
   - service/module boundaries
   - runner and event flow
   - auth/config resolution path
   - SSE flow and replay semantics
4. Update contract docs under `server/docs/contracts`:
   - jobs endpoint request/response envelopes
   - SSE event envelope/cursor rules
   - SDK surface expectations and migration notes
5. Normalize doc consistency:
   - ensure all new docs use template structure
   - update links from `00-refactor-plan-index.md` and related files
   - remove stale schma-centric examples where no longer valid
6. Add docs quality checks:
   - broken-link scan
   - consistency pass on terminology (`Veritie`, `SSE`, `/v1/jobs*`, `config_id` resolution)

## Deliverables
- ADR additions/updates under `server/docs/adr`
- Architecture updates under `server/docs/architecture`
- Contract updates under `server/docs/contracts`
- Updated migration/deprecation documentation for SDK naming transition
- Validated doc link integrity and terminology consistency

## Dependencies
- 11 through 17

## Risks and Mitigations
- Risk: docs lag behind implementation details.
- Mitigation: derive updates from implementation branch outputs and tests, not memory.
- Risk: duplicate or conflicting decision records.
- Mitigation: include a decision inventory step before adding new ADRs.
- Risk: stale schma naming persists in public examples.
- Mitigation: targeted scan and explicit migration note section for remaining compatibility aliases.

## Verification
- Manual and automated link checks pass.
- Contract examples match tested API payload shapes.
- ADRs include clear decision rationale and consequences.
- Terminology scan confirms Veritie-first naming with intentional compatibility exceptions documented.

## Acceptance Gates
- All public API and SDK contracts are documented and current.
- ADRs capture key irreversible decisions and tradeoffs.
- Architecture docs reflect actual runtime boundaries and data flow.
- Cross-links/templates are consistent and pass validation checks.
