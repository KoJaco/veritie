Below is a **ready-to-paste “Cursor prompt”** that will scaffold the full skeleton of your Test-&-Tag SaaS inside an **existing Remix + Supabase + Drizzle + shadcn/ui + Tailwind** workspace.
Copy everything between the **✂︎** lines into Cursor and run it from the project root.

---

✂︎ **Cursor Prompt – Voice-Driven Test & Tag MVP** ✂︎

````cursor
You are Cursor.
Scaffold a minimal, production-ready Test-&-Tag web app that uses my already-configured Remix / Supabase / Drizzle stack with shadcn/ui and TailwindCSS.
Assume authentication is implemented and `SupabaseClient` plus session helpers are available in `/app/utils/supa.server.ts`.

────────────────────────────────────────────────────────
## 1. Database layer (Drizzle)

### Tables
1. **test_tag_reports**
   - `id`              UUID  PK  default gen_random_uuid()
   - `account_id`      UUID  FK → accounts.id
   - `asset_id`        text  not null
   - `location`        text  null
   - `equipment_class` text  null      -- I or II
   - `tested_at`       date  not null
   - `next_due`        date  not null
   - `earth_ohms`      numeric(6,3) null
   - `insulation_mohm` numeric(7,2) null
   - `leakage_ma`      numeric(6,3) null
   - `polarity_pass`   boolean null
   - `result_pass`     boolean not null
   - `notes`           text   null
   - `raw_transcript`  text   null
   - `pii_redacted`    boolean default false
   - `created_at`      timestamptz default now()

2. **account_profile**   (one-to-one with accounts)
   - `account_id`          UUID PK FK → accounts.id
   - `tester_name`         text
   - `tester_licence_no`   text
   - `pat_make_model`      text
   - `pat_serial`          text
   - `pat_calibrated_at`   date
   - `pat_cert_url`        text    -- Supabase Storage path

Generate Drizzle schema files under `/app/db/schema/` and migration SQL.

────────────────────────────────────────────────────────
## 2. API routes

### POST `/api/reports/voice`
*Purpose*  Receives function roll-ups from Schma SDK and persists one `test_tag_reports` row.
*Input JSON*
```json
{
  "asset_id": "12345",
  "location": "Engine room",
  "result_pass": true,
  "notes": "Cord slightly frayed",
  "raw_transcript": "Test complete, pass."
}
````

‣ Infer `tested_at = today`, `next_due` = today + 3 months.
Return `201 Created` and the new row id.

### GET `/api/reports/export.csv?from=YYYY-MM-DD&to=YYYY-MM-DD`

‣ Stream CSV with all columns above scoped to current `account_id`.

### GET `/api/reports/export.pdf?from=…`

‣ Placeholder handler that returns `501` + TODO: fill with gofpdf.

────────────────────────────────────────────────────────

## 3. Remix routes / UI

### 3.1 `/inspect`

_Page goal_ Technician speaks while testing; results appear in a live table.

Components to build:

-   `MicButton` (shadcn `<Button variant="outline">`)
-   `LiveReportTable` (shadcn `<Table>` with columns: Asset, Location, Pass/Fail, Notes)
-   State handled with `useReducer` (`ADD_ROW`, `UPDATE_ROW`).

Flow:

1. On “Start test”, connect Schma SDK (`window.schma.connect(key)`).
2. For each `record_test_result` roll-up, POST to `/api/reports/voice` then update table.
3. “Finish & Export” button opens modal with date range → calls CSV export → downloads file.

### 3.2 `/profile`

Form with shadcn `<Input>` + `<FileUpload>` to edit `account_profile` fields.
Server action writes via Drizzle; file uploads to Supabase Storage bucket `cal-cert`.

────────────────────────────────────────────────────────

## 4. Client utilities

1. `app/utils/schma.client.ts`

    ```ts
    import Schma from "schma-sdk";
    export const connectMemo = (apiKey: string) =>
        Schma.connect({
            key: apiKey,
            onFunction(fn) {
                /* POST to /api/reports/voice */
            },
        });
    ```

2. `app/utils/csv.ts` – helper to download CSV blob.

────────────────────────────────────────────────────────

## 5. shadcn components to import

-   Button, Table, Input, Label, Dialog, Alert.

────────────────────────────────────────────────────────

## 6. Environment variables

Document required vars in `.env.example`

```
SUPABASE_URL=
SUPABASE_SERVICE_ROLE_KEY=
SCHMA_API_KEY=
```

────────────────────────────────────────────────────────

## 7. Testing stubs

-   Unit test `/api/reports/voice` using `vitest` and mocked Drizzle client.
-   Cypress test: visit `/inspect`, inject fake Schma event, verify table row appears.

────────────────────────────────────────────────────────

### Output format

1. **File tree diff** (▼ / ▲) of new & changed files.
2. Full contents of each new file.
3. Shell commands to run migrations & dev server.

Begin now.

```
✂︎ **end cursor prompt** ✂︎

---

**How to use**
1. Copy the prompt into a new Cursor tab at project root.
2. Hit *Run Prompt* → Cursor creates DB schema, Remix routes, handlers, and shadcn components.
3. Execute the shell commands Cursor prints (usually `pnpm db:migrate && pnpm dev`).
4. Open `/inspect`, paste your Schma API key, and start talking.

This gives you a complete skeleton that plugs straight into the Schma backend once your MVP server is live.
```
