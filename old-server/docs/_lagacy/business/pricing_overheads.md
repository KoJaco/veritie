## 🟦 **1. Core Infrastructure Overheads**

-   **Frontend Hosting (Marketing + Docs site)** → likely Vercel/Netlify.
-   **Admin Dashboard Hosting** → small app, may stay in free tier for a long time.
-   **Backend Hosting (Server/API)** → you already factored _runtime minutes_ into per-session cost (Fly.io). But base allocations (persistent volumes, always-on processes, etc.) count as overhead.

---

## 🟩 **2. Observability & Monitoring**

-   **Prometheus/Grafana** → infra monitoring (metrics, uptime, scaling).
-   **Logging** → (e.g. Logtail, Datadog, Logflare, or ELK stack). Needed for debugging + auditing.
-   **Error Monitoring** → (e.g. Sentry or Rollbar).

👉 You might bundle some of these into Fly.io or Supabase if you keep it lean.

---

## 🟨 **3. Data & Storage**

-   **Database hosting** → Supabase (Postgres), possibly free tier early, but later \$25–50/mo minimum.
-   **File/object storage** → Supabase Storage or S3-compatible.

    -   Today: transcripts & metadata only (small).
    -   Later: optional audio storage (cost will scale linearly).

---

## 🟥 **4. Billing & Finance**

-   **Stripe**

    -   2.9% + \$0.30 per transaction (standard).
    -   Stripe Billing (subscriptions, invoicing) = extra 0.5–0.8%.
    -   At scale → this is a _variable_ cost, not fixed, but you’ll want to account for it in your margins.

---

## 🟧 **5. Communications & Support**

-   **SMTP/Transactional email** → Postmark, SendGrid, Resend, SES (\~\$10–30/mo to start).
-   **Customer Support (future)**

    -   Live chat widget (Intercom, Crisp, Zendesk) → \$50–100/mo.
    -   Ticketing → can wait until you have customers.

---

## 🟪 **6. Misc Business Costs**

-   **Domains & SSL** → \~\$10–15/yr per domain.
-   **Legal/Compliance** (later) → SOC2 readiness, privacy policies, contracts (not monthly, but expect \$\$ eventually).
-   **Analytics** → Plausible, PostHog, or GA4 (free → \$20–50/mo if privacy-first).

---

## 📊 **Overhead Table (Early Stage)**

| Category            | Likely Tool          | Cost (Early Stage) | Notes                  |
| ------------------- | -------------------- | ------------------ | ---------------------- |
| Frontend Hosting    | Vercel/Netlify       | \$20–50/mo         | Marketing site + docs  |
| Admin Dashboard     | Vercel/Netlify       | Free–\$20/mo       | Likely free quota      |
| Backend Base Infra  | Fly.io base          | \$15–30/mo         | Beyond per-session CPU |
| Database            | Supabase/Postgres    | \$25–50/mo         | Early paid tier        |
| Object Storage      | Supabase/S3          | \$0–10/mo          | Increases w/audio      |
| Monitoring          | Prometheus + Grafana | \$0–20/mo          | Self-host or managed   |
| Logging             | Logtail/Datadog      | \$0–20/mo          | Usage dependent        |
| Error Tracking      | Sentry               | Free–\$29/mo       | Starter tier           |
| Transactional Email | Postmark/SendGrid    | \$10–30/mo         | Based on volume        |
| Stripe Fees         | Stripe               | 2.9% + \$0.30      | Variable per txn       |
| Domain/SSL          | Namecheap/Cloudflare | \$10–15/yr         | Minimal                |
| Analytics           | Plausible/Posthog    | \$20–50/mo         | Optional early         |
| Customer Support    | Intercom/Crisp       | \$0–50/mo          | Later stage            |

👉 Realistically, you’re looking at **\$150–250/mo fixed baseline** once you move past free tiers. That’s well within reach if your gross margin per session stays 70–80%+.

---

## ✅ Key Insights

1. **You’ve already nailed variable cost modeling (sessions).**
2. **Overheads are small in comparison** → they won’t blow your economics unless you add enterprise audio storage or heavy compliance tools.
3. **Stripe + Support scale with users** → so think of them as semi-variable costs.
4. **Margin covers this** → your markup needs to be enough to absorb \~\$200–300/mo in fixed overheads at the early stage, plus Stripe fees at scale.
