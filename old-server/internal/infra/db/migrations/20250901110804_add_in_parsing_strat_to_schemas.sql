-- Modify "structured_output_schemas" table
ALTER TABLE "public"."structured_output_schemas" ADD COLUMN "parsing_strategy" "public"."schema_parsing_strategy_enum" NOT NULL DEFAULT 'auto';
