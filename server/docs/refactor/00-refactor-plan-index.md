# Refactor Execution Plan Index

This directory contains the execution plan as an ordered sequence of implementation slices.

## Ordered Plan

1. [01-project-setup](./01-project-setup.md)
2. [02-refactor-spec-normalization](./02-refactor-spec-normalization.md)
3. [03-server-go-baseline-ci](./03-server-go-baseline-ci.md)
4. [04-server-db-atlas-checks](./04-server-db-atlas-checks.md)
5. [05-server-foundation-config-obs-runtime](./05-server-foundation-config-obs-runtime.md)
6. [06-server-db-postgres-core](./06-server-db-postgres-core.md)
7. [07-server-auth-principal-config-snapshot](./07-server-auth-principal-config-snapshot.md)
8. [08-server-jobs-domain-state-machine](./08-server-jobs-domain-state-machine.md)
9. [09-server-providers-stt-llm](./09-server-providers-stt-llm.md)
10. [10-server-worker-runner-orchestration](./10-server-worker-runner-orchestration.md)
11. [11-server-http-jobs-contract](./11-server-http-jobs-contract.md)
12. [12-server-sse-stream-contract](./12-server-sse-stream-contract.md)
13. [13-server-observability-and-usage](./13-server-observability-and-usage.md)
14. [14-server-tests-unit-integration-e2e](./14-server-tests-unit-integration-e2e.md)
15. [15-sdk-bootstrap-veritie-sdk](./15-sdk-bootstrap-veritie-sdk.md)
16. [16-sdk-port-core-client](./16-sdk-port-core-client.md)
17. [17-sdk-port-react-hook-and-batch-contracts](./17-sdk-port-react-hook-and-batch-contracts.md)
18. [18-docs-adr-architecture-contracts-refresh](./18-docs-adr-architecture-contracts-refresh.md)
19. [19-final-cutover-archive-old-dirs](./19-final-cutover-archive-old-dirs.md)

## Current Status Snapshot

- Refactor branch-plan documents `01` through `19` are now fully scaffolded and fleshed out.
- Implementation status must be tracked per execution branch/PR, not inferred from this index file.
- Existing CI scaffolds present in workspace:
  - `.github/workflows/server-go-baseline.yml`
  - `.github/workflows/server-db-atlas-checks.yml`
