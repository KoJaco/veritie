# Schma — STT Diarization (v1)

> **Purpose:** Add provider‑agnostic diarization so Schma can attribute transcript words (and segments/turns) to speakers in real time, without changing the app‑facing streaming API.

---

## 1) Goals & Non‑Goals

**Goals**

-   Word‑level speaker attribution with stable, session‑scoped IDs.
-   Optional turn/segment aggregation for analytics and UX.
-   Provider‑agnostic config (enable/disable, min/max speakers, channel hints).
-   Preserve compatibility with existing `STTClient.Stream` signature.
-   Low overhead: < 5 ms P95 incremental processing beyond STT decode.

**Non‑Goals (v1)**

-   No speaker identification (naming people) — diarization only (who spoke when).
-   No cross‑session speaker tracking.
-   No model training in the hot path; rely on providers’ diarization + normalization logic.

---

## 2) Data Model & Types (Backward‑Compatible)

```go
// Positive speaker IDs, session‑scoped. 1..N. 0/omit = unknown.
type SpeakerID uint16

type Word struct {
    Text           string    `json:"text"`
    Start          float32   `json:"start"`
    End            float32   `json:"end"`
    Confidence     float32   `json:"confidence,omitempty"`
    PunctuatedWord string    `json:"punctuated_word,omitempty"`
    Speaker        SpeakerID `json:"speaker,omitempty"` // NEW
    SpeakerConfidence float32 `json:"speaker_confidence,omitempty"` // NEW, but only for batch processes.
}

// Optional segment‑level diarization ("turns").
type Turn struct {
    ID         string    `json:"id"`
    Speaker    SpeakerID `json:"speaker"`
    Start      float32   `json:"start"`
    End        float32   `json:"end"`
    Text       string    `json:"text,omitempty"`
    Confidence float32   `json:"confidence,omitempty"`
    Final      bool      `json:"final,omitempty"`
}

type Transcript struct {
    Text        string  `json:"text"`
    IsFinal     bool    `json:"final"`
    Confidence  float32 `json:"confidence,omitempty"`
    Stability   float32 `json:"stability,omitempty"`
    Words       []Word  `json:"words,omitempty"`
    ChunkDurSec float64 `json:"chunk_dur_sec,omitempty"`

    // Diarization (optional)
    Turns             []Turn `json:"turns,omitempty"`                // NEW
    DiarizationStable bool   `json:"diarization_stable,omitempty"`   // NEW
    Channel           int    `json:"channel,omitempty"`              // NEW (for multi‑channel paths)
}
```

> `Speaker` on `Word` and `Turns` are optional; existing clients remain compatible.

---

## 3) Config Surface (Provider‑Agnostic)

```go
type DiarizationConfig struct {
    Enabled       bool `json:"enabled"`
    MinSpeakers   int  `json:"min_speakers,omitempty"` // 0 = unknown
    MaxSpeakers   int  `json:"max_speakers,omitempty"`
    ChannelCount  int  `json:"channel_count,omitempty"` // e.g., 2 for stereo
    PreferChannel bool `json:"prefer_channel,omitempty"` // treat channels as speakers if available
}

type STTConfig struct {
    Provider    string             `json:"provider,omitempty"`
    SampleHertz int                `json:"sample_hertz,omitempty"`
    Encoding    string             `json:"encoding,omitempty"`
    Diarization *DiarizationConfig `json:"diarization,omitempty"` // NEW
}
```

-   Adapters translate these hints into provider flags when supported.
-   Fallback: if a provider can’t honor a hint, it’s ignored gracefully.

---

## 4) Adapter Normalization Rules

**Contract**

-   Speaker IDs exposed to the app must be **stable** (1..N) for the session.
-   Prefer **word‑level** labels. If provider gives only segment labels, project them to words by time overlap.
-   If a provider revises earlier audio with new speaker labels, emit a new `Transcript` with updated `Words`/`Turns` and `DiarizationStable=false`. When stable, set `DiarizationStable=true`.

**Channel diarization**

-   If `PreferChannel` and `ChannelCount>1`, map `channelIndex+1 → SpeakerID` for maximum stability.

**Deepgram**

-   Map provider speaker indices to stable 1..N. If interim renumbering occurs, maintain an internal map.
-   Build `Turns` by coalescing consecutive words with same `Speaker` and < 500 ms internal gaps.

**Google**

-   When using speaker diarization, Google outputs word‑level (or segment) speaker tags. Normalize to `Word.Speaker` then build `Turns`.
-   If only segment speaker tags exist, assign across words by [Start,End] overlap ≥ 50%.

---

## 5) Pipeline Integration

