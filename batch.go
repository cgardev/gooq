package gooq

import (
	"context"
	"database/sql"
)

// BatchExec renders and executes each statement in order against the given
// Querier, returning the result of every executed statement. Each step is any
// value exposing an SQL method, which every terminal INSERT, UPDATE, and DELETE
// builder satisfies. Execution stops at the first error: the slice returned
// alongside a non-nil error holds the results of the steps that ran before the
// failure. Rendering and execution share no transaction; wrap the call in one by
// passing a *sql.Tx as the Querier.
func BatchExec(ctx context.Context, db Querier, steps ...interface {
	SQL() (string, []any, error)
}) ([]sql.Result, error) {
	results := make([]sql.Result, 0, len(steps))
	for _, step := range steps {
		query, args, err := step.SQL()
		if err != nil {
			return results, err
		}
		result, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}
