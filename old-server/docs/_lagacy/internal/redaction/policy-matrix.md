# 📋 Schma Data Handling Policy (MVP)

### 1. **Protected Health Information (PHI)**

-   **Transmission:**

    -   May be transmitted to clients over encrypted, authenticated WebSocket sessions.
    -   Sessions are short-lived; PHI only exists in memory for the duration of the session.

-   **LLM Processing:**

    -   _Mode A (Vertex AI + BAA):_ PHI may flow directly to the LLM.
    -   _Mode B (Public Gemini / non-HIPAA LLM):_ PHI must be redacted or tokenized before reaching the LLM.

-   **Database Storage:**

    -   PHI is **never stored** in the Schma database in MVP.

-   **Default Policy:**

    -   PHI is transient only. Schma does not persist it.

-   **Future Option:**

    -   Org-level opt-in may allow PHI storage later, under strong encryption and compliance controls.

---

### 2. **Personally Identifiable Information (PII)**

-   **Transmission:**

    -   Transmitted over encrypted WebSocket/API channels with short-lived, authenticated sessions.

-   **LLM Processing:**

    -   Default: PII may be sent to the LLM.
    -   Optional: organizations can enable _Strict PII Mode_, which redacts before LLM.

-   **Database Storage:**

    -   PII is stored by default (encrypted at rest) and used for projects, notes, and structured outputs.

-   **Default Policy:**

    -   PII is persisted unless strict mode is enabled.

-   **Future Option:**

    -   Org-level toggle for redacting PII before both LLM and storage.

---

### 3. **Payment Card Information (PCI)**

-   **Transmission:**

    -   Always detected and masked immediately.
    -   Never transmitted in raw form.

-   **LLM Processing:**

    -   Always redacted before reaching the LLM.

-   **Database Storage:**

    -   PCI is **never stored** in the Schma database.

-   **Default Policy:**

    -   Schma never allows PCI to persist in the system.

-   **Future Option:**

    -   None. PCI is permanently excluded from storage and LLM exposure.

---

### 4. **Transcripts**

-   **Transmission:**

    -   Streamed in real-time over secure WebSocket connections.

-   **LLM Processing:**

    -   Redaction layers applied according to data type (PHI/PII/PCI).

-   **Database Storage:**

    -   Only redacted transcripts may be persisted (for analytics, debugging, logs).
    -   Raw transcripts are not stored in MVP.

-   **Default Policy:**

    -   Redacted transcripts are safe to persist.

-   **Future Option:**

    -   Org-level opt-in may allow raw transcript storage (excluding PHI and PCI in MVP).

---

### 5. **Structured Outputs and Function Calls**

-   **Transmission:**

    -   Returned to clients over WebSocket. May contain placeholders if redaction is applied.

-   **LLM Processing:**

    -   If redaction is active, placeholders are used. Server may replace placeholders with original values before returning to the client (but not persisting them).

-   **Database Storage:**

    -   Stored as part of notes/projects, but with PHI excluded in MVP.

-   **Default Policy:**

    -   Stored with PHI stripped, PII allowed.

-   **Future Option:**

    -   Storage of PHI values if orgs explicitly opt-in.
