# Normalization (Punctuation + Truecasing + ITN)

**Purpose.** Convert raw ASR text into a clean, LLM/rules‑friendly form in real time using a single “norm” sidecar:

-   **Punctuation + truecasing** (multilingual, ONNX)
-   **Inverse Text Normalization (ITN)** via WFST grammars (deterministic)

This keeps latency low, improves PCI/PHI/PII detection, and maximizes LLM accuracy for downstream tasks.

---

## 1) Scope & Goals

**In scope**

-   Real‑time post‑ASR normalization for short, ongoing turns
-   Languages: 47‑language punctuation/truecasing/segmentation model
-   Deterministic ITN for **numbers, dates, times, currency, measures, telephone**
-   AU locale bias (day‑first dates); ISO output (`YYYY‑MM‑DD`) by default
-   Works in streaming (micro‑batch) and batch modes

**Out of scope**

-   Speaker diarization, TTS normalization
-   Neural ITN (we standardize on WFST)

**Success criteria**

-   Added latency: **≤ 8 ms p50**, **≤ 20 ms p95** per short chunk (1–2 sentences)
-   ≥ 99% precision for numeric/date/phone canonicalization on our eval set
-   Downstream redaction (PCI/PHI/PII) recall improves vs. raw ASR

---

## 2) High‑Level Architecture

```
ASR partials → coalescer (2–5 ms / N tokens)
  → norm sidecar (UDS):
       1) Punct + Truecase (ONNX)
       2) ITN (WFST)
  → back to Go pipeline → PCI gate → PHI/PII redaction → (optional) PHI tokenizer sidecar → PHI ONNX → spans/mask → client
```

**Why this order?** Punctuation helps ITN parse; ITN produces canonical digits/dates/phones so PCI/PHI rules are simpler and more accurate.

---

## 3) Components

### 3.1 Punctuation + Truecasing

-   **Model**: Multilingual ONNX model that does **punctuation, truecasing, and sentence segmentation** across 47 languages.
-   **Quantization**: dynamic INT8 (AVX2); CPU‑friendly.
-   **Outputs**: punctuated, cased text (sentence boundaries optional).

### 3.2 Inverse Text Normalization (ITN)

-   **Engine**: WFST grammars (deterministic) via NeMo Text Processing.
-   **Domains**: numbers, ordinals, dates, times, currency, measures, telephone.
-   **AU bias**: prefer day‑first; output ISO `YYYY‑MM‑DD` (configurable). If needed, extend grammars for AU‑specific patterns.

---

## 4) API (UDS HTTP)

### 4.1 Endpoints

-   `POST /norm`

    -   **Req** `{ "text": string, "day_first": boolean=true }`
    -   **Res** `{ "text": string }`

-   `POST /norm_batch`

    -   **Req** `{ "texts": string[], "day_first": boolean=true }`
    -   **Res** `{ "texts": string[] }`

**Notes**

-   Use **UNIX domain socket** `/tmp/norm.sock` for low overhead & isolation.
-   Keep connections alive (HTTP keep‑alive).

### 4.2 Streaming pattern (client)

-   Maintain **committed** buffer + **tail** (unstable N tokens).
-   Coalesce new tokens for **2–5 ms** or until ≥ _N_ tokens, then call `/norm_batch`.
-   Send a small **overlap** (e.g., last 12–20 chars) to allow cross‑boundary patterns (e.g., dates) to form.
-   Emit only the **new committed prefix** after each call.

---

## 5) Configuration

```yaml
norm:
    uds_path: /tmp/norm.sock
    coalesce_ms: 3 # 2–5ms typical
    overlap_chars: 16
    locale:
        day_first: true # AU default
        date_output: ISO # ISO | locale
    punctuation:
        model: pcs_47lang # 47‑language ONNX
        quantize: int8 # dynamic int8
    itn:
        lang: en # NeMo grammars (en)
        custom_au_rules: true # optional; see §10
```

---

## 6) Latency & Capacity Planning

-   **UDS hop**: \~0.2–0.8 ms
-   **Punct/case (INT8)**: \~3–7 ms on short chunks
-   **ITN (WFST)**: \~0.5–2 ms
-   **Total add**: **\~3–8 ms p50** per call

**Scaling**

-   Start with **1 worker**; increase when CPU > 70%.
-   Horizontal scale with Fly process groups; keep UDS per instance.

---

## 7) Failure Modes & Degradation

