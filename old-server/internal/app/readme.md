# What is the app dir for?

Orchestration and use-cases; the glue.

"What must be done" --> interfaces, contracts, types

### Responsibilities

-   Compose ports: receives concrete adapters via Deps{} struct.
-   Manage lifecycle: start/stop goroutines when WS opens/closes.
-   Hold session context: config from transport, prompt cache, token count.
-   Translate domain events to adapter calls (call STT until silence -> call FastParser -> call LLM if applicable -> send results back to WS client)

### Dependencies allowed

-   `domain/*` (for entities and ports)
-   `infra/*` (for adapters, by means of the interfaces passed in)
-   Concurrency helpers (`sync`, `context`, `errgroup`, `mutex`)
-   Structured logger (injected) and metrics

### Tests

-   Fast, race-detector enbaled (go text -race)
-   Use generated mocks for the ports.
