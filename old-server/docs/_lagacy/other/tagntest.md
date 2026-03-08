## 1. Field-set strategy

-   **Keep every standard field in the schema** (asset ID, location, class, test date, next due, results, tester ID, instrument ID + cal-date, notes).
-   **Let account-owners toggle visibility** when they design their PDF/CSV template:

    _Hide on export_ → column/tag element suppressed
    _Remove from DB_ → **never** (stay compliant, support un-hiding later).

That gives flexibility without risking an audit because the raw data is still there.

---

## 2. Exports

-   **CSV** – perfect for bulk upload to company ERP or Excel.
-   **PDF** – satisfies AMSA/AS 3760 “tamper-evident” rule.
    _One PDF per appliance test_ **or** “Daily bundle” (all rows for a calendar day).
-   Optional **public link** (`/download/:report_id?token=…`) so an inspector can open it on-site without login.

This pair is enough for day one; no need for Word/email merges yet.

---

## 3. Storing licence & calibration details

Add to the **account profile**:

```
accounts
  id
  name
  tester_licence_no       TEXT
  tester_name             TEXT
  pat_make_model          TEXT
  pat_serial              TEXT
  pat_calibration_pdf_url TEXT   -- Supabase Storage or S3
  pat_calibrated_at       DATE
```

-   Licence number and calibration cert are uploaded once, not per report.
-   When you render the PDF you pull these values into the footer.
-   Add a dashboard banner “PAT calibration expired” if `NOW() > pat_calibrated_at + 365 days`.

---

## 4. Multi-tenancy & roles

### MVP suggestion

-   **Tenant = electrical business** (`accounts` table you already have).
-   **Roles**

    -   **Owner** – billing, API keys, licence details.
    -   **Technician** – create tests, export PDF.
    -   **Auditor/Viewer** – read-only export.

Use a **per-seat** model but keep it simple:
_First seat included, extras A\$10/mo each_. RLS policies from Schma’s schema already support `account_id` scoping, so extending to multiple users costs very little now and saves a migration later.

---

## 5. Pricing model draft

Assume your Schma backend voice cost ≈ A\$0.0008 per second (Deepgram + Gemini Flash).

| Tier           | Price (monthly, AUD)  | Included                            | Overage                    |
| -------------- | --------------------- | ----------------------------------- | -------------------------- |
| **Solo**       | **A\$19**             | 500 tests (≈ 2 hrs voice)           | \$0.04 per additional test |
| **Team**       | **A\$59** + \$10/tech | 2 000 tests, 5 GB PDF storage       | same \$0.04                |
| **Enterprise** | custom                | Unlimited seats, SSO, on-prem Schma | actual usage pass-through  |

_Why it works_

-   Per-test marginal cost is pennies; a 2 000-test Team plan costs you < A\$35 in compute.
-   Electricians typically charge A\$3–5 per tag; saving 10–15 s/test easily covers A\$0.03 software cost.

---

## 6. Order of implementation after Schma MVP

1. **Profile page** – licence number & calibration upload.
2. **Role & seat logic** – extend Supabase `account_members`.
3. **Export service** – CSV first, then PDF template with licence footer.
4. **Visibility toggles** – simple array of hidden columns stored in `app_settings`.

You can iterate on PDF cosmetics and tiered pricing after internal use proves latency & accuracy.
