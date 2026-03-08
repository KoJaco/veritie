## 🔢 **Current Cost Profile** -- MUST STILL BE VERIFIED

-   **Average session**

    -   Duration: \~32s
    -   Cost: **\$0.0047**
    -   Tokens: \~3,900 per minute of audio (70,375 ÷ 9.6 mins total)
    -   Savings from caching: \~11% (7,607 ÷ 70,375)

-   **Implied cost per minute**

    -   \$0.0047 ÷ (32s/60) ≈ **\$0.0088/min**

So Schma’s raw infra costs (with Deepgram STT + Gemini Flash 2.0) are **under 1 cent per minute** — very low.

---

## 💰 **Pricing Implications**

1. **Huge margin headroom**

    - If your cost is \~\$0.009/min, you can easily charge **\$0.05–\$0.15/min** to developers and still look cheap vs market.
    - For comparison:

        - Deepgram alone: \~\$0.004–0.012/min
        - AssemblyAI: \~\$0.015–0.025/min
        - AWS Transcribe: \~\$0.024/min
        - Vapi: \$0.05/min (plus separate STT + LLM costs)

2. **Position Schma as “transparent, predictable, cheaper than rolling your own.”**

    - Developers could hack Deepgram + Gemini together themselves — but they’d face hidden token costs, debugging, retries, and integration pain.
    - You offer **predictability + orchestration + savings (from caching)**.

3. **Caching already adds real value**

    - 11% token savings is meaningful, especially at scale.
    - Pitch this as: _“Schma automatically optimizes your LLM usage — saving you 10–20% on average.”_
    - That directly justifies your markup.

---

## 📊 **Example Pricing Framework Using These Numbers**

| Tier               | What’s Included                                 | Price Point           | Margin vs Cost |
| ------------------ | ----------------------------------------------- | --------------------- | -------------- |
| **Free Trial**     | \$10 credits (\~1,000 mins at \$0.01/min)       | Free                  | –              |
| **Developer PAYG** | PAYG @ **\$0.05/min** (rounded from 32s chunks) | \~82% gross margin    |                |
| **Pro (Volume)**   | 10K mins/month minimum @ **\$0.03/min**         | \~70% gross margin    |                |
| **Enterprise**     | Custom (e.g. \$0.02–\$0.025/min at 100K+ mins)  | \~55–65% gross margin |                |

👉 Even if you scale to enterprise and charge **3× your cost base**, your margin is healthy while still undercutting incumbents.

---

## 🎯 **Strategic Takeaways**

-   **Moat through pricing transparency:** Competitors hide true costs; you surface them in real time.
-   **Moat through savings:** Your caching/optimisation _already_ saves \~10% — as you improve this, Schma becomes “the cost shield.”
-   **Upsell path:** PAYG → volume discounts → enterprise add-ons (compliance, BYO DB, audio storage).

---

⚡ So here’s the story you can tell incubators/investors:

-   _“Our infra costs average under 1 cent per minute. We can price at \$0.05–0.10/min, still under market, and take 70–80% margins. Developers pay more for transparency, reliability, and built-in savings — not raw STT. As usage scales, optimization features (caching, smart routing) widen margins further.”_
