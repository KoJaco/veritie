# 📑 Schma – Business Proposal (Condensed)

---

## 1. Executive Summary

-   **Schma**: Developer-first infrastructure layer for voice-to-action.
-   Converts speech into structured actions, functions, or enhanced text in real-time.
-   Bridges the gap between STT providers and LLMs with orchestration, compliance, and cost transparency.
-   Vision: **the Stripe/Twilio of voice-to-action**.

---

## 2. Problem

-   Developers:

    -   Hard to integrate STT + LLM reliably.
    -   Unpredictable costs, brittle pipelines, poor debugging.

-   Businesses:

    -   Demand efficiency via speech → action.
    -   Current tools are siloed (consumer apps) or raw infra (STT only).

-   No provider-neutral, compliance-ready infrastructure exists today.

---

## 3. Solution (Schma)

-   **Core features (MVP already working):**

    -   Real-time & batch orchestration.
    -   Structured outputs & function invocation.
    -   Session intelligence (ordering, merging).
    -   Replay debugger & audit trail.
    -   Cost transparency & caching (10–20% token savings).
    -   PII/PHI redaction.
    -   SDKs/snippets for fast integration.

-   **Planned features:**

    -   BYO Database (enterprise compliance).
    -   Function call handler.
    -   Embeddable UI components.
    -   Dynamic routing across STT/LLMs.
    -   Optional audio storage.

---

## 4. Market Opportunity

-   **Initial wedge:** Developers (B2D) integrating voice features into SaaS/internal tools.
-   **Expansion:** Enterprises seeking efficiency (legal, healthcare, finance, customer support).
-   **Macro trend:** AI adoption + voice interfaces rising quickly.
-   **Market size:** Multi-billion dollar AI infra and conversational AI markets.

---

## 5. Business Model

-   **Pricing:**

    -   PAYG (\~\$0.05–\$0.10/min; infra cost \~\$0.009/min).
    -   Free trial credits + capped sandbox for testing.
    -   Pro tier: volume discounts, priority support.
    -   Enterprise: compliance features, BYO DB, audio storage.

-   **Margins:** 70–80% gross margins, with pricing still under competitors.
-   **Future revenue:** embeddable components, analytics dashboards, function mediation.

---

## 6. Competitive Landscape

-   **STT providers (Deepgram, AssemblyAI):** raw transcripts only.
-   **Voice agent builders (Vapi, Voiceflow):** limited orchestration, no compliance/logging.
-   **Vertical apps (Otter, Fireflies):** consumer-facing, not infra.
-   **Schma:** provider-neutral infra with orchestration, compliance, analytics, and transparent pricing.

---

## 7. Traction

-   MVP complete; already processing test sessions with real costs measured.
-   Caching delivering \~11% token savings.
-   Showcase apps planned (built in 1 week each) to demonstrate potential.
-   Early user onboarding strategy defined (developer SDKs + showcase apps).

---

## 8. Roadmap & Milestones

-   **Next 6 months:**

    -   Launch showcase apps.
    -   Onboard first developer users.
    -   Build Pro tier pricing + docs.

-   **12 months:**

    -   Enterprise pilots (compliance features, BYO DB).
    -   Expand to multiple LLMs.
    -   Begin seed fundraising.

---

## 9. Risks & Mitigation

-   **Pricing volatility (STT/LLM costs):** Mitigated by provider neutrality + dynamic routing.
-   **Compliance burden:** Addressed via PII/PHI redaction, BYO DB, external audits.
-   **Competition:** Moat through orchestration, developer lock-in (SDKs), and cost transparency.

---

## 10. Ask (for incubator/accelerator)

-   **Mentorship & guidance** (go-to-market, fundraising).
-   **Support** with credits/funding for infra.
-   **Network access** to early enterprise partners and design-partner developers.
