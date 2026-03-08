#!/bin/bash

# Database management script for Schma.ai
# This script provides advanced database operations

set -e

DB_DIR="internal/infra/db"
SCHEMA_FILE="$DB_DIR/schema.hcl"
MIGRATIONS_DIR="$DB_DIR/migrations"
SUPABASE_MIGRATIONS_DIR="$DB_DIR/supabase/migrations"

# Load environment variables if .env exists
if [ -f ".env" ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Default database URL
DEV_DB_URL=${DEV_DATABASE_URL:-"postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if required tools are installed
check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v atlas &> /dev/null; then
        log_error "Atlas is not installed. Please install it first:"
        echo "  curl -sSf https://atlasgo.sh | sh"
        exit 1
    fi
    
    if ! command -v sqlc &> /dev/null; then
        log_error "SQLC is not installed. Please install it first:"
        echo "  go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest"
        exit 1
    fi
    
    if ! command -v supabase &> /dev/null; then
        log_error "Supabase CLI is not installed. Please install it first:"
        echo "  npm install -g supabase"
        exit 1
    fi
    
    log_success "All dependencies are installed"
}

# Generate migration
generate_migration() {
    local name=$1
    if [ -z "$name" ]; then
        log_error "Migration name is required"
        echo "Usage: $0 migrate <migration_name>"
        exit 1
    fi
    
    log_info "Generating migration: $name"
    cd "$DB_DIR"
    
    atlas migrate diff \
        --dev-url "$DEV_DB_URL" \
        --to "file://./schema.hcl" \
        --dir "file://./migrations" \
        "$name"
    
    log_success "Migration '$name' generated successfully"
}

# Generate SQLC models
generate_models() {
    log_info "Generating SQLC models..."
    cd "$DB_DIR"
    sqlc generate
    log_success "SQLC models generated successfully"
}

# Apply migrations to development database
apply_migrations() {
    log_info "Applying migrations to development database..."
    cd "$DB_DIR"
    
    atlas schema apply \
        --url "$DEV_DB_URL" \
        --to "file://./schema.hcl" \
        --auto-approve
    
    log_success "Migrations applied successfully"
}

# Sync migrations to Supabase
sync_supabase() {
    log_info "Syncing migrations to Supabase..."
    
    # Ensure supabase migrations directory exists
    mkdir -p "$SUPABASE_MIGRATIONS_DIR"
    
    # Copy migrations
    if [ -d "$MIGRATIONS_DIR" ] && [ "$(ls -A $MIGRATIONS_DIR)" ]; then
        cp -r "$MIGRATIONS_DIR"/* "$SUPABASE_MIGRATIONS_DIR/"
        log_info "Migrations copied to Supabase directory"
    else
        log_warning "No migrations found to copy"
    fi
    
    # Push to Supabase
    cd "$DB_DIR"
    supabase db push
    
    log_success "Migrations synced to Supabase successfully"
}

# Full development workflow
dev_workflow() {
    local name=$1
    if [ -z "$name" ]; then
        log_error "Migration name is required for dev workflow"
        echo "Usage: $0 dev <migration_name>"
        exit 1
    fi
    
    log_info "Starting full development workflow..."
    generate_migration "$name"
    generate_models
    apply_migrations
    sync_supabase
    log_success "Development workflow completed successfully!"
}

# Reset database
reset_database() {
    log_warning "This will reset your database. All data will be lost!"
    read -p "Are you sure you want to continue? [y/N]: " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Resetting database..."
        cd "$DB_DIR"
        supabase db reset
        log_success "Database reset completed"
    else
        log_info "Database reset cancelled"
    fi
}

# Show status
show_status() {
    log_info "Database Status:"
    echo
    echo "Schema file: $SCHEMA_FILE"
    echo "Migrations directory: $MIGRATIONS_DIR"
    echo "Supabase migrations: $SUPABASE_MIGRATIONS_DIR"
    echo "Development DB URL: $DEV_DB_URL"
    echo
    
    # Count migrations
    if [ -d "$MIGRATIONS_DIR" ]; then
        migration_count=$(ls -1 "$MIGRATIONS_DIR"/*.sql 2>/dev/null | wc -l)
        echo "Atlas migrations: $migration_count"
    else
        echo "Atlas migrations: 0"
    fi
    
    if [ -d "$SUPABASE_MIGRATIONS_DIR" ]; then
        supabase_count=$(ls -1 "$SUPABASE_MIGRATIONS_DIR"/*.sql 2>/dev/null | wc -l)
        echo "Supabase migrations: $supabase_count"
    else
        echo "Supabase migrations: 0"
    fi
}

# Show help
show_help() {
    echo "Database management script for Schma.ai"
    echo
    echo "Usage: $0 <command> [arguments]"
    echo
    echo "Commands:"
    echo "  migrate <name>     Generate a new migration"
    echo "  models             Generate SQLC models"
    echo "  apply              Apply migrations to dev database"
    echo "  sync               Sync migrations to Supabase"
    echo "  dev <name>         Full development workflow"
    echo "  reset              Reset database (careful!)"
    echo "  status             Show database status"
    echo "  deps               Check dependencies"
    echo "  help               Show this help message"
    echo
    echo "Examples:"
    echo "  $0 migrate add_batch_jobs"
    echo "  $0 dev add_user_preferences"
    echo "  $0 status"
}

# Main script logic
main() {
    case "$1" in
        migrate)
            check_dependencies
            generate_migration "$2"
            ;;
        models)
            check_dependencies
            generate_models
            ;;
        apply)
            check_dependencies
            apply_migrations
            ;;
        sync)
            check_dependencies
            sync_supabase
            ;;
        dev)
            check_dependencies
            dev_workflow "$2"
            ;;
        reset)
            check_dependencies
            reset_database
            ;;
        status)
            show_status
            ;;
        deps)
            check_dependencies
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            if [ -z "$1" ]; then
                show_help
            else
                log_error "Unknown command: $1"
                show_help
                exit 1
            fi
            ;;
    esac
}

# Run main function with all arguments
main "$@" 