package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsRetryableTxErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "serialization failure", err: &pgconn.PgError{Code: "40001"}, want: true},
		{name: "deadlock", err: &pgconn.PgError{Code: "40P01"}, want: true},
		{name: "other pg error", err: &pgconn.PgError{Code: "23505"}, want: false},
		{name: "non pg error", err: errors.New("boom"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryableTxErr(tc.err)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
