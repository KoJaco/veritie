# 02 Refactor Spec Normalization

## Objective
Normalize refactor specifications so path mappings, scope boundaries, and architecture vocabulary match current repository reality.

## Why This Branch Exists
All migration branches depend on unambiguous documentation. Incorrect paths or scope language causes wasted implementation effort.

## In Scope
- Path correction across refactor docs
- Structure doc alignment to `/server`
- MVP scope language for excluded WebSocket/realtime transport
- Transport boundary clarification between HTTP endpoints and SSE streaming
- Canonical mapping between legacy and target roots

## Out of Scope
- Any code implementation in server or sdk
- CI workflow changes

## Implementation Plan
1. Update `refactor-map.md`:
   - replace `/old/server` references with `/old-server`
   - replace `/veritie` references with `/server`
   - add a canonical paths and scope section
2. Update `structure.md`:
   - replace `/new` root with `/server`
   - align STT provider examples to `deepgram.go` (default) + `speechmatics.go` (secondary adapter) with provider abstraction
   - include `internal/pkg/schema` and `internal/pkg/evidence`
   - mark `internal/transport/ws` as future/out-of-scope for MVP
3. Update `ground-truth.md`:
   - make transport boundaries explicit as `transport/http` and `transport/sse`
4. Run a targeted grep for stale path tokens and remove remaining mismatches.

## Deliverables
- Updated `server/docs/refactor/refactor-map.md`
- Updated `server/docs/refactor/structure.md`
- Updated `server/docs/refactor/ground-truth.md`

## Dependencies
- 01 Project Setup

## Risks and Mitigations
- Risk: over-correcting references that are intentionally historical.
- Mitigation: limit normalization to refactor docs in `server/docs/refactor/*`.
- Risk: structure docs drifting from actual tree changes later.
- Mitigation: require updates in branch 18 (docs refresh) when architecture changes.

## Verification
- Search check for stale tokens in normalized target docs (`refactor-map.md`, `structure.md`, `ground-truth.md`): `/old/server`, `/veritie`, `/new`.
- Manual consistency pass across map, structure, and ground-truth docs.

## Acceptance Gates
- No stale `/old/server`, `/veritie`, or `/new` path references remain in `refactor-map.md` and `structure.md` content (excluding intentional mention in migration instructions).
- `internal/pkg/schema` and `internal/pkg/evidence` are represented.
- Transport boundary wording is explicit and internally consistent.

## Completion Status (2026-03-11)

Branch 02 is complete for scoped deliverables.

Implemented evidence:
- Normalized mapping and scope language: `server/docs/refactor/refactor-map.md`
- Updated intended `/server` structure and package representation: `server/docs/refactor/structure.md`
- Explicit transport boundary language (`transport/http`, `transport/sse`): `server/docs/refactor/ground-truth.md`

Verification snapshot:
- Search check for stale tokens across normalized target docs passes: no `/old/server`, `/veritie`, or `/new` matches in target files.
- Manual consistency pass confirms MVP excludes WebSocket/realtime paths in canonical refactor docs.

Related documentation:
- `server/docs/architecture/server-foundation-runtime-and-ci-guardrails.md`
- `server/docs/contracts/server-foundation-runtime-and-ci-contract.md`
- `server/docs/adr/ADR-0003-server-foundation-runtime-and-ci-guardrails.md`
- `server/docs/decisions/refactor-02-05-foundation-completion-summary.md`
