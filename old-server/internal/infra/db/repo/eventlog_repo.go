package repo

import (
	"github.com/jackc/pgx/v5"
	db "schma.ai/internal/infra/db/generated"
)

// TODO: Define port for this repo with compile time check.


type EventlogRepo struct {
	q *db.Queries
}

func NewEventlogRepo(conn *pgx.Conn) *EventlogRepo {
	return &EventlogRepo{
		q: db.New(conn),
	}
}
