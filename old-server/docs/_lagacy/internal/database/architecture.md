# Database Architecture

## Overview

The database layer provides type-safe, migration-managed data persistence using PostgreSQL with Atlas for schema management and SQLC for query generation. It follows a repository pattern with clean separation between domain models and database concerns.

## Core Architecture

### Technology Stack

```
Application Layer
       ↓
Repository Pattern (Domain Interfaces)
       ↓
SQLC Generated Code (Type Safety)
       ↓
PostgreSQL Database (ACID Transactions)
       ↓
Atlas Migrations (Schema Management)
```

### Component Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Domain        │    │   Repository    │    │   Generated     │
│  Interfaces     │───▶│  Adapters       │───▶│  SQLC Code      │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Clean         │    │   Data          │    │   Type-Safe     │
│ Architecture    │    │  Mapping        │    │   Queries       │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Domain         │    │   PostgreSQL    │    │    Atlas        │
│  Models         │    │   Database      │    │  Migrations     │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Database Schema

### Core Tables

#### Accounts & Applications

```sql
-- Multi-tenant account structure
CREATE TABLE accounts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  TEXT NOT NULL,
    subscription_status   subscription_status,
    stripe_customer_id    TEXT,
    stripe_subscription_id TEXT,
    trial_ends_at         TIMESTAMP,
    plan                  TEXT NOT NULL DEFAULT 'free',
    created_at            TIMESTAMP NOT NULL DEFAULT now(),
    updated_at            TIMESTAMP NOT NULL DEFAULT now()
);

-- Applications belong to accounts
CREATE TABLE apps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id  UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    api_key     TEXT NOT NULL UNIQUE,
    config      JSON,
    created_at  TIMESTAMP NOT NULL DEFAULT now(),
    updated_at  TIMESTAMP NOT NULL DEFAULT now()
);
```

#### Session Management

```sql
-- Real-time session tracking
CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    closed_at  TIMESTAMP
);

-- Session transcripts
CREATE TABLE transcripts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    content    JSON NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

-- Extracted function calls
CREATE TABLE function_calls (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    args       JSON NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
```

#### Dynamic Function Schemas

```sql
-- Function definitions with versioning
CREATE TABLE function_schemas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id      UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    session_id  UUID REFERENCES sessions(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    description TEXT,
    parameters  JSON NOT NULL, -- JSON Schema
    checksum    TEXT NOT NULL, -- For deduplication
    created_at  TIMESTAMP DEFAULT now(),

    UNIQUE(app_id, checksum) -- Prevent duplicate schemas
);

-- Many-to-many session-schema relationships
CREATE TABLE session_function_schemas (
    session_id         UUID REFERENCES sessions(id) ON DELETE CASCADE,
    function_schema_id UUID REFERENCES function_schemas(id) ON DELETE CASCADE,
    PRIMARY KEY (session_id, function_schema_id)
);
```

#### Batch Processing

```sql
-- Batch job queue and status tracking
CREATE TABLE batch_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id       UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    account_id   UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    status       batch_job_status NOT NULL DEFAULT 'queued',
    file_path    TEXT NOT NULL,
    file_size    BIGINT NOT NULL,
    config       JSONB NOT NULL,
    result       JSONB,
    error_message TEXT,
    started_at   TIMESTAMP,
    completed_at TIMESTAMP,
    created_at   TIMESTAMP NOT NULL DEFAULT now(),
    updated_at   TIMESTAMP NOT NULL DEFAULT now()
);
```

#### Usage Tracking & Analytics

```sql
-- Detailed usage logging for billing
CREATE TABLE usage_logs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    app_id     UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    type       TEXT NOT NULL, -- 'stt', 'llm', 'function_call'
    metric     JSON NOT NULL, -- Provider-specific metrics
    logged_at  TIMESTAMP NOT NULL DEFAULT now()
);

-- Aggregated usage for fast billing queries
CREATE TABLE usage_aggregates (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id     UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    app_id         UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    period_start   DATE NOT NULL,
    period_end     DATE NOT NULL,
    stt_seconds    DECIMAL(10,2) DEFAULT 0,
    llm_tokens     BIGINT DEFAULT 0,
    function_calls INTEGER DEFAULT 0,
    total_cost     DECIMAL(10,4) DEFAULT 0,
    created_at     TIMESTAMP DEFAULT now(),

    UNIQUE(account_id, app_id, period_start, period_end)
);
```

