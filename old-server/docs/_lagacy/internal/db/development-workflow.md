# Database Development Workflow

This document outlines the streamlined database development workflow for Schma.ai using Atlas, SQLC, and Supabase.

## Quick Start

### 1. Simple Migration (Most Common)

```bash
# Create a new migration and sync everything
make db-quick-migrate name=add_batch_jobs
```

This single command will:

-   Generate Atlas migration from schema changes
-   Generate SQLC models
-   Sync to Supabase

### 2. Full Development Workflow

```bash
# For more control over each step
make db-dev name=add_batch_jobs
```

### 3. Individual Commands

```bash
# Generate migration only
make db-migrate name=add_batch_jobs

# Generate SQLC models only
make db-generate

# Apply to dev database only
make db-apply

# Sync to Supabase only
make db-sync
```

## Command Reference

### Makefile Commands

| Command                             | Description                              | Example                                |
| ----------------------------------- | ---------------------------------------- | -------------------------------------- |
| `make db-quick-migrate name=<name>` | Full workflow: migrate + generate + sync | `make db-quick-migrate name=add_users` |
| `make db-dev name=<name>`           | Full workflow with apply step            | `make db-dev name=add_users`           |
| `make db-migrate name=<name>`       | Generate migration from schema           | `make db-migrate name=add_users`       |
| `make db-generate`                  | Generate SQLC models                     | `make db-generate`                     |
| `make db-apply`                     | Apply migrations to dev DB               | `make db-apply`                        |
| `make db-sync`                      | Sync migrations to Supabase              | `make db-sync`                         |
| `make db-reset`                     | Reset database (with confirmation)       | `make db-reset`                        |
| `make dev`                          | Start development server                 | `make dev`                             |
| `make help`                         | Show all available commands              | `make help`                            |

### Advanced Script Commands

For more advanced operations, use the `scripts/db.sh` script:

```bash
# Check dependencies
./scripts/db.sh deps

# Show database status
./scripts/db.sh status

# Full development workflow
./scripts/db.sh dev add_batch_jobs

# Individual operations
./scripts/db.sh migrate add_batch_jobs
./scripts/db.sh models
./scripts/db.sh apply
./scripts/db.sh sync
```

## Workflow Steps Explained

### 1. Schema Changes

Edit `internal/infra/db/schema.hcl` to add new tables, columns, or modify existing schema:

```hcl
table "batch_jobs" {
  schema = schema.public

  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }

  column "status" {
    type = enum.batch_job_status
    null = false
    default = sql("'queued'")
  }

  # ... more columns
}
```

### 2. Generate Migration

```bash
make db-migrate name=add_batch_jobs
```

This creates a new SQL migration file in `internal/infra/db/migrations/`

### 3. Generate SQLC Models

```bash
make db-generate
```

This reads your SQL queries in `internal/infra/db/queries/` and generates type-safe Go code.

### 4. Apply to Development Database

```bash
make db-apply
```

This applies the schema changes to your local development database.

### 5. Sync to Supabase

```bash
make db-sync
```

This copies migrations to the Supabase directory and pushes them to your Supabase project.

## File Structure

```
internal/infra/db/
├── schema.hcl              # Atlas schema definition
├── migrations/             # Atlas-generated migrations
│   └── 20240101_add_batch_jobs.sql
├── supabase/
│   └── migrations/         # Copy of migrations for Supabase
│       └── 20240101_add_batch_jobs.sql
├── queries/               # SQL queries for SQLC
│   ├── batch_jobs.sql
│   └── users.sql
├── generated/             # SQLC-generated Go code
│   ├── batch_jobs.sql.go
│   └── models.go
└── repo/                 # Repository implementations
    └── batch_repo.go
```

## Environment Configuration

Create a `.env` file in your project root:

```bash
# Database URLs
DATABASE_URL=postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable
DEV_DATABASE_URL=postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable

# Other configuration...
```

## Best Practices

### 1. Migration Naming

Use descriptive names for migrations:

```bash
# Good
make db-migrate name=add_batch_jobs_table
make db-migrate name=add_user_email_index
make db-migrate name=update_session_status_enum

# Avoid
make db-migrate name=fix
make db-migrate name=update
```

### 2. Schema Organization

Keep your `schema.hcl` organized:

```hcl
# Enums first
enum "user_role" { ... }
enum "batch_job_status" { ... }

# Core tables
table "accounts" { ... }
table "users" { ... }

# Feature tables
table "sessions" { ... }
table "batch_jobs" { ... }
```

### 3. Query Organization

Organize SQL queries by domain:

```
queries/
├── auth.sql        # Authentication queries
├── batch.sql       # Batch job queries
├── sessions.sql    # Session queries
└── users.sql       # User management queries
```

### 4. Testing Migrations

Always test migrations on a copy of production data:

```bash
# Reset database to clean state
make db-reset

# Apply your changes
make db-apply

# Test your application
make dev
```

## Troubleshooting

### Common Issues

**1. Atlas not found**

```bash
curl -sSf https://atlasgo.sh | sh
```

**2. SQLC not found**

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

**3. Supabase CLI not found**

```bash
npm install -g supabase
```

**4. Database connection failed**

-   Check your DATABASE_URL in `.env`
-   Ensure your local Supabase is running
-   Verify connection credentials

### Getting Help

```bash
# Show all available commands
make help

# Check database status
./scripts/db.sh status

# Check dependencies
./scripts/db.sh deps
```

## Integration with Development

### Daily Workflow

1. **Start development**:

    ```bash
    make dev
    ```

2. **Make schema changes**:
    - Edit `schema.hcl`
3. **Generate and apply**:

    ```bash
    make db-quick-migrate name=your_change
    ```

4. **Test your changes**:
    ```bash
    make test
    ```

### CI/CD Integration

The Makefile commands are designed to work in CI/CD environments:

```yaml
# Example GitHub Actions
- name: Generate and apply migrations
  run: |
      make db-migrate name=ci_migration
      make db-generate
      make test-db
```

This streamlined workflow eliminates the manual copying of files and provides a consistent, repeatable process for database changes.
