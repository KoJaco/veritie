# Schma — PII Redaction Feature (v1)

> **Purpose:** Prevent storage and logging of personally identifiable information (PII) by detecting and masking sensitive spans in transcripts and function-call payloads **before** they hit the database.

## <!-- https://github.com/philterd/phileas?tab=readme-ov-file -->

## 1) Goals & Non‑Goals

**Goals**

-   Detect common PII types in AU + general contexts with low latency.
-   Redact PII prior to persistence (DB, analytics) while keeping client UX unchanged.
-   Provide per‑app configurability (types, thresholds, masking mode, allowlists).
-   Produce structured detection metadata for observability without storing raw PII.

**Non‑Goals (v1)**

-   No reversible vault (original PII retrieval). Client-facing apps can retain originals if they choose.
-   No Python sidecar (spaCy/Presidio). Keep detection in‑process (Go) for latency and simplicity.
-   No cloud DLP dependency on the hot path. Optional, post‑v1 add‑on only.

---

## 2) Scope

**PII types (initial):**

-   Contact: `EMAIL`, `PHONE`, `URL`, `IPV4`, `IPV6`.
-   Financial: `CARD` (Luhn), `BSB`, `ACCOUNT_NO` (heuristic + anchors).
-   AU IDs: `ABN` (mod‑89), `ACN` (checksum), `TFN` (ATO weights), `MEDICARE` (+IRN), _Driver Licence_ (state-specific via anchors + patterns where reliable).
-   General entities: `NAME` (PERSON), `ORG`, `LOC`, `DATE` (DOB‑sensitive).

**Artifacts**

-   Transcripts (rolling + final), function call arguments, server logs, session snapshots.

---

## 3) Architecture Overview

```
STT → Normalize (numbers, spacing) → DetectPII (Rules → NER → Heuristics) → Merge/Decide → Mask → Persist/Emit
```

-   **RulesDetector (regex + checksums)**: High precision for structured tokens (emails, phones, IDs, cards, ABN/ACN/TFN/Medicare).
-   **NERDetector (ONNX DistilBERT)**: Recall for squishy entities (names, orgs, locations, dates). Sliding window over recent tokens.
-   **HeuristicContext (fastText synonyms + anchors)**: Keyword proximity boosts (e.g., "tfn", "medicare", "dob", "licence").
-   **Decision**: Redact if _validated rule hit_ OR _(NER.score ≥ threshold and not allowlisted)_. Rule wins on overlap.
-   **Masking**: Format‑preserving for numeric/structured IDs; tag masks for entities (e.g., `[NAME]`).

**Placement**

-   Runs on **transcript deltas** and **function args** before they enter persistence/log streams.

---

## 4) Normalization (non‑persisted)

-   Collapse whitespace/hyphens in digit runs for checksum validation (retain original for masking).
-   Number words → digits for AU phones/cards (e.g., "oh four", "double three").
-   Lowercase for email/URL checks; preserve case for masking output.

---

## 5) Detection Components

### 5.1 RulesDetector (compiled at startup)

-   **Email**: RFC‑ish regex + TLD check.
-   **Phone (AU)**: `+61`/`0` prefixes, mobile/landline patterns.
-   **Credit/Debit Card**: 13–19 digits with Luhn.
-   **ABN**: 11 digits (spaces allowed) + mod‑89 algorithm.
-   **ACN**: 9 digits + weighted mod‑10 check.
-   **TFN**: 8–9 digits + ATO weights.
-   **Medicare**: 10‑digit base + issue no. (+IRN) with range checks.
-   **IP/URL**: Conservative regex; anchor rules to avoid over‑matching.

> Rules emit spans **only after** post‑validation (checksum/range). This keeps precision high.

### 5.2 NERDetector (DistilBERT via ONNX Runtime)

-   Entity tags: `PERSON`, `ORG`, `LOC`, `DATE`.
-   **Windowing**: run on last \~256 tokens (overlap 64) of rolling transcript.
-   **Concurrency**: fixed worker pool (2–4). If saturated, degrade to Rules‑only for that delta.
-   **Offsets**: maintain token↔byte mapping; offsets recorded as **byte indices** in the original string.

### 5.3 HeuristicContext

-   Anchor keywords (with fastText synonyms): `{tfn, tax file number, medicare, driver’s licence, dob, date of birth, passport, abn, acn, bsb, account}`.
-   If an anchor appears within ±N tokens of a digit run, lower NER threshold for `DATE`/`NAME` or force rule re‑check on the normalized view.

---

## 6) Decision & Merge Logic

1. Collect spans from Rules, NER, Heuristics.
2. Merge overlaps: prefer **Rules > Heuristics > NER** when spans collide.
3. For `DATE`, treat as **DOB** when near anchors (e.g., "DOB", first‑person possession) → apply stricter mask.
4. Apply allowlist (per app) before finalizing spans (e.g., customer’s own company name).

---

## 7) Masking Strategy (storage/logs)

**Format‑preserving (where possible):**

-   **EMAIL** → `j***@domain.tld`
-   **PHONE** → `+61 **** *** 23`
-   **CARD** → `**** **** **** 1234`
-   **ABN** → `** *** *** 123`
-   **ACN/TFN/MEDICARE** → keep last 2–3 digits; preserve separators/spaces.

**Tag masks (no stable format):**

-   **NAME** → `[NAME]`
-   **ORG** → `[ORG]`
-   **LOC** → `[LOC]`
-   **DATE** → `[DATE]` or year‑only for suspected DOB (`199*-**-**`) per policy.

**Optional linkability:** store `HMAC_SHA256(original, per_app_salt)` alongside masked text for dedup/joins without exposing the original.

---

## 8) Config (per app)