### Enums and Types

```sql
-- Subscription lifecycle states
CREATE TYPE subscription_status AS ENUM (
    'incomplete', 'incomplete_expired', 'trialing',
    'active', 'past_due', 'canceled', 'unpaid', 'paused'
);

-- User permission levels
CREATE TYPE user_role AS ENUM ('owner', 'admin', 'user');

-- Batch job processing states
CREATE TYPE batch_job_status AS ENUM (
    'queued', 'processing', 'completed', 'failed'
);

-- Permission system entities
CREATE TYPE entity AS ENUM (
    'account', 'users', 'roles', 'permissions',
    'forms', 'subscriptions', 'billing', 'usage_metrics'
);

-- RBAC actions
CREATE TYPE actions AS ENUM ('create', 'retrieve', 'update', 'delete');
```

### Indexes and Performance

```sql
-- High-frequency query optimization
CREATE INDEX apps_api_key_idx ON apps(api_key);
CREATE INDEX apps_account_idx ON apps(account_id);
CREATE INDEX sessions_app_idx ON sessions(app_id);
CREATE INDEX function_schemas_app_name_idx ON function_schemas(app_id, name);
CREATE INDEX batch_jobs_status_idx ON batch_jobs(status);
CREATE INDEX batch_jobs_created_at_idx ON batch_jobs(created_at);
CREATE INDEX usage_logs_account_date_idx ON usage_logs(account_id, logged_at);
CREATE INDEX usage_logs_session_idx ON usage_logs(session_id);

-- Composite indexes for complex queries
CREATE INDEX function_calls_session_name_idx ON function_calls(session_id, name);
CREATE INDEX usage_aggregates_billing_idx ON usage_aggregates(account_id, period_start, period_end);
```

## Schema Management with Atlas

### Configuration (`atlas.hcl`)

```hcl
env "local" {
  src = "file://internal/infra/db/schema.hcl"
  url = "postgres://postgres:password@localhost:5432/schma_dev?sslmode=disable"
}

env "production" {
  src = "file://internal/infra/db/schema.hcl"
  url = env("DATABASE_URL")
}
```

### Schema Definition (`internal/infra/db/schema.hcl`)

Atlas uses HCL for type-safe schema definitions:

```hcl
table "apps" {
    schema = schema.public

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    column "account_id" {
        type = uuid
        null = false
    }

    column "api_key" {
        type = text
        null = false
    }

    primary_key {
        columns = [column.id]
    }

    foreign_key "app_account_fk" {
        columns     = [column.account_id]
        ref_columns = [table.accounts.column.id]
        on_delete   = "CASCADE"
    }

    index "apps_api_key_idx" {
        columns = [column.api_key]
        unique  = true
    }
}
```

### Migration Workflow

```bash
# Generate migration from schema changes
atlas migrate diff --env local

# Apply migrations to database
atlas migrate apply --env local

# Production deployment
atlas migrate apply --env production --auto-approve
```

### Generated Migrations

Atlas automatically generates SQL migrations:

```sql
-- 20250709164517_initial_schema.sql
-- Create "accounts" table
CREATE TABLE "public"."accounts" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" text NOT NULL,
  "subscription_status" "public"."subscription_status" NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);

-- Create "apps" table
CREATE TABLE "public"."apps" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "account_id" uuid NOT NULL,
  "name" text NOT NULL,
  "api_key" text NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "app_account_fk" FOREIGN KEY ("account_id")
    REFERENCES "public"."accounts" ("id") ON DELETE CASCADE
);
```

## Type-Safe Queries with SQLC

### Configuration (`internal/infra/db/sqlc.yaml`)

```yaml
version: "2"
sql:
    - engine: "postgresql"
      schema: "./migrations"
      queries: "./queries"
      gen:
          go:
              package: "db"
              out: "./generated"
              sql_package: "pgx/v5"
              emit_json_tags: true
              emit_interface: false
```

### Query Definitions (`internal/infra/db/queries/`)

#### App Queries (`apps.sql`)