```
Audio In → STT Adapter (provider) → Normalize Diarization →
  (Words with Speaker) → Build Turns → Emit Transcript →
    PII Redactor (preserve Speaker) → Persistence/Downstream
```

-   **Order:** Diarization happens inside the adapter before redaction; speaker labels remain on `Word` after masking.
-   **Revisions:** Only emit changed regions; avoid replaying whole history when speakers stabilize.
-   **Smoothing:** Require N consecutive words (e.g., 3) before flipping a `Turn` speaker to reduce jitter.

---

## 6) Role Mapping (Optional Layer)

```go
type SpeakerRole struct {
    Speaker SpeakerID `json:"speaker"`
    Role    string    `json:"role"` // "agent","customer","host",...
}
```

-   Accept a client message to pin roles (e.g., UI sets Speaker 1 = agent).
-   Analytics can aggregate by role without modifying diarization internals.

---

## 7) Persistence & API

-   Persist `Words` with `Speaker` in session storage (or JSON column).
-   Optionally persist `Turns` for fast analytics/UX (speaker timelines, jump‑to‑turn).
-   Downstream apps can compute per‑speaker summaries, sentiment, and action items.

**Example Transcript message (abbreviated)**

```json
{
    "type": "transcript",
    "text": "hi jordan thanks for joining...",
    "final": false,
    "diarization_stable": false,
    "words": [
        { "text": "Hi", "start": 0.1, "end": 0.2, "speaker": 1 },
        { "text": "Jordan", "start": 0.2, "end": 0.45, "speaker": 1 },
        { "text": "thanks", "start": 0.46, "end": 0.7, "speaker": 1 },
        { "text": "Yep", "start": 1.1, "end": 1.25, "speaker": 2 }
    ],
    "turns": [
        {
            "id": "t1",
            "speaker": 1,
            "start": 0.1,
            "end": 0.98,
            "text": "Hi Jordan, thanks for joining",
            "final": false
        },
        {
            "id": "t2",
            "speaker": 2,
            "start": 1.1,
            "end": 1.4,
            "text": "Yep",
            "final": false
        }
    ]
}
```

---

## 8) Performance Targets

-   Adapter normalization: < 2 ms P95 per transcript delta (\~1 KB JSON).
-   Turn builder: linear over word count; coalesce in O(n).
-   No additional network RTT; fields piggyback on existing `Transcript` frames.

---

## 9) Quality KPIs & Evaluation

-   **DER (Diarization Error Rate)** target: ≤ 12% on mixed two‑speaker calls; ≤ 8% with channel separation.
-   **Speaker flip rate**: < 1 flip per minute after first 10s.
-   **Turn fragmentation**: median words per turn ≥ 6.
-   Periodic canary evaluation on a held‑out set (internal fixtures + customer opt‑in data).

---

## 10) Observability

-   `stt_diarization_enabled_total{provider}`
-   `stt_diarization_turns_total{provider}`
-   `stt_diarization_flip_total{provider}`
-   Latency histograms for adapter normalization + turn builder.

---

## 11) Testing Strategy

-   **Unit:** segment→word projection, overlap math, turn coalescing, stability flags.
-   **Property:** arbitrary overlapped segments produce monotonic, non‑overlapping turns by speaker.
-   **Synthetic audio:** alternating A/B speakers, interruptions, cross‑talk.
-   **Provider fixtures:** recorded JSON from Deepgram & Google mapped to expected `Word.Speaker` + `Turns`.
-   **Load:** long streams (30–60 min) to catch memory/ID mapping drift.

---

## 12) Rollout Plan

1. **Phase 1 — Word.Speaker only** (feature flag per app). Validate latency + correctness.
2. **Phase 2 — Turns + DiarizationStable**. Enable coalescing and stability semantics.
3. **Phase 3 — Role mapping + analytics**. Optional client message; per‑role summaries.

---

## 13) Risks & Mitigations

-   **Provider relabel jitter** → internal ID mapping + stability threshold; `DiarizationStable` signaling.
-   **Over‑segmentation** → coalesce small gaps; minimum turn duration.
-   **Multi‑channel mismatch** → explicit `ChannelCount`/`PreferChannel` hints; fall back to speaker diarization.
-   **Downstream coupling** → keep fields optional; default behavior unchanged when diarization disabled.

---

## 14) Ownership & Next Steps

-   **Owner:** Schma Core (Backend)
-   **M1:** Implement Word.Speaker in Deepgram + Google adapters; add flag + metrics.
-   **M2:** Add Turn builder + stability; persist optional `turns`.
-   **M3:** Client role mapping message + analytics widgets (per‑speaker timelines).
