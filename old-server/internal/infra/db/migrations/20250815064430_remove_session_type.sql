-- Modify "sessions" table
ALTER TABLE "public"."sessions" ADD COLUMN "type" "public"."session_type_enum" NULL;