```sql
-- name: GetAppByAPIKey :one
SELECT id, account_id, name, api_key, config, created_at, updated_at
FROM apps
WHERE api_key = $1;

-- name: GetAppByID :one
SELECT id, account_id, name, api_key, config, created_at, updated_at
FROM apps
WHERE id = $1;

-- name: ListAppsByAccount :many
SELECT id, account_id, name, api_key, config, created_at, updated_at
FROM apps
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

#### Session Queries (`sessions.sql`)

```sql
-- name: CreateSession :one
INSERT INTO sessions (app_id)
VALUES ($1)
RETURNING id, app_id, created_at, closed_at;

-- name: GetSession :one
SELECT id, app_id, created_at, closed_at
FROM sessions
WHERE id = $1;

-- name: CloseSession :exec
UPDATE sessions
SET closed_at = now()
WHERE id = $1;
```

#### Batch Job Queries (`batch_jobs.sql`)

```sql
-- name: CreateBatchJob :one
INSERT INTO batch_jobs (app_id, account_id, file_path, file_size, config)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, app_id, account_id, status, file_path, file_size,
          config, created_at, updated_at;

-- name: GetBatchJob :one
SELECT id, app_id, account_id, status, file_path, file_size,
       config, result, error_message, started_at, completed_at,
       created_at, updated_at
FROM batch_jobs
WHERE id = $1;

-- name: ListQueuedJobs :many
SELECT id, app_id, account_id, status, file_path, file_size,
       config, result, error_message, started_at, completed_at,
       created_at, updated_at
FROM batch_jobs
WHERE status = 'queued'
ORDER BY created_at ASC
LIMIT $1;

-- name: UpdateBatchJobStatus :exec
UPDATE batch_jobs
SET status = $2,
    result = $3,
    error_message = $4,
    updated_at = now(),
    started_at = CASE WHEN $2 = 'processing' THEN now() ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('completed', 'failed') THEN now() ELSE completed_at END
WHERE id = $1;
```

### Generated Go Code

SQLC generates type-safe Go code:

```go
// Generated types
type App struct {
    ID        pgtype.UUID      `json:"id"`
    AccountID pgtype.UUID      `json:"account_id"`
    Name      string           `json:"name"`
    ApiKey    string           `json:"api_key"`
    Config    []byte           `json:"config"`
    CreatedAt pgtype.Timestamp `json:"created_at"`
    UpdatedAt pgtype.Timestamp `json:"updated_at"`
}

// Generated query methods
type Queries struct {
    db DBTX
}

func (q *Queries) GetAppByAPIKey(ctx context.Context, apiKey string) (App, error) {
    const getAppByAPIKey = `-- name: GetAppByAPIKey :one
SELECT id, account_id, name, api_key, config, created_at, updated_at
FROM apps
WHERE api_key = $1`

    row := q.db.QueryRow(ctx, getAppByAPIKey, apiKey)
    var i App
    err := row.Scan(
        &i.ID,
        &i.AccountID,
        &i.Name,
        &i.ApiKey,
        &i.Config,
        &i.CreatedAt,
        &i.UpdatedAt,
    )
    return i, err
}
```

## Repository Pattern Implementation

### Domain Interface

```go
// Domain defines what it needs, not how to get it
type AppFetcher interface {
    FetchAppForAPIKey(ctx context.Context, key string) (AppInfo, error)
    FetchAppByID(ctx context.Context, id pgtype.UUID) (AppInfo, error)
}

type AppInfo struct {
    AppID     pgtype.UUID
    AccountID pgtype.UUID
    Name      string
    CreatedAt pgtype.Timestamp
    UpdatedAt pgtype.Timestamp
}
```

### Infrastructure Implementation

```go
// Repository adapter implements domain interface
type AppRepo struct {
    q *db.Queries // Generated SQLC queries
}

func NewAppRepo(conn *pgx.Conn) *AppRepo {
    return &AppRepo{q: db.New(conn)}
}

func (r *AppRepo) FetchAppForAPIKey(ctx context.Context, key string) (auth.AppInfo, error) {
    row, err := r.q.GetAppByAPIKey(ctx, key)
    if err != nil {
        return auth.AppInfo{}, err
    }

    return mapRowToAppInfo(row)
}

// Data mapping separates DB concerns from domain
func mapRowToAppInfo(row db.App) (auth.AppInfo, error) {
    return auth.AppInfo{
        AppID:     row.ID,
        AccountID: row.AccountID,
        Name:      row.Name,
        CreatedAt: row.CreatedAt,
        UpdatedAt: row.UpdatedAt,
    }, nil
}
```

### Batch Job Repository

```go
type BatchRepo struct {
    q *db.Queries
}

