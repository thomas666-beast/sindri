package dbadapter

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type Querier struct {
	db *sql.DB
}

func New(pool *pgxpool.Pool) *Querier {
	return &Querier{db: stdlib.OpenDBFromPool(pool)}
}

func (q *Querier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q *Querier) Close() error { return q.db.Close() }
