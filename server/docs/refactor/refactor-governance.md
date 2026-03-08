# Refactor Governance

This document defines execution conventions for the Veritie refactor.

## Canonical scope

- Legacy references stay in `old-server/` and `old-sdk/` until final cutover.
- Active implementation targets are `server/` and `sdk/`.
- MVP scope is batch pipeline, jobs API, SSE, auth, DB, providers, observability, and docs.
- WebSocket transport and real-time audio streaming are out of MVP scope.

## Branch conventions

- Branches are isolated, pauseable work slices with explicit acceptance gates.
- Keep branch diffs focused to one sequence step from the approved branch plan.
- Do not mix SDK and server contract changes without updating corresponding contract docs.

## Required quality gates per branch

- Server branches: compile, `go test ./...`, `go vet ./...`, race on touched packages.
- DB branches: schema checks plus migration replay checks.
- SDK branches: typecheck, unit tests, and compatibility tests for deprecated exports.
- Docs branches: links, path accuracy, and cross-reference consistency.

## Completion criteria for final cutover

- CI is green across server and SDK workflows.
- Public API no longer exposes `schma` naming (except explicit compatibility aliases and deprecation notes).
- `old-server/` and `old-sdk/` are archived or removed with migration notes.
