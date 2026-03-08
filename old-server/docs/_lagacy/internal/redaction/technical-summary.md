# 🚦 Schma Data Handling: Developer Summary (MVP)

Schma is designed to keep sensitive data safe by default. Here’s how we handle different types of information:

---

## 🔒 PHI (Protected Health Information)

-   **Default:** Schma never stores PHI in the database.
-   **Transmission:** PHI may be streamed back to your client app during live sessions (encrypted WebSocket).
-   **LLM Processing:**

    -   With **Vertex AI + BAA** → PHI may flow directly to Gemini.
    -   With **Public endpoints** → PHI is redacted/tokenized before LLM.

-   **Developer note:** You are responsible for any PHI you choose to store in your systems. Future: org-level opt-in for PHI storage under strict compliance controls.

---

## 👤 PII (Personally Identifiable Information)

-   **Default:** Stored by Schma (encrypted at rest) since it’s needed for core features (projects, notes, tasks).
-   **LLM Processing:** PII may flow to LLMs by default.
-   **Config:** Enable _Strict PII Mode_ if your org requires redaction before LLM and/or before DB storage.
-   **Developer note:** Expect names, emails, and contact info to persist unless strict mode is turned on.

---

## 💳 PCI (Payment Card Information)

-   **Always redacted:** PCI is masked immediately, never sent to LLMs, never stored in DB.
-   **Developer note:** Do not use Schma for payment processing. Any PCI input is destroyed on sight.

---

## 📜 Transcripts

-   **Real-time:** Streamed over encrypted WS, short-lived in memory.
-   **Storage:** Only **redacted transcripts** may be stored (for analytics/logging). Raw transcripts are not persisted by default.
-   **Developer note:** If you need raw transcripts, you’ll need to export them client-side and manage storage yourself.

---

## 🛠 Structured Outputs & Function Calls

-   **LLM Returns:** May contain placeholders if redaction is active.
-   **Server:** Can re-inject original values before sending back to the client.
-   **Database:** Stored by Schma, but PHI excluded in MVP.
-   **Developer note:** Safe to assume PII will persist, PHI won’t, PCI never.

---

## ✅ TL;DR

-   **PHI** → transient only, never stored (future opt-in).
-   **PII** → stored by default, strict mode available.
-   **PCI** → always redacted, never stored.
-   **Redacted transcripts** → safe for analytics/logs.
-   **Client apps** → responsible for storing whatever raw data they choose to keep.
