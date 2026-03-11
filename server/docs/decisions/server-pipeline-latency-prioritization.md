# Decision Note: Server Pipeline Latency Prioritization

## Date

2026-03-11

## Summary

Veritie should prioritize latency work according to observed user-facing impact in the short-audio MVP pipeline, focusing first on upload and model stages before optimizing comparatively small control-plane paths.

## Decision

Use the following ranked optimization order:

1. Upload time
2. STT latency
3. Structured extraction latency
4. Queue/worker pickup latency
5. Job bootstrap and upload finalize latency

## Rationale

- Upload is typically the largest perceived delay and is network-bound.
- STT is the first major post-upload processing wait and often externally variable.
- Extraction latency scales with schema/context complexity and can grow quickly.
- Queue pickup is usually smaller but becomes visible under cold or saturated workers.
- Bootstrap/finalize are important, but over-optimizing them first yields lower end-user impact.

## Impact

- Prioritizes engineering effort toward highest-impact latency drivers.
- Reinforces app runtime caching, regional colocation, and short-audio ingest constraints.
- Guides default orchestration choices: snapshot-on-create, checkpoint batching, and sequential extraction -> tool suggestion.
- Sets expectation that SSE progress clarity is part of perceived performance even when raw stage latency remains.

## Follow-ups

- [ ] Track stage-level p50/p95 for upload, STT, extraction, queue pickup, bootstrap, and finalize.
- [ ] Add latency budget dashboard and alert thresholds for upload-complete -> processing-start.
- [ ] Reevaluate multipart upload threshold after short-audio MVP metrics stabilize.
- [ ] Reevaluate parallel tool suggestion after extraction quality/latency baselines are established.

## References

- Related ADR/Contracts: `server/docs/adr/ADR-0001-server-pipeline-runtime-boundary.md`, `server/docs/contracts/server-pipeline-runtime-contract.md`
- Related architecture: `server/docs/architecture/server-pipeline-core-flow.md`
- Related refactor docs: `server/docs/refactor/ground-truth.md`, `server/docs/refactor/13-server-observability-and-usage.md`
- Issue: #
- PR: #
