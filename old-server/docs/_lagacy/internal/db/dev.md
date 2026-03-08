# Instructions for dev using atlas and sqlc locally

1. Spin up dev db for atlas to connect to (diffing)

```bash
docker run --name schma-dev-postgres \
 -e POSTGRES_PASSWORD=postgres \
 -p 54321:5432 \
 -d postgres:15
```

2. Make util script executable.

```bash
chmod +x scripts/db.sh
```
