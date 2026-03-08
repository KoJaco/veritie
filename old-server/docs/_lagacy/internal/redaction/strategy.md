# Strategy implement for PHI, PCI, and PII

## PCI handling

-   Create a separate PCI redaction service.
-   defaults to redacting pre-llm (Always redact PCI). Want nothing to do with storing people's credit card info.
-   Assumption: PCI is unlikely
-   PCI data (card numbers, CVV, CSV, expiry)

## PII handling

-   Default: allow PII through LLM (with vendor guarantees, documented in ToS/privacy)
-   If stript mode is enabled, redact PII before hitting LLM (note: performance / context quality may degrade)
-   Regardless of config, redact before DB.
-   Allow users to redact pre-llm via config (as with PCI)

## PHI handling

\*\*\* Important

-   see Business Associate Potential document. With a business associate agreement (BAA) in place, we can send PHI to Gemini ONLY when using the Vertex AI backend. This requires a refactor of the V2 services in our infra.

Mode A (POST MVP)

Context: Signed a BAA with Google, using Gemini via Vertex AI Generative AI (HIPAA-compliant), refactored LLM infra v2 to support vertex backend via env or config flag.

PHI treatment:

-   PHI can legally flow to the LLM (no need for pre-LLM redaction, disable service)

Pipeline:

-   Audio -> STT -> transcript
    --> (optional: tokensization layer for PHI, reversible internally)
    --> vertex AI gemini
    --> Structured output / function arguments
    --> (Optional: de-tokenization before UI)

-   Storage and Logs
    --> Allowed to store PHI in DB (must encrypt at rest, access control, audit)
    --> Redacted/placeholder versions recommended for logs and analytics

-   Trade-offs:
    --> Simplies real-time flow
    --> maintains transcript integrity, potentially higher accuracy.
    --> Higher cost and setup overhead (vertex, deployement, BAA process)

Mode B (PRE MVP, supported Post mvp too)

Context: using public LLM endpoints

PHI Treatment

-   PHI must be redacted or tokenized pre-LLM.
-   Replace sensitive spans with placeholders (e.g., <PHI:NAME#1>).
-   Maintain secure mapping to restore values downstream (UI)

Pipeline:

-   Audio -> STT (raw transcript)
    --> PHI Redaction service (ClinicalBERT or rules)
    --> Redacted transcript (with placeholders)
    --> Public Gemini API (safe: no PHI exposure)
    --> Structured Output (placeholders inside)
    --> De-tokenization (server-side) before DB/UI

Storage and logs:

-   Store redacted transcript, structured output, function call arguments as defaults
-   Raw transcript storage only if org opts in -> must encrypt + restrict retention.

Trade-offs:

-   Can ship MVP quickly without BAA negotiation.
-   Future-proof for any LLM
-   Complex: real-time-placeholder management, re-injection, latency.

```vbnet
                 ┌──────────────────────────────┐
                 │         Audio Input          │
                 └──────────────┬───────────────┘
                                │
                                ▼
                     ┌──────────────────────┐
                     │ Speech-to-Text (STT)│
                     │ (HIPAA-covered if A)│
                     └───────────┬─────────┘
                                 │
     ┌───────────────────────────┼───────────────────────────┐
     │                           │                           │
     ▼                           ▼                           ▼

   MODE A: Vertex AI + BAA                  MODE B: Public AI Studio / API
   ─────────────────────────                ──────────────────────────────
   • PHI allowed to flow directly           • PHI must be redacted/tokenized
   • No pre-LLM redaction required          • Placeholders replace PHI
   • Optional internal tokenization         • Secure mapping maintained
                                            • More latency & complexity

   Pipeline:                                Pipeline:
   Transcript                               Transcript
        │                                        │
        ▼                                        ▼
   (Optional: tokenization)                 PHI Redaction Service
        │                                        │
        ▼                                        ▼
   Vertex AI Gemini (BAA)                  Public Gemini API (non-HIPAA)
        │                                        │
        ▼                                        ▼
   Structured Output / Functions           Structured Output (with placeholders)
        │                                        │
        ▼                                        ▼
   UI (with PHI) Storage (No PHI)         De-tokenization → UI
   (encrypted, ACL, audit logs)           (PHI restored locally if allowed)
```

## Storage (MVP)

PHI:

-   never store PHI for the MVP build.
-   Allowing organisation to opt-in for PHI storage should be a planned feature, notify users of intent in marketing website but for now we're not storing it.

PCI:

-   Never store PCI, ever

PII:

-   PII storage should be configurable. Strict mode for org (redact PII before LLM + redact PII before DB -- privacy-sensitive orgs)
-   Unlike PHI, don't have to block storage at MVP
-   Treat PII as "stored by default, but configurable stricter handling available."
