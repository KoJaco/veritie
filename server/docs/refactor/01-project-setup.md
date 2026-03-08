# 01 Project Setup

## Objective
Establish repository conventions, guardrails, and shared context so all refactor branches can execute with low ambiguity.

## Why This Branch Exists
Without setup standards, later branches will drift on paths, naming, quality gates, and branch boundaries. This branch creates the operating baseline.

## In Scope
- Root-level workspace orientation and directory map
- Baseline ignore rules for generated artifacts and secrets
- Governance rules for branch scope, quality gates, and cutover policy
- Refactor source-of-truth linkage from root docs

## Out of Scope
- Server runtime implementation
- SDK implementation
- CI workflow logic (covered in 03 and 04)

## Implementation Plan
1. Add/update root `README.md` with canonical workspace map and source-of-truth references.
2. Add/update root `.gitignore` with patterns for Go, Node, Python, editor, and secret files.
3. Create/update `server/docs/refactor/refactor-governance.md` with:
   - scope boundaries
   - branch isolation rules
   - required checks per branch type
   - cutover completion conditions
4. Ensure governance references align with `server/docs/refactor/00-refactor-plan-index.md`.

## Deliverables
- Root `README.md`
- Root `.gitignore`
- `server/docs/refactor/refactor-governance.md`

## Dependencies
- None

## Risks and Mitigations
- Risk: convention docs become stale as plan evolves.
- Mitigation: include explicit ownership in governance doc and update check in docs-refresh branch (18).
- Risk: ignore rules accidentally hide required source files.
- Mitigation: keep patterns conservative and avoid broad `**/*`-style ignores.

## Verification
- Manual doc read-through for consistency with plan index.
- Confirm no critical tracked file classes are newly ignored.

## Acceptance Gates
- Core repository conventions are documented and discoverable.
- Refactor source-of-truth docs are linked from root context.
- Governance defines branch boundaries, quality gates, and final cutover criteria.
