# What is 'infra' for?

Adapters to the real world.

### What does it contain?

One sub-package per external service or technology. E.g.:

-   `db`: usage.MeterRepo, migration helpers
-   `authjwt`: auth.Validator, parses JWT
-   `fastparser`: speech.FastParser (loads distilBERT)
-   `fasttext`: speech.FastTextParser (loads fastText)
-   `sttdeepgram`: speech.STTClient (deepgram STT)
-   `sttgoogle`: speech.STTClient (Google STT)
-   `llmgemini`: speech.LLM (HTTP to Gemini endpoint)

### Dependencies allowed

-   `std-lib`, third-party clients (pgx, google.golang.org/api/speech/v1),
-   import remote interfaces from `domain`, NEVER the other way.
-   May also import shared logging / metrics and packages (zap, slog)

### Tests

Mostly integration-style with fakes or wiremock; use `go:build !integration` tags if they hit the network.
