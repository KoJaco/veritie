# 📄 Draft Privacy Policy (MVP)

**Last updated: \[Date]**

## 1. Introduction

Schma is a real-time voice-to-structured-data platform. We process audio, transcripts, and structured outputs on your behalf. Protecting your privacy and sensitive information is central to how we’ve designed our system. This Privacy Policy explains what we collect, how we use it, and what we never store.

---

## 2. Information We Process

### Audio and Transcripts

-   Audio is streamed to our speech-to-text service and processed in real time.
-   Transcripts are generated in memory and may be analyzed by our language models to produce structured outputs (e.g., notes, function calls).

### Protected Health Information (PHI)

-   Schma does **not store PHI** in our database by default.
-   PHI may be transmitted back to your client application during a live session.
-   PHI exists only in memory for the duration of the session.
-   A future **organization-level opt-in** feature will allow storage of PHI under enhanced compliance controls.

### Personally Identifiable Information (PII)

-   PII (such as names, emails, contact details) may be processed and stored by default, as this is often necessary for productivity features.
-   Organizations may choose a **Strict PII Mode**, where PII is redacted before being sent to the language model and before being stored.

### Payment Card Information (PCI)

-   Schma does **not process or store PCI data**.
-   If PCI is detected in input, it is immediately redacted or masked and never sent to a language model or stored in our database.

---

## 3. Storage

-   **PHI:** never stored by default (transient only).
-   **PII:** stored by default (encrypted at rest).
-   **PCI:** never stored.
-   **Transcripts:** only redacted versions may be stored for analytics or debugging; raw transcripts are not retained.

---

## 4. Security

-   All transmission occurs over encrypted connections (TLS/WSS).
-   Sessions are short-lived and authenticated against your organization’s API key.
-   Data is encrypted at rest and protected by access controls.

---

## 5. Your Responsibility

-   Schma transmits data back to your client application.
-   You are responsible for any storage or further processing of that data within your systems.
-   If you handle PHI or PII, you must ensure you comply with applicable regulations (e.g., HIPAA, GDPR, Australian Privacy Act).

---

## 6. Changes

We may update this policy from time to time. Continued use of Schma constitutes acceptance of the updated policy.

---
