env "default" {
  url = "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable"
  migration {
    dir = "file://internal/infra/db/postgres/migrations"
  }
}

env "supabase" {
  url = getenv("SUPABASE_DATABASE_URL")
  dev = "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable"
  migration {
    dir = "file://internal/infra/db/postgres/migrations"
  }
}
