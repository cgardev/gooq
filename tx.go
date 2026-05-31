package gooq

import (
	"context"
	"database/sql"
	"errors"
)

// WithTx runs fn inside a database transaction. The transaction is started with
// BeginTx and is committed when fn returns a nil error. When fn returns an
// error the transaction is rolled back and the returned error joins fn's error
// with any rollback error. When fn panics the transaction is rolled back and
// the panic is re-raised after the rollback completes.
//
// The *sql.Tx passed to fn satisfies Querier, so every statement builder can be
// executed against it and participates in the same transaction.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			// Roll back before re-raising so the connection is not left with an
			// open transaction. A rollback error is intentionally discarded
			// because the panic is the primary failure.
			_ = tx.Rollback()
			panic(recovered)
		}
	}()

	if err = fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