-   **Timeout** (default 50–100 ms): return input unchanged, tag as `norm_bypass=true` in metadata; downstream gates still run.
-   **PCS model load failure**: run **ITN only**; conservative fallback.
-   **ITN failure**: return punctuated text only; PCI/PHI still run (lower precision for numbers/dates).
-   **Circuit breaker**: trip after N consecutive timeouts; auto‑recover on health.

---

## 8) Observability

**Metrics (Prometheus)**

-   `norm_requests_total{mode=batch|single}`
-   `norm_latency_ms_bucket` (p50/p95)
-   `norm_chars_total`
-   `norm_bypass_total{reason=timeout|error}`
-   `pcs_latency_ms`, `itn_latency_ms`

**Logs**

-   Sample **masked** inputs/outputs (no raw PII/PCI/PHI).
-   Emit model/version hashes for traceability.

---

## 9) Security & Privacy

-   UDS only; bind to loopback when TCP is required.
-   No persistence; in‑memory only.
-   Inputs/outputs are **post‑ASR strings**; downstream PCI/PHI gates still apply.
-   Rate‑limit and auth are unnecessary inside a single VM, but expose only via private network if needed.

---

## 10) Locale/AU behavior

-   Day‑first date interpretation; choose ISO output for machine processing.
-   Written numeric dates: normalize `DD/MM/YYYY` ↔ spoken dates.
-   Phones: normalize spoken digits (e.g., “oh four…”) to canonical `0412 345 678` so PCI/PII rules match reliably.
-   If strict AU behavior is desired, add grammar tweaks (see **Upgrade to Sparrowhawk** for how to export and deploy custom grammars).

---

## 11) Deployment

### 11.1 Sidecar process

-   Python app (FastAPI/uvicorn) with PCS ONNX + NeMo ITN.
-   Run with `--http httptools --loop uvloop`.
-   Health: `GET /healthz` returns `{ ok: true }`.

### 11.2 Fly.io

-   Two processes: `app` (Go main) and `norm` (this sidecar) in the **same VM**.
-   Share `/tmp/norm.sock`; supervise with Fly process groups.

---

## 12) Testing

**Unit tests**

-   Numbers, ordinals, decimals (“two hundred and five”, “double oh seven”, “three point five”).
-   Dates (spoken + numeric), AU bias, ambiguous forms.
-   Times (“half past four”, “three fifteen p m”).
-   Currency (“twenty dollars” → `$20`).
-   Measures (“ten milligrams” → `10 mg`).
-   Telephones (“oh four one two…” → `0412 345 678`).

**Perf tests**

-   Micro‑batch windows 2–5 ms; capture p50/p95.
-   Failure injection: slow model, ITN disabled, UDS unavailable.

---

## 13) Rollout Plan

1. Shadow mode on a subset of traffic; compare against raw ASR.
2. Enable for all **English** first.
3. Enable for more languages as needed.
4. Turn on AU grammar tweaks (if required) after collecting edge cases.

---

## 14) Upgrade Path: Sparrowhawk (C++)

**Why**: when you need sub‑millisecond ITN and/or want to remove Python from the hot path.

**Plan**

1. **Customize NeMo grammars** (Pynini) for AU specifics.
2. **Export** grammars to OpenFST **FAR** files.
3. Package Sparrowhawk with exported FARs as a local C++ service.
4. Replace NeMo ITN calls with Sparrowhawk endpoint (same API shape).
5. Keep the punctuation ONNX layer as‑is; only ITN changes.

**Risk/Backout**

-   Keep both ITN paths behind a flag: `itn_backend=python|sparrowhawk`.
-   A/B latency and correctness on the same inputs before cutover.

---

## 15) Versioning & Config Flags

-   `norm.version` (semantic; bump on model/grammar changes)
-   `norm.itn_backend` = `python` | `sparrowhawk`
-   `norm.day_first` = `true|false`
-   `norm.date_output` = `ISO|locale`
-   `norm.coalesce_ms` = `2–5`

---

## 16) FAQ

-   **Why WFST for ITN?** Deterministic, auditable, and very fast; grammars upgrade cleanly to C++.
-   **Why one sidecar?** Fewer hops; lower p95; simpler ops.
-   **What if normalization fails?** Fail‑open with raw text but mark `norm_bypass`; redaction still runs; PCI gate unchanged.
-   **Offsets?** We operate on normalized text only; if upstream alignment is required, return a lightweight char diff map (optional extension).

---

## 17) Open Items

-   Optional: return sentence boundaries for better chunking to LLM.
-   Optional: offset map API for clients that need alignment.
-   Optional: language auto‑detect if non‑English traffic grows.
