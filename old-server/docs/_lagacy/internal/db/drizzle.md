Using Atlas schema generations as the source of truth. Copying migrations to supabase dir and pushing from the backend (server). The frontend, I'm using drizzle and need to introspect to scheam to generate TypeScript defintions.

```bash
npx drizzle-kit introspect:pg \
  --connectionString=SUPABASE_CONNECTION_STRING \
  --out=drizzle/schema.ts
```
