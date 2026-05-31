package gooq

import "context"

// ReturningInto executes a data-manipulation statement that renders a RETURNING
// clause and maps the returned rows into a slice of the struct type S.
//
// Methods cannot introduce their own type parameters, so the typed RETURNING
// surface is exposed as a free generic function. The INSERT, UPDATE, and DELETE
// builders already render a RETURNING projection when Returning was called and
// already expose a SQL method, so the statement is run through QueryContext and
// its rows are mapped with the same plan FetchInto uses.
func ReturningInto[S any](ctx context.Context, db Querier, step statement) ([]S, error) {
	return FetchInto[S](ctx, db, step)
}

// ReturningOneInto executes a RETURNING statement and returns a single mapped
// row. It returns the zero value of S and a nil error when no row is returned,
// the single row when exactly one is returned, or ErrTooManyRows when more than
// one row is returned.
func ReturningOneInto[S any](ctx context.Context, db Querier, step statement) (S, error) {
	return FetchOneInto[S](ctx, db, step)
}
