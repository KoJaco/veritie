# 03 Server Go Baseline CI

## Objective
Establish mandatory baseline CI checks for the `server` Go module to prevent regressions as refactor branches land.

## Why This Branch Exists
The server tree is being rebuilt incrementally; baseline checks must fail fast on compile/test/module drift before deeper branch work begins.

## In Scope
- GitHub Actions workflow for server Go checks
- Trigger filtering to server-relevant paths
- Build safety checks: tests, vet, race, coverage, module tidiness

## Out of Scope
- DB schema/migration validation (covered in 04)
- SDK CI pipeline (covered in 15)

## Implementation Plan
1. Create/update `.github/workflows/server-go-baseline.yml`.
2. Configure triggers for:
   - pull requests touching `server/**`
   - pushes to `main` touching `server/**`
3. Set job working directory to `server`.
4. Add steps:
   - setup Go from `server/go.mod`
   - dependency download
   - module tidiness verification (`go mod tidy` + diff check)
   - `go test ./...`
   - `go vet ./...`
   - `go test -race ./...`
   - `go test -coverprofile=coverage.out ./...`
   - upload coverage artifact

## Deliverables
- `.github/workflows/server-go-baseline.yml`

## Dependencies
- 01 Project Setup

## Risks and Mitigations
- Risk: workflow fails on empty or early-stage package tree due to future placeholders.
- Mitigation: keep checks standard and fail loudly to force valid stubs.
- Risk: module diff check fails on missing `go.sum` in early stages.
- Mitigation: conditional check logic for `go.sum` presence.

## Verification
- Workflow YAML lint (local/manual review).
- Dry run via PR touching `server/` paths.

## Acceptance Gates
- CI runs automatically on server-related changes.
- `go test ./...`, `go vet ./...`, and `go test -race ./...` are enforced.
- Coverage profile artifact is generated on successful runs.
