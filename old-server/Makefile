.PHONY: test test-unit test-integration test-benchmark test-coverage
.PHONY: db-migrate db-generate db-apply db-sync db-dev db-reset
.PHONY: dev build run

# Database configuration
DB_DIR := internal/infra/db
SCHEMA_FILE := $(DB_DIR)/schema.hcl
MIGRATIONS_DIR := $(DB_DIR)/migrations
SUPABASE_MIGRATIONS_DIR := $(DB_DIR)/supabase/migrations
DEV_DB_URL := postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable

# =============================================================================
# Database Management
# =============================================================================

# Generate a new migration from schema changes
db-migrate:
	@echo "🔄 Generating new migration..."
	@cd $(DB_DIR) && atlas migrate diff \
		--dev-url "$(DEV_DB_URL)" \
		--to "file://./schema.hcl" \
		--dir "file://./migrations" \
		"$(name)"
	@echo "✅ Migration generated successfully"

# Generate SQLC models
db-generate:
	@echo "🔄 Generating SQLC models..."
	@cd $(DB_DIR) && sqlc generate
	@echo "✅ SQLC models generated successfully"

# Apply migrations to development database
db-apply:
	@echo "🔄 Applying migrations to development database..."
	@cd $(DB_DIR) && atlas schema apply \
		--url "$(DEV_DB_URL)" \
		--to "file://./schema.hcl"
	@echo "✅ Migrations applied successfully"

# Sync migrations to Supabase
db-sync:
	@echo "🔄 Syncing migrations to Supabase..."
	@cp -r $(MIGRATIONS_DIR)/* $(SUPABASE_MIGRATIONS_DIR)/
	@cd $(DB_DIR) && supabase db push
	@echo "✅ Migrations synced to Supabase successfully"

# Full database development workflow
db-dev: db-migrate db-generate db-apply db-sync
	@echo "🎉 Database development workflow completed!"

# Reset database (careful!)
db-reset:
	@echo "⚠️  Resetting database..."
	@read -p "Are you sure you want to reset the database? [y/N]: " confirm && [ "$$confirm" = "y" ]
	@cd $(DB_DIR) && supabase db reset
	@echo "✅ Database reset completed"

# Quick database commands for common workflows
db-quick-migrate:
	@if [ -z "$(name)" ]; then \
		echo "❌ Please provide a migration name: make db-quick-migrate name=your_migration_name"; \
		exit 1; \
	fi
	@echo "🚀 Running quick migration workflow..."
	@$(MAKE) db-migrate name=$(name)
	@$(MAKE) db-generate
	@$(MAKE) db-sync
	@echo "🎉 Quick migration completed!"

# =============================================================================
# Development
# =============================================================================

# Start development server
dev:
	@echo "🚀 Starting development server..."
	go run ./cmd/schma

# Build the application
build:
	@echo "🔨 Building application..."
	go build -o bin/schma ./cmd/schma

# Run the built application
run: build
	@echo "🚀 Running application..."
	./bin/schma

# =============================================================================
# Testing
# =============================================================================

# Run all tests
test: test-unit test-integration

# Run unit tests only
test-unit:
	go test -v ./internal/auth/... ./internal/domain/... ./internal/app/...

# Run integration tests
test-integration:
	go test -v -tags=integration ./internal/infra/...

# Run benchmarks
test-benchmark:
	go test -bench=. ./internal/auth/...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run tests in CI mode
test-ci:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Setup test database
test-db-setup:
	docker run --name schma-test-db -e POSTGRES_DB=schma_test -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -p 5433:5432 -d postgres:15

# Cleanup test database
test-db-cleanup:
	docker stop schma-test-db || true
	docker rm schma-test-db || true

# Test auth components
test-auth:
	go test -v ./internal/domain/auth/... ./internal/app/auth/... ./internal/infra/auth/... ./internal/transport/http/...

# Test auth with coverage
test-auth-coverage:
	go test -v -coverprofile=auth-coverage.out ./internal/domain/auth/... ./internal/app/auth/... ./internal/infra/auth/... ./internal/transport/http/...
	go tool cover -html=auth-coverage.out -o auth-coverage.html

# Benchmark auth components
bench-auth:
	go test -bench=. ./internal/infra/auth/... ./internal/app/auth/...

# Run all tests
test-all:
	go test -v ./...
	go test -v -tags=integration ./internal/infra/...

# =============================================================================
# Utilities
# =============================================================================

# Show help
help:
	@echo "📖 Schma.ai Development Commands"
	@echo ""
	@echo "🗄️  Database Commands:"
	@echo "  db-migrate name=<name>    Generate new migration"
	@echo "  db-generate              Generate SQLC models"
	@echo "  db-apply                 Apply migrations to dev DB"
	@echo "  db-sync                  Sync migrations to Supabase"
	@echo "  db-dev                   Full database workflow"
	@echo "  db-quick-migrate name=<name>  Quick migration workflow"
	@echo "  db-reset                 Reset database (careful!)"
	@echo ""
	@echo "🚀 Development Commands:"
	@echo "  dev                      Start development server"
	@echo "  build                    Build application"
	@echo "  run                      Build and run application"
	@echo ""
	@echo "🧪 Testing Commands:"
	@echo "  test                     Run all tests"
	@echo "  test-unit               Run unit tests"
	@echo "  test-integration        Run integration tests"
	@echo "  test-coverage           Run tests with coverage"
	@echo "  test-auth               Test auth components"
	@echo ""
	@echo "📝 Example Usage:"
	@echo "  make db-quick-migrate name=add_batch_jobs"
	@echo "  make db-dev"
	@echo "  make dev"

# =============================================================================
# Documentation
# =============================================================================

docs-seed:
	@mkdir -p docs/references/schema
	# Generate package docs
	gomarkdoc ./... -o docs/references/packages.md
	# Inspect DB schema
	@if [ -n "$$DATABASE_URL" ]; then atlas schema inspect -u $$DATABASE_URL > docs/references/schema/schema.sql; else echo "DATABASE_URL not set; skipping atlas inspect"; fi

docs-validate:
	@echo "Run link checks & basic sanity (optional lychee)"
	# lychee --offline --config .lychee.toml docs || true

docs-build: docs-seed docs-validate
	@echo "Docs build complete"

# Default target
.DEFAULT_GOAL := help