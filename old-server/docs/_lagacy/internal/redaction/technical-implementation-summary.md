# Schma Redaction & Sensitive-Data Blueprint (Abstract)

## 0) Objectives (MVP posture)

-   **PHI:** never stored; allowed to transit over encrypted WS; pre-LLM redaction in Mode B.
-   **PII:** stored by default (encrypted at rest); optional strict mode to redact pre-LLM & pre-DB.
-   **PCI:** never stored; always redacted/masked; never sent to LLM.
-   **Streaming first:** sub-10 ms redaction budget per delta; batch supported but not primary.
-   **Two modes**

    -   **Mode B (now):** public endpoints → PHI redacted pre-LLM.
    -   **Mode A (later):** Vertex AI + BAA → PHI may flow to LLM, toggle off pre-LLM PHI redaction.

---

## 1) Hexagonal decomposition

### Domain (pure core)

**Ports / Contracts**

-   **Normalizer** → `Normalize(raw) → (norm, alignment, meta)`
-   **Redactor** (implemented by PCI/PHI/PII)

    -   `RedactTranscript(norm) → spans`
    -   `RedactFunctionArgs(normJSON) → spans`
    -   `RedactStructuredOutput(normJSON) → spans`

-   **RedactionOrchestrator** → merge spans (priority: `PCI > PHI > PII`), apply one rewrite to RAW via alignment, emit placeholders/masks.
-   **PlaceholderVault** → session-scoped reversible store for PHI/PII (never PCI); TTL-bounded; “no logging” guarantee.
-   **PolicyResolver** → resolves effective session/org policy (Mode, strict-PII, storage, etc.).
-   **PersistenceGuard** → gate anything headed to DB according to policy (e.g., “PHI prohibited”, “redacted transcripts only”).
-   **OutputValidator** → placeholder grammar guard + schema/type checks before any detokenization or persistence.

**Core Types**

-   `Span { start, end, kind: "PCI"|"PHI"|"PII", confidence, ruleId }`
-   `Alignment` (norm↔raw mapping; UTF-8 safe)
-   `Placeholder { kind, ordinal, text }` (grammar-constrained)
-   `RedactionPolicy { redactPHI, redactPII, redactPCI, placeholderGrammar, priorityOrder }`

> Domain knows nothing about ONNX/WS/Redis/KMS/LLM endpoints.

---

### App (use-cases / orchestration)

-   **HandleTranscriptDelta**

    1. Normalize → `(norm, alignment)`
    2. Fan-out to PCI/PHI/PII redactors on `norm`
    3. Orchestrate spans (PCI>PHI>PII), rewrite **RAW** once, update Vault (PHI/PII)
    4. **To LLM:** redacted RAW (Mode B)
       **To Client WS:** raw or redacted per UX policy (no DB PHI)
    5. Persist redacted transcript if enabled

-   **HandleLLMFunctionCall** / **HandleStructuredOutput**

    -   Validate placeholders → detokenize from Vault for outbound WS only
    -   Persist via **PersistenceGuard**: PHI stripped (MVP), PII per policy, PCI disallowed

-   **Backpressure**

    -   If redaction queue lags: pause LLM sends; continue WS transcript stream

-   **Session lifecycle**

    -   Policy snapshot at join; Vault TTL tied to session (+ short grace)

---

### Infra (adapters)

-   **NormalizerImpl**

    -   Spoken-number → digit; “double/triple”; dates/DOB; CVV/expiry; email/phone smoothing; alignment produced once per delta

-   **PCI Redactor**

    -   Regex+Luhn on **normalized digits**; expiry/CVV context; output = irreversible mask + `<PCI:CARD>` to LLM

-   **PHI Redactor**

    -   ClinicalBERT-ONNX spans; DOB/ID rules; output = placeholders + Vault entries

-   **PII Redactor**

    -   DistilBERT-ONNX + rules (email/phone/address/name); active only if strict-PII enabled for pre-LLM; output = placeholders + Vault entries

-   **PlaceholderVaultImpl**

    -   In-memory (optionally Redis) + envelope encryption via KMS; short TTL; never logged

-   **Config/Lexicon Loader**

    -   Boot-time seed for normalization vocabulary, context markers, condition/medication seeds, AU locale hints; per-org overlays supported

---

### Transport (thin shell)

-   WS/HTTP handlers → call App use-cases; **no business logic**
-   Auth: API key → principal → short-lived session token
-   All payload logging is **redacted-only**

---

### pkg (shared libs)

-   `redaction/spans` (overlap resolution, rune-safe slicing)
-   `redaction/grammar` (placeholder validation/formatting)
-   `ringbuffer` (boundary-safe streaming window)
-   `metrics` (latency, FP/FN, backpressure)
-   `security/kms` (envelope crypto helpers)
-   `ids` (session-scoped deterministic ordinals/tags)

