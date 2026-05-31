package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_locking_test.go exercises the row-locking clauses, which are exclusive
// to PostgreSQL. A full concurrency test is out of scope; these prove that
// FOR UPDATE, FOR SHARE, and FOR UPDATE SKIP LOCKED render and execute inside a
// transaction and return the locked rows. The tests bind the default
// (PostgreSQL) dialect, so they never call Using.

func TestPostgresRowLocking(t *testing.T) {
	t.Run("FOR UPDATE locks and returns the selected rows", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.InPrint.EQ(true)).
			OrderBy(db.Book.Id.Asc()).
			ForUpdate().
			Fetch(ctx, tx)
		noError(t, "select for update", err)

		// The Go and C books are in print, so both are returned and locked.
		equal(t, "locked row count", len(rows), 2)
		equal(t, "first locked row", rows[0].V1, bookGo)
		equal(t, "second locked row", rows[1].V1, bookC)
	})

	t.Run("FOR SHARE locks and returns the selected rows", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			ForShare().
			Fetch(ctx, tx)
		noError(t, "select for share", err)

		equal(t, "shared row count", len(rows), 1)
		equal(t, "shared row", rows[0].V1, bookGo)
	})

	t.Run("FOR UPDATE SKIP LOCKED returns the unlocked rows", func(t *testing.T) {
		ctx, tx := library(t)

		// With no competing transaction holding a lock, SKIP LOCKED skips nothing
		// and returns every matching row; the point is that the clause renders and
		// executes against real PostgreSQL.
		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			OrderBy(db.Book.Id.Asc()).
			ForUpdate().
			SkipLocked().
			Fetch(ctx, tx)
		noError(t, "select for update skip locked", err)

		equal(t, "row count", len(rows), 3)
	})
}
