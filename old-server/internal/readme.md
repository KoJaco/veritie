# How do our layers fit together?

transport/ws
│
▼
app/pipeline
│
▼
─┐infra/sttgoogle (implements)
-├─────────────────────────────────────▶ domain/speech ◀── transport/http
─┘infra/llmgemini (implements)

-   The outer layer knows the inner layers' interfaces; inner layers never know the outers' concrete types.
-   Compile-time cycles can't happen if you follow the rule:
    -   Domain <-> (no one)
    -   app <-> domain
    -   transport <-> app
    -   infra <-> domain

### Cross-cutting tips

-   Concern - Config structs; placement - `configs/` YAML -> parsed in `cmd/...`, injected downwards.
-   Concern - Logging; placement - Build logger in `cmd/...`, pass `*slog.Logger` into `app` and `infra` via `Deps`. `Domain` NEVER logs.
-   Concern - Metrics; placement - `infra/metrics` or just Prometheus middleware in `transport/http` & counters emitted from `app`.
-   Concer - Versioning; placement - `pkg/api` for public protobuf/OpenAPI if exporting an SDK... Keeps version churn out of `internal/`
