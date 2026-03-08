package db

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func MustOpen(ctx context.Context, dsn string) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		panic(err)
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return pool
}

// Optional helper if you run inside Supabase Docker container locally.
func FromEnv(ctx context.Context) *pgxpool.Pool {
	return MustOpen(ctx, os.Getenv("SUPABASE_DB_URL"))
}
