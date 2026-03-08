# Commands used with atlas

1. Schema inspect, test conn

```bash
atlas schema inspect --url "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable"
```

2. Run first diff (assuming we're in `./server`)

```bash
atlas migrate diff \
  --dev-url "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable" \
  --to "file://./internal/infra/db/schema.hcl" \
  --dir "file://./internal/infra/db/migrations" \
  "initial_schema"
```

if in internal/infra/db dir (change name)

```bash
atlas migrate diff \
  --dev-url "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable" \
  --to "file://./schema.hcl" \
  --dir "file://./migrations" \
  "<name of migration>"
```

3. If applying any manual migrations

```bash
atlas migrate hash \
  --dir "file://./internal/infra/db/migrations"
```
