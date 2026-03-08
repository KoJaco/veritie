# Project structure

This document describes the intended structure under `/server`.

```
/server
  cmd/
    api/
      main.go
    worker/
      main.go
  internal/
    config/
      config.go
      validate.go
    obs/
      logger.go
      metrics.go
      tracing.go
    app/
      jobs/
        service.go
        runner.go
        policy.go
        model.go
        interfaces.go
      sse/
      health/
    transport/
      http/
        server.go
        routes.go
        middleware.go
        handlers/
          jobs.go
          stream.go
      messaging/
        consumer.go
        producer.go
    infra/
      db/
        postgres/
          pool.go
          tx.go
          migrations/
          jobs_repo.go
          events_repo.go
      redis/
        client.go
      providers/
        stt/
          deepgram.go
          speechmatics.go
          provider.go
          transcript.go
        llm/
          gemini.go
      storage/
        s3.go
    pkg/
      schema/
      evidence/
    runtime/
      buildinfo.go
```

## Summary

- `cmd/`: process entrypoints for API and worker binaries.
- `internal/config/`: configuration models and validation helpers.
- `internal/obs/`: logging, metrics, and tracing abstractions.
- `internal/app/`: core domain logic; jobs orchestration plus optional SSE/health helpers.
- `internal/transport/`: HTTP/SSE transport and optional messaging layers.
- `internal/infra/`: infrastructure adapters for databases, caches, providers, and storage.
- `internal/pkg/`: shared library packages for schema and evidence helpers.
- `internal/runtime/`: build-time metadata for versioning.

## Optional or future-facing areas

- `cmd/worker/`: use when splitting worker processes from API.
- `internal/app/sse/`: used for SSE broadcasting/stream management.
- `internal/transport/ws/`: explicitly out of scope for MVP batch refactor; keep only as future area.
- `internal/transport/messaging/`: messaging adapters (NATS/Kafka, etc.).
- `internal/infra/redis/`: Redis client integration.
- `internal/infra/db/postgres/migrations/`: in-repo migrations, if desired.
