---
audience: engineers
status: draft
last_verified: 2024-03-21
---

# Session Manager

## You'll learn

-   Use-cases (`Start`, `Advance`, `AddUsage`, `Snapshot`, `Close`).
-   Invariants, retries, and idempotency.
-   Sequence across WS events.

## Where this lives in hex

App layer orchestration.

## Ports and Methods

-   [ ] TODO: Document Start method and parameters
-   [ ] TODO: Document Advance method and parameters
-   [ ] TODO: Document AddUsage method and parameters
-   [ ] TODO: Document Snapshot method and parameters
-   [ ] TODO: Document Close method and parameters

## Sequence (connect → close)

```mermaid
sequenceDiagram
  participant C as Client
  participant T as Transport(WS)
  participant A as App(Session Manager)
  participant STT as STT Provider
  participant LLM as LLM
  participant DB as DB

  C->>T: Connect + API Key
  T->>A: Authenticate(AppID)
  A->>DB: Load AppSettings (cached)
  T->>A: AudioChunk / Control
  A->>STT: Stream
  STT-->>A: Transcript delta
  A->>LLM: Function finalization (policy)
  A-->>T: Draft/Final events
  A->>DB: Usage + EventLog
  C-->>T: Close
  T->>A: Close(Session)
```

## Failure Modes & Policies

-   [ ] TODO: Document rate limiting configuration
-   [ ] TODO: Document backpressure mechanisms
-   [ ] TODO: Document retry policies for STT/LLM/DB
-   [ ] TODO: Document idempotency key implementation