---

## 2) Data lifecycles

**Streaming transcript (Mode B)**

1. STT delta (RAW) → **Normalize** → (NORM, alignment)
2. **Detect** on NORM (PCI/PHI/PII) → spans
3. **Orchestrate** spans → rewrite RAW once (masks + placeholders)
4. **Send to LLM:** redacted RAW
5. **Send to client:** policy-chosen view (no DB PHI)
6. **Persist:** redacted transcript only

**LLM outputs (functions/structured)**

1. LLM returns placeholders → **Validate grammar**
2. **Detokenize** from Vault for outbound WS only
3. **Persist:** via PersistenceGuard (PHI stripped; PII per policy; PCI absent)

---

## 3) Policies & configuration (effective at session start)

-   `MODE=B` (default MVP)

    -   `redactPHI=true`, `redactPCI=true`, `redactPII=strict?`

-   Storage: `storePHI=false`, `storeRawTranscripts=false`, `storePII=true`
-   Logging: `payload_logging=redacted_only`
-   Latency budget: `normalize+detect+rewrite ≤ 8 ms p95`
-   Org toggles: **Strict PII Mode**, PCI detection on/off hint (always redact if detected), locale options (AU/US date)

---

## 4) Performance, reliability, safety

**Targets**

-   Normalizer p95 ≤ 2 ms; detectors p95 ≤ 6 ms on typical deltas
-   Zero PHI to LLM in Mode B (provable by output validator & metrics)
-   Near-zero PCI false positives (Luhn + context gating)

**Guards**

-   If a detector fails, still enforce PCI regex; never pass unredacted PHI to LLM when policy requires redaction
-   Placeholder grammar whitelist; reject unknown placeholders
-   Vault misses → block detokenization & return placeholder (safe-by-default)

---

## 5) Observability & audit

-   Metrics: per-stage latency, detector hit rates, overlaps, placeholder counts, backpressure incidents
-   Redaction sampling dashboards (redacted snippets only) for QA on FP/FN (no originals)
-   Audit log: who/what/when for data accesses; never record PHI/PCI values

---

## 6) Testing (pre-prod)

-   **Normalizer golden set:** spoken digits, double/triple, DOB formats, CVV/expiry phrases, chunk boundaries, Unicode
-   **PCI:** PAN+Luhn w/ context; phones/IBAN decoys; expiry/CVV edge cases
-   **PHI:** names/DOB/IDs/conditions/meds embedded in natural speech; false-positive traps (“May”, “Ward”, surnames that are common nouns)
-   **PII strict mode:** verify no PII to LLM/DB when ON
-   **E2E streaming:** chunked flow → placeholders → detokenize → persistence guard invariants
-   **Chaos:** kill a detector; ensure blocking remains correct

---

## 7) Rollout plan (sequenced, reversible)

1. **Introduce Normalizer** (no redaction), measure latency & stability
2. **Enable PCI redaction** (mask + `<PCI:CARD>` to LLM)
3. **Enable PHI redaction** (placeholders + Vault) on transcript→LLM path
4. **Enforce return-path validation** (placeholders → detokenize → schema/type checks)
5. **Wire PersistenceGuard** (PHI blocked from DB; redacted transcripts only)
6. **Strict PII Mode** (org toggle) for pre-LLM & pre-DB
7. **Mode A toggle** ready (Vertex+BAA): set `redactPHI=false` while retaining PCI & optional PII

---

## 8) Risk register (top items & mitigations)

-   **Chunk boundary misses** → ring buffer + alignment; regression tests
-   **Over-redaction harming LLM reasoning** → placeholders preserve narrative; monitor accuracy; tune grammars
-   **Latency spikes** → pre-warm ONNX; small NORM windows; backpressure LLM only
-   **Vault leakage** → KMS envelope encryption; no logs; short TTL; strict ACLs
-   **Policy drift** → PolicyResolver snapshot at session start; include policy hash in telemetry

---

## 9) Documentation deliverables

-   `/docs/redaction-architecture.md` (this blueprint + diagrams)
-   `/docs/data-policy.md` (the non-table outline you approved)
-   `/docs/privacy-tos-drafts.md` (draft legal language you approved)
-   `/docs/operations-runbook.md` (feature flags, rollback steps, SLOs)

---

### One-page view (lifecycle)

-   **Audio → STT → Normalize → Detect (PCI/PHI/PII) → Orchestrate → Redacted → LLM → Placeholders → Validate/Detokenize → Client WS (transient)**
-   **Persist:** only redacted artifacts; no PHI/PCI in DB; PII per policy.