```json
{
    "pii": {
        "enabled": true,
        "types": [
            "EMAIL",
            "PHONE",
            "CARD",
            "ABN",
            "ACN",
            "TFN",
            "MEDICARE",
            "NAME",
            "ORG",
            "LOC",
            "DATE"
        ],
        "masking_mode": "format-preserving", // or "tags-only"
        "thresholds": { "NAME": 0.85, "ORG": 0.85, "LOC": 0.85, "DATE": 0.8 },
        "allowlist": ["Acme Pty Ltd"],
        "hash_linkability": true,
        "degrade_on_load": true
    }
}
```

-   Resolved via existing app settings cache (TTL 30s). Changes apply without restart.

---

## 9) Public Interfaces (Go)

```go
type Span struct {
    Type       string  // EMAIL, PHONE, CARD, ABN, NAME, ORG, LOC, DATE, ...
    Start, End int     // byte offsets in the ORIGINAL string
    Source     string  // "rule" | "ner" | "heuristic"
    Score      float32 // used for NER
}

type Detector interface { Detect(s string) ([]Span, error) }

type Masker interface { Mask(s string, spans []Span) (string, error) }

func DetectPII(s string, detectors ...Detector) []Span
func RedactPII(s string, spans []Span, m Masker) string
```

**Integration points**

-   Transcript path: run `DetectPII` on deltas → `RedactPII` → append to session buffer (redacted) → emit to DB/logs.
-   Function path: apply to **argument values** before event logging/persistence.
-   Logs: scrub at boundary; never log unredacted payloads.

---

## 10) Performance Targets

-   Rules pass: < 1 ms per typical delta (< 1 KB).
-   NER pass: < 8–12 ms per 256‑token window on shared Fly VM; pool size 2–4.
-   End‑to‑end redaction overhead budget: **< 20 ms P95** added to transcript path.
-   If pool saturated or ONNX error: **rules‑only** fallback; emit metric.

---

## 11) Observability & Audit

**Metrics**

-   `pii_detection_total{type,source}` counters.
-   `pii_latency_seconds_bucket{stage=rules|ner|merge}` histograms.
-   `pii_fallback_total{reason=pool_saturated|onnx_error}`.
-   `pii_masking_mode{app}` gauge.

**Structured audit** (no originals)

-   Store `{type, start, end, hash, source, score, ts, app_id, session_id}` in a lightweight side table when `pii.audit=true`.
-   Sampling toggle (e.g., 1–5%) for manual QA.

---

## 12) Security & Compliance

-   Data minimization: store masked strings only; originals never written to disk/logs.
-   HMAC with per‑app salt (stored in KMS/secret manager) for linkability.
-   RLS on audit table by `app_id`.
-   Configurable retention for audit spans (e.g., 14–30 days) — metadata only.

---

## 13) Testing Strategy

-   **Golden tests** for regex/checksum validators and maskers.
-   **Offset tests** (byte vs rune) with mixed Unicode content.
-   **Windowing tests** for NER to ensure consistent spans across overlaps.
-   **Heuristic tests** (anchors near numbers) and negative cases (avoid over‑masking).
-   **Load tests** to validate pool sizing and fallback behavior.

---

## 14) Rollout Plan

1. **Phase 1 (Rules‑only)**: email/phone/card/ABN/ACN/TFN/Medicare + masking; metrics in place.
2. **Phase 2 (NER + Heuristics)**: enable DistilBERT windowing; add anchors; tune thresholds.
3. **Phase 3 (Admin QA)**: sampling endpoint `/admin/pii:test` to inspect masked output and spans; adjust allowlists per app.
4. **Optional**: enterprise tier with cloud DLP as a secondary checker (off the hot path, hard timeout).

---

## 15) Risks & Mitigations

-   **Over‑redaction (loss of utility)** → allowlist per app; entity masks only where beneficial.
-   **Under‑redaction (leak)** → rules are conservative/validated; NER thresholds tuned; QA sampling; add anchors.
-   **Latency spikes** → bounded NER pool; degrade to rules‑only; monitor `pii_fallback_total`.
-   **Checksum false positives** → keep range checks and realistic prefixes (e.g., AU phone leading digits).

---

## 16) Examples (Before → After)

**Email + Name**

-   _Before_: "Email me at [**jordan.patel@harborlogistics.com**](mailto:jordan.patel@harborlogistics.com) — I’m **Jordan**."
-   _After_: "Email me at **j**\*@harborlogistics.com\*\* — I’m **[NAME]**."

**ABN + Org**

-   _Before_: "Our ABN is **83 914 571 673**; company is **Harbor Logistics Pty Ltd**."
-   _After_: "Our ABN is \*\*\*\* \*\*\* \*\*\* 673\*\*; company is **[ORG]**."

**DOB**

-   _Before_: "My **date of birth** is **1992‑11‑03**."
-   _After_: "My **date of birth** is **[DATE]**."

---

## 17) Appendix — AU Checksums (pseudo)

**ABN**: subtract 1 from first digit; weights `[10,1,3,5,7,9,11,13,15,17,19]`; `(sum % 89) == 0`.

**ACN**: weights `[8,7,6,5,4,3,2,1]` on first eight digits; check digit = `(10 - (sum % 10)) % 10`.

**TFN**: 9 digits with weights `[1,4,3,7,5,8,6,9,10]`; `(sum % 11) == 0`. (Handle 8‑digit variant accordingly.)

**Luhn** (card): double alternate digits from the right; subtract 9 if >9; sum % 10 == 0.

---

### Ownership & Next Steps

-   **Owner:** Schma Core (Backend)
-   **M1:** Ship Phase 1 (rules‑only) behind feature flag; add metrics.
-   **M2:** Enable NER windowing + heuristics; tune thresholds with sampled QA.
-   **M3:** Admin QA endpoint & per‑app allowlist UI.
