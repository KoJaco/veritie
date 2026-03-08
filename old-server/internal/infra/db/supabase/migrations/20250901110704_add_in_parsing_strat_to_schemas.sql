-- Create enum type "schema_parsing_strategy_enum"
CREATE TYPE "public"."schema_parsing_strategy_enum" AS ENUM ('auto', 'update-ms', 'after-silence', 'end-of-session');
-- Modify "function_schemas" table
ALTER TABLE "public"."function_schemas" ADD COLUMN "parsing_strategy" "public"."schema_parsing_strategy_enum" NOT NULL DEFAULT 'auto';
