# What is this for?

-   **Token & duration counters**, in `internal/domain/usage/meter.go` maintains counters.
-   **Update path**, Each time the pipeline emits a `speech.Result`, increment `usage.Meter` fields.
-   **Persistence**, `internal/infra/db/meter_repo.go` bulk-upserts into `session_usage` table when the session ends (similar pattern to log flush).
-   **Pricing layer**, Keep a Go map of price per token/minute in `internal/domain/usage/pricing.go`. `Meter.Cost()` returns AUD.
