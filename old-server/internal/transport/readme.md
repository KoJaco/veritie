## What is the transport dir for?

Transport is a thin, replaceable shell; could be gRPC or CLI without touching domain or infra.

### Purpose

-   Translate protocol details to application calls and vice-versa.
-   The interface between protocol and application calls

### Contains

-   Websocket upgrade & frame loop
-   HTTP ops endpoint (healthz and metrics)
-   Auth middleware that extracts the token and attaches domain/auth.Principal to the context.Context

### Does NOT contain

-   Business rules (session limits, billing, prompt logic)
-   Direct calls to STT or LLM. These go through app ports.
