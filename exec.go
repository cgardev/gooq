package gooq

import (
	"context"
	"database/sql"
)

// Querier is the minimal execution surface the library needs. It is satisfied
// by *sql.DB, *sql.Tx, and *sql.Conn, and by compatible third-party pools such
// as pgx's stdlib adapter.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
