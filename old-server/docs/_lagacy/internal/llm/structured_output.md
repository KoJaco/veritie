## LLM Modes: Functions vs Structured Output (MVP Strategy)

### Purpose

Define how real-time LLM integration supports either function parsing or structured output during an audio session, not both concurrently, for simplicity, determinism, and cost control.

---

### Decision

-   Exactly one LLM mode is active per audio session:
    -   Mode A: Function parsing (tools + function-calling).
    -   Mode B: Structured output (JSON schema–constrained generation).
-   Mode can switch between audio sessions on the same WebSocket connection.
-   SDK guarantees client config includes either `function_config` or `structured_config`, never both.

Rationale:

-   Prevents conflicting provider configs (tools vs schema).
-   Simplifies caching, dynamic updates, and safe swap protocol.
-   Halves token/call costs vs dual-mode concurrency.
-   Avoids concurrency/race conditions between two parallel LLM outputs.

---

### Protocol

Client → Server (Config):

-   `function_config` OR `structured_config` (mutually exclusive).
-   If both are provided, server returns an error and ignores the config.

Client → Server (Dynamic updates):

-   `dynamic_config_update` for functions.
-   `dynamic_structured_update` for structured output.

Server → Client:

-   `functions` messages containing merged function calls (Mode A).
-   `structured` messages containing incremental `delta` and a `final` object (Mode B).
-   `config_update_ack` after dynamic updates.
-   `error` for invalid/mixed configs.

SDK union typing (example):

```ts
type ClientConfig =
    | {
          type: "config";
          function_config: FunctionConfig;
          structured_config?: undefined;
      }
    | {
          type: "config";
          structured_config: StructuredConfig;
          function_config?: undefined;
      };
```

---

### Session Lifecycle

Connection:

-   WebSocket handler validates exclusivity (rejects mixed config).
-   Sets connection-level LLM mode to “functions” or “structured” (or “none” for STT-only).

Audio session (audio_start → audio_stop):

-   On `audio_start`, pre-warm LLM cache for the chosen mode with latest config.
-   Pipeline runs STT → final transcripts trigger LLM slot per mode.
-   On `audio_stop`, pipeline flushes “final” outputs and stops.
-   Between audio sessions, clients may switch mode via a new config.

---

### Pipeline Integration

Config:

-   Pipeline holds exactly one of:
    -   `FuncCfg *speech.FunctionConfig`
    -   `StructuredCfg *speech.StructuredConfig`

Slots on final transcripts:

-   Mode A (functions):
    -   Debounce by `FuncCfg.UpdateMs`.
    -   Build dynamic prompt using transcript + prior calls.
    -   Call `LLM.Enrich(...)`.
    -   Merge/update function calls; emit `functions` message if changed.
-   Mode B (structured):
    -   Debounce by `StructuredCfg.UpdateMS`.
    -   Build dynamic prompt using transcript + prior object.
    -   Call `LLM.GenerateStructured(...)`.
    -   Validate/prune against schema; deep-merge into current object.
    -   Emit `structured` delta if changed; emit `final` on session end.

Deep-merge (structured):

-   Start simple: recursive merge of maps; arrays replace by default.
-   Ignore null deletes initially; add strategies later as needed.

---

### LLM Adapter (Gemini)

Single adapter with two methods:

-   Functions:
    -   Configure tools + function-calling mode.
    -   System prompt for function extraction.
    -   Returns `[]FunctionCall` + usage.
-   Structured:
    -   Configure response schema + `application/json` mime type.
    -   System prompt guiding schema-constrained incremental filling.
    -   Returns `map[string]any` + usage.

Separate session state and cache key per mode; do not toggle a single session between tools and schema mid-stream.

---
