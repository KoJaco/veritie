# Veritie Workspace

This workspace hosts the Veritie refactor in parallel with legacy sources.

## Directory map

- `server/`: new Go server target (batch-first architecture)
- `sdk/`: new TypeScript SDK target
- `old-server/`: legacy Go server reference
- `old-sdk/`: legacy TypeScript SDK reference
- `frontend/`: frontend workspace (not in current refactor implementation scope)

## Refactor source of truth

- `server/docs/refactor/ground-truth.md`
- `server/docs/refactor/refactor-map.md`
- `server/docs/refactor/structure.md`

## Branching model

Use fine-grained, pauseable branches aligned to the documented branch sequence in `server/docs/refactor/refactor-governance.md`.
