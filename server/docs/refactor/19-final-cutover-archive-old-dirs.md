# 19 Final Cutover Archive Old Dirs

## Objective
Finalize migration by retiring legacy `old-server` and `old-sdk` directories only after all quality, contract, and documentation gates are satisfied.

## Why This Branch Exists
Keeping legacy and new stacks side-by-side indefinitely creates ambiguity and maintenance drag. This branch performs the controlled cutover and leaves a clear migration trail.

## In Scope
- Archive/remove `old-server` and `old-sdk`
- Finalize migration notes and compatibility matrix
- Clean up residual schma naming in public-facing surfaces (except intentional compatibility aliases)
- Perform post-cutover validation checklist across server/sdk/docs/CI

## Out of Scope
- New feature work
- Breaking removal of compatibility aliases before agreed deprecation window
- Historical rewrite of legacy implementation history beyond archival references

## Split Decision
No split required. Archive actions, naming cleanup, and validation checklist should ship together to avoid half-cutover states.

## Implementation Plan
1. Confirm pre-cutover readiness:
   - CI green for server and sdk
   - branch 18 documentation updates complete
   - compatibility policy documented for remaining aliases
2. Prepare archive strategy:
   - choose archive form (remove + git history, or move to dedicated archive location)
   - ensure any required historical references are preserved in docs
3. Execute directory retirement:
   - remove or archive `old-server`
   - remove or archive `old-sdk`
   - update references in docs/scripts/workflows to remove old-path dependencies
4. Final naming cleanup:
   - scan for `schma` in public contract surfaces and primary docs
   - retain only explicitly documented compatibility aliases and notes
5. Publish cutover artifacts:
   - final migration notes
   - compatibility matrix (old name -> new name, support window)
   - post-cutover rollback guidance (if emergency re-enable is needed)
6. Run post-cutover validation:
   - server and sdk CI
   - docs link/contract checks
   - smoke checks for key SDK and API entrypoints

## Deliverables
- Final cutover commit set removing/archiving legacy directories
- Updated docs with migration notes and compatibility matrix
- Post-cutover validation checklist/results
- Residual-name audit report (schma -> veritie)

## Dependencies
- 14 Server Tests Unit Integration E2E
- 18 Docs ADR Architecture Contracts Refresh

## Risks and Mitigations
- Risk: hidden dependency on old directories breaks CI or tooling.
- Mitigation: pre-cutover reference scan and post-cutover smoke tests.
- Risk: premature removal of compatibility paths breaks consumers.
- Mitigation: preserve documented aliases until deprecation window ends.
- Risk: incomplete naming cleanup leaves mixed branding in contracts.
- Mitigation: explicit automated scan and manual final review before merge.

## Verification
- CI passes after legacy directories are retired.
- No unresolved `schma` references in public contract/docs surfaces except documented compatibility cases.
- Migration notes and compatibility matrix are complete and linked from main docs.
- Smoke tests pass for primary API and SDK entrypoints.

## Acceptance Gates
- CI is green for server and sdk after cutover.
- Legacy directories are archived/removed with clear historical traceability.
- Public documentation and contract surfaces are Veritie-first with explicit compatibility exceptions only.
- Post-cutover validation checklist is complete and attached to the branch outcome.
