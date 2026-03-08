BEGIN;

-- 0) Ensure the target enum exists (idempotent)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_type t
    JOIN pg_namespace n ON n.oid = t.typnamespace
    WHERE t.typname = 'action_enum' AND n.nspname = 'public'
  ) THEN
    CREATE TYPE public.action_enum AS ENUM ('create','retrieve','update','delete');
  END IF;
END$$;

-- 1) Drop any old default so we don't cast a default of the wrong type
ALTER TABLE public.permissions
  ALTER COLUMN actions DROP DEFAULT;

-- 2) Add the new array-typed column
ALTER TABLE public.permissions
  ADD COLUMN actions_new public.action_enum[];

-- 3) Populate it.
--    If actions is scalar on this DB (your Supabase case), wrap it to a 1-element array.
--    If it's NULL, keep NULL. (If you prefer empty array, change NULL branch.)
UPDATE public.permissions
SET actions_new = CASE
  WHEN actions IS NULL THEN NULL
  ELSE ARRAY[ (actions::text)::public.action_enum ]
END;

-- If your column is TEXT instead of enum on some rows/DBs, the ::text above still works.

-- 4) Swap columns
ALTER TABLE public.permissions DROP COLUMN actions;
ALTER TABLE public.permissions RENAME COLUMN actions_new TO actions;

-- 5) Optional: set a default empty array
-- ALTER TABLE public.permissions
--   ALTER COLUMN actions SET DEFAULT '{}'::public.action_enum[];

COMMIT;