func (r *BatchRepo) Create(ctx context.Context, appID, accountID pgtype.UUID,
    filePath string, fileSize int64, config map[string]any) (batch.Job, error) {

    // Convert config to JSONB
    configBytes, err := json.Marshal(config)
    if err != nil {
        return batch.Job{}, err
    }

    row, err := r.q.CreateBatchJob(ctx, db.CreateBatchJobParams{
        AppID:     appID,
        AccountID: accountID,
        FilePath:  filePath,
        FileSize:  fileSize,
        Config:    configBytes,
    })
    if err != nil {
        return batch.Job{}, err
    }

    return mapDBJobToDomain(row), nil
}

func (r *BatchRepo) UpdateStatus(ctx context.Context, id pgtype.UUID,
    status batch.JobStatus, result map[string]any, errorMsg string) error {

    var resultBytes []byte
    if result != nil {
        var err error
        resultBytes, err = json.Marshal(result)
        if err != nil {
            return err
        }
    }

    return r.q.UpdateBatchJobStatus(ctx, db.UpdateBatchJobStatusParams{
        ID:           id,
        Status:       db.BatchJobStatus(status),
        Result:       resultBytes,
        ErrorMessage: pgtype.Text{String: errorMsg, Valid: errorMsg != ""},
    })
}
```

## Connection Management

### Database Connection (`internal/infra/db/postgres.go`)

```go
func NewConnection(databaseURL string) (*pgx.Conn, error) {
    config, err := pgx.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("failed to parse database URL: %w", err)
    }

    // Connection pool settings
    config.ConnConfig.ConnectTimeout = 10 * time.Second
    config.ConnConfig.RuntimeParams["application_name"] = "schma-server"

    conn, err := pgx.ConnectConfig(context.Background(), config)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }

    // Test connection
    if err := conn.Ping(context.Background()); err != nil {
        conn.Close(context.Background())
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return conn, nil
}
```

### Transaction Management

```go
// Service-level transaction management
func (s *Service) ProcessBatchWithTransaction(ctx context.Context, jobID string) error {
    return s.db.BeginFunc(ctx, func(tx pgx.Tx) error {
        // All operations within single transaction
        queries := db.New(tx)

        // Update job status
        if err := queries.UpdateBatchJobStatus(ctx, /* params */); err != nil {
            return err // Automatic rollback
        }

        // Create usage records
        if err := queries.CreateUsageLog(ctx, /* params */); err != nil {
            return err // Automatic rollback
        }

        // Update aggregates
        if err := queries.UpdateUsageAggregate(ctx, /* params */); err != nil {
            return err // Automatic rollback
        }

        return nil // Automatic commit
    })
}
```

## Performance Optimization

### Query Performance

```sql
-- Optimized billing query with proper indexing
EXPLAIN (ANALYZE, BUFFERS)
SELECT
    SUM(stt_seconds) as total_stt_seconds,
    SUM(llm_tokens) as total_llm_tokens,
    SUM(total_cost) as total_cost
FROM usage_aggregates
WHERE account_id = $1
  AND period_start >= $2
  AND period_end <= $3;

-- Index usage:
-- -> Index Scan using usage_aggregates_billing_idx
--    Index Cond: ((account_id = $1) AND (period_start >= $2) AND (period_end <= $3))
--    Planning Time: 0.123 ms
--    Execution Time: 0.456 ms
```

### Connection Pooling

```go
// Production connection pool configuration
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = time.Minute * 30
config.HealthCheckPeriod = time.Minute
```

### Prepared Statements

SQLC automatically uses prepared statements:

```go
// Compiled once, executed many times
const getAppByAPIKey = `
SELECT id, account_id, name, api_key, config, created_at, updated_at
FROM apps
WHERE api_key = $1`

// Prepared statement cached by pgx
func (q *Queries) GetAppByAPIKey(ctx context.Context, apiKey string) (App, error) {
    row := q.db.QueryRow(ctx, getAppByAPIKey, apiKey)
    // ... scanning logic
}
```

## Data Migration Strategies

### Schema Migrations

```sql
-- Forward migration: Add new column
ALTER TABLE apps ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- Backward migration: Remove column
ALTER TABLE apps DROP COLUMN description;
```

### Data Migrations

```sql
-- Migrate data with new schema
UPDATE function_schemas
SET checksum = md5(parameters::text)
WHERE checksum IS NULL;

