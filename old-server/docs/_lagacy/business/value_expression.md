# 📌 **Schma – Consolidated Vision**

---

## ✅ **Current Value Proposition**

**Schma is the real-time voice-to-action infrastructure layer for developers and businesses.**
It sits between **speech-to-text providers** and **LLMs**, handling all the complexity of turning speech into **functions, structured outputs, or enhanced text** with full transparency, reliability, and developer tooling.

### Current Unique Value

1. **Session Intelligence Layer**

    - Merges and orders function calls across a session.
    - Retains context, handles partial/draft calls, improves UX.

2. **Developer Debugging & Replay**

    - Full session replay: transcript turns, LLM calls, timing, metrics.
    - Audit trail for debugging and compliance.

3. **Data & Metrics Storage**

    - Complete logging of transcripts, usage, and analytics.
    - No audio storage by default → compliance-first stance.

4. **Transparency & Reliability**

    - Accurate cost tracking, session metrics, robust error handling.
    - SDKs and snippets for quick integration into apps.

---

## 🟩 **Planned Features & Their Value**

### 1. **Compliance & Trust Layer (Must-Have for B2B)**

-   **PII/PHI redaction** (already in place).
-   Configurable **data retention policies** (delete after X days, or never store).
-   Optional enterprise features: **BYO database**, HIPAA/SOC2 roadmap.
    **Value:** Makes Schma adoption possible in healthcare, finance, and enterprise — removes a key adoption blocker.

---

### 2. **Cost Optimisation & Smart Routing (Defensibility)**

-   Automatic **caching of LLM calls** (already partially there).
-   **Real-time config estimation** (already there).
-   Future: **dynamic LLM routing** (e.g. default to Gemini Flash, fallback to Pro if complex).
    **Value:** Lowers unpredictable AI costs, turns Schma into the _“cost shield”_ for developers.

---

### 3. **Embeddable UI / DX Enhancements (Adoption)**

-   Copy-paste snippets for React, Flutter, etc. (already doable).
-   Later: drop-in components (voice recorder, transcript viewer, form filler).
    **Value:** Lowers barrier to entry, makes integration a “10-minute task.”

---

### 4. **Enterprise Audio Handling (Optional, Later)**

-   Default: no audio storage (compliance-first).
-   Future: optional client-managed audio upload or enterprise storage add-on.
    **Value:** Unlocks customers who need audio audit trails or training data, without bloating MVP.

---

### 5. **Function Call Mediation (Future Power Feature)**

-   Instead of just returning suggested function calls, Schma can **execute & mediate** them.
-   Turns Schma into a **function orchestration layer** as well as parsing.
    **Value:** Moves Schma up the stack → from infra to workflow automation platform.

---

## 🚦 **Priority List**

### **Phase 1: MVP → Accelerator Ready (Now – 3 months)**

-   Core STT → LLM orchestration (✅ already).
-   Session intelligence + replay debugger (✅).
-   Transcript + metrics storage (✅).
-   Cost transparency endpoints (✅).
-   SDK + snippets for quick integration.

👉 This is what you take into **Genesis/INCUBATE** and to early showcase users.

---

### **Phase 2: Traction & Enterprise Credibility (3–9 months)**

-   Compliance & Trust Layer (data retention policies, PII/PHI redaction polished).
-   Cost optimisation v1 (caching, batching refinements).
-   First embeddable UI snippets (e.g., “Voice Recorder → Transcript Box”).

👉 This is what you take into **Startmate/Alchemist/Techstars** — shows traction + enterprise readiness.

---

### **Phase 3: Enterprise-Grade Platform (9–18 months)**

-   BYO database + optional enterprise audio storage.
-   Cost optimisation v2 (dynamic LLM routing, fallback strategies).
-   Function call mediation layer.
-   Expanded embeddable UI library.

👉 This is your “scale-up” roadmap — what big customers and investors expect.

---

# 🎯 **Summary**

-   **Schma today = reliable, transparent, developer-first infra for real-time voice → action.**
-   **Next step = compliance + cost control → unlocks enterprise and makes Schma sticky.**
-   **Future = orchestration & embeddable UI → Schma becomes the default voice infra layer powering thousands of apps.**
