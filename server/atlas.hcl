env "default" {
  url = "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable"
  migration {
    dir = "file://internal/infra/db/postgres/migrations"
  }
}

env "supabase" {
  url = "postgresql://postgres.rwipujxpkhfjbyezdydw:uimyN3fhKGAyEDKK@aws-1-ap-southeast-2.pooler.supabase.com:5432/postgres"
  dev = "postgres://postgres:postgres@localhost:54321/postgres?sslmode=disable"
  migration {
    dir = "file://internal/infra/db/postgres/migrations"
  }
}