-- Add constraint after data migration
ALTER TABLE function_schemas
ADD CONSTRAINT unique_app_checksum UNIQUE (app_id, checksum);
```

### Zero-Downtime Migrations

```sql
-- Step 1: Add nullable column
ALTER TABLE apps ADD COLUMN new_config JSONB;

-- Step 2: Populate data (in application)
UPDATE apps SET new_config = config::jsonb WHERE new_config IS NULL;

-- Step 3: Add NOT NULL constraint
ALTER TABLE apps ALTER COLUMN new_config SET NOT NULL;

-- Step 4: Drop old column
ALTER TABLE apps DROP COLUMN config;

-- Step 5: Rename column
ALTER TABLE apps RENAME COLUMN new_config TO config;
```

## Testing Strategy

### Unit Tests

```go
func TestAppRepo_FetchAppForAPIKey(t *testing.T) {
    db := setupTestDB(t)
    repo := NewAppRepo(db)

    // Create test data
    app := createTestApp(t, db, "test-api-key")

    // Test successful fetch
    result, err := repo.FetchAppForAPIKey(context.Background(), "test-api-key")
    assert.NoError(t, err)
    assert.Equal(t, app.Name, result.Name)

    // Test non-existent key
    _, err = repo.FetchAppForAPIKey(context.Background(), "invalid-key")
    assert.Error(t, err)
    assert.Equal(t, pgx.ErrNoRows, err)
}
```

### Integration Tests

```go
func TestBatchRepo_Integration(t *testing.T) {
    db := setupTestDB(t)
    repo := NewBatchRepo(db)

    appID := createTestApp(t, db)
    accountID := createTestAccount(t, db)

    // Test job creation
    job, err := repo.Create(context.Background(), appID, accountID,
        "/tmp/test.wav", 1024, map[string]any{"test": true})
    assert.NoError(t, err)
    assert.Equal(t, batch.StatusQueued, job.Status)

    // Test status update
    err = repo.UpdateStatus(context.Background(), job.ID,
        batch.StatusCompleted, map[string]any{"result": "success"}, "")
    assert.NoError(t, err)

    // Verify update
    updated, err := repo.Get(context.Background(), job.ID)
    assert.NoError(t, err)
    assert.Equal(t, batch.StatusCompleted, updated.Status)
    assert.Equal(t, "success", updated.Result["result"])
}
```

### Performance Tests

```go
func BenchmarkAppRepo_FetchByAPIKey(b *testing.B) {
    db := setupBenchDB(b)
    repo := NewAppRepo(db)

    // Create test data
    apps := createTestApps(b, db, 1000)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        app := apps[i%len(apps)]
        _, err := repo.FetchAppForAPIKey(context.Background(), app.APIKey)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Monitoring & Observability

### Database Metrics

```go
// Key performance indicators
db_connections_active
db_connections_idle
db_query_duration_seconds{query="GetAppByAPIKey"}
db_query_errors_total{query="CreateBatchJob"}
db_migration_version
db_size_bytes
db_index_usage_ratio
```

### Query Performance Monitoring

```sql
-- Enable query performance tracking
ALTER SYSTEM SET track_activity_query_size = 4096;
ALTER SYSTEM SET pg_stat_statements.track = 'all';

-- Monitor slow queries
SELECT
    query,
    calls,
    total_time,
    mean_time,
    rows
FROM pg_stat_statements
WHERE mean_time > 100 -- Queries slower than 100ms
ORDER BY mean_time DESC;
```

### Health Checks

```go
func (db *Database) HealthCheck(ctx context.Context) error {
    // Connection health
    if err := db.conn.Ping(ctx); err != nil {
        return fmt.Errorf("database ping failed: %w", err)
    }

    // Query health
    var result int
    err := db.conn.QueryRow(ctx, "SELECT 1").Scan(&result)
    if err != nil {
        return fmt.Errorf("database query failed: %w", err)
    }

    // Migration health
    version, err := db.GetMigrationVersion(ctx)
    if err != nil {
        return fmt.Errorf("migration version check failed: %w", err)
    }

    log.Printf("Database healthy: version=%s", version)
    return nil
}
```

The database architecture provides a robust, type-safe foundation for data persistence with automated schema management, query generation, and comprehensive monitoring capabilities.
