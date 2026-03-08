# What does this domain contain?

The pure core; rules of the application as a whole.

### What does it contain?

1.  Entities / value objects - durable concepts in the business language (e.g. session.Session, speech.Transcript, usage.Meter)
2.  Aggregates / State machines - methods that protect invariants (e.g. (\*Session).Advance(event))
3.  Ports (interfaces) - What the domain NEEDS from the outside world (STTClient, FastParser, LLM, MeterRepo, AuthValidator)
4.  Errors - sentinel vars used for branching logic

### What does it not contain?

1. net/http, database/sql, grpc, SDK structs, env look-ups, loggers.
2. Concrete struct types from other layers.

### Import direction

`domain` may import only:

-   `std-lib` + other sub-packages of `internal/domain`

\*\*\* Everything else imports INTO it.

### Tests

-   Table-driven tests, no mocks needed -- just call methods and assert state.
