package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxFunc runs inside a database transaction.
type TxFunc[T any] func(ctx context.Context, tx pgx.Tx) (T, error)

// WithTx runs fn in a transaction, rolling back on panic/error.
func WithTx[T any](ctx context.Context, pool *pgxpool.Pool, fn TxFunc[T]) (_ T, err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return *new(T), fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	result, err := fn(ctx, tx)
	if err != nil {
		return *new(T), err
	}

	if err := tx.Commit(ctx); err != nil {
		return *new(T), fmt.Errorf("commit tx: %w", err)
	}

	return result, nil
}

// WithTxRetry retries serializable/deadlock errors up to maxRetries.
func WithTxRetry[T any](ctx context.Context, pool *pgxpool.Pool, maxRetries int, fn TxFunc[T]) (T, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := WithTx(ctx, pool, fn)
		if err == nil {
			return result, nil
		}
		if !isRetryableTxErr(err) || attempt == maxRetries {
			return *new(T), err
		}
		lastErr = err
	}
	return *new(T), lastErr
}

func isRetryableTxErr(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	// serialization_failure or deadlock_detected
	return pgErr.Code == "40001" || pgErr.Code == "40P01"
}
