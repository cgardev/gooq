package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_result_mapping_test.go proves the result-mapping, typed RETURNING, and
// transaction helpers against the real PostgreSQL container. Most cases run
// inside the per-test transaction from library(t); only WithTx needs a raw
// *sql.DB, so it operates on a dedicated table it creates and drops itself, which
// keeps it from polluting the rest of the suite.

func TestPostgresFetchInto(t *testing.T) {
	t.Run("maps selected columns into a struct by db tag and field name", func(t *testing.T) {
		ctx, tx := library(t)

		query := gooq.Select4(db.Book.Id, db.Book.Title, db.Book.Price, db.Book.Subtitle).
			From(db.Book).
			OrderBy(db.Book.Id.Asc())

		books, err := gooq.FetchInto[bookView](ctx, tx, query)
		noError(t, "fetch into book views", err)
		equal(t, "row count", len(books), 3)

		// The Go book has a subtitle; the other two do not.
		equal(t, "first id", books[0].ID, bookGo)
		equal(t, "first title", books[0].Title, "The Go Programming Language")
		equal(t, "first price", books[0].Price, 39.99)
		equal(t, "first subtitle valid", books[0].Subtitle.Valid, true)
		equal(t, "first subtitle value", books[0].Subtitle.V, "An Idiomatic Guide")

		equal(t, "second subtitle valid", books[1].Subtitle.Valid, false)
	})
}

func TestPostgresFetchMapAndGroups(t *testing.T) {
	t.Run("FetchMap indexes books by their unique id", func(t *testing.T) {
		ctx, tx := library(t)

		query := gooq.Select4(db.Book.Id, db.Book.Title, db.Book.Price, db.Book.Subtitle).
			From(db.Book)

		byID, err := gooq.FetchMap[string, bookView](ctx, tx, query, "id")
		noError(t, "fetch map by id", err)
		equal(t, "map size", len(byID), 3)
		equal(t, "go book title", byID[bookGo].Title, "The Go Programming Language")
		equal(t, "c book title", byID[bookC].Title, "The C Programming Language")
	})

	t.Run("FetchGroups groups reviews by the non-unique book_id", func(t *testing.T) {
		ctx, tx := library(t)

		query := gooq.Select3(db.Review.Id, db.Review.BookId, db.Review.Rating).
			From(db.Review)

		byBook, err := gooq.FetchGroups[string, reviewView](ctx, tx, query, "book_id")
		noError(t, "fetch groups by book id", err)

		// The Go book has two reviews; the Practice and C books have one each.
		equal(t, "go book review count", len(byBook[bookGo]), 2)
		equal(t, "practice book review count", len(byBook[bookPractice]), 1)
		equal(t, "c book review count", len(byBook[bookC]), 1)
	})
}

func TestPostgresFetchOptional(t *testing.T) {
	t.Run("returns false when no row matches", func(t *testing.T) {
		ctx, tx := library(t)

		_, ok, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ("00000000-0000-0000-0000-000000000000")).
			FetchOptional(ctx, tx)
		noError(t, "fetch optional (no rows)", err)
		equal(t, "present", ok, false)
	})

	t.Run("returns the row and true when exactly one matches", func(t *testing.T) {
		ctx, tx := library(t)

		row, ok, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchOptional(ctx, tx)
		noError(t, "fetch optional (one row)", err)
		equal(t, "present", ok, true)
		equal(t, "title", row.V1, "The Go Programming Language")
	})

	t.Run("reports too many rows when more than one matches", func(t *testing.T) {
		ctx, tx := library(t)

		_, ok, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			FetchOptional(ctx, tx)
		isError(t, "fetch optional (many rows)", err, gooq.ErrTooManyRows)
		equal(t, "present on error", ok, false)
	})
}

func TestPostgresReturningInto(t *testing.T) {
	t.Run("maps the id returned by an INSERT ... RETURNING", func(t *testing.T) {
		ctx, tx := library(t)

		const id = "f0000000-0000-0000-0000-000000000010"
		step := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes,
			).
			Values(id, authorKernighan, "Returning Test", 12.50,
				int64(100), true, doc(t, map[string]any{"format": "paperback"})).
			Returning(db.Book.Id)

		returned, err := gooq.ReturningInto[returnedID](ctx, tx, step)
		noError(t, "insert returning into", err)
		equal(t, "returned count", len(returned), 1)
		equal(t, "returned id", returned[0].ID, id)
	})

	t.Run("maps the id returned by a DELETE ... RETURNING", func(t *testing.T) {
		ctx, tx := library(t)

		// The C book has a single dependent review; remove it first so the foreign
		// key constraint permits deleting the book.
		_, err := gooq.DeleteFrom(db.Review).
			Where(db.Review.BookId.EQ(bookC)).
			Execute(ctx, tx)
		noError(t, "delete dependent reviews", err)

		step := gooq.DeleteFrom(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Returning(db.Book.Id)

		deleted, err := gooq.ReturningOneInto[returnedID](ctx, tx, step)
		noError(t, "delete returning one into", err)
		equal(t, "deleted id", deleted.ID, bookC)

		remaining, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Fetch(ctx, tx)
		noError(t, "verify deletion", err)
		equal(t, "remaining", len(remaining), 0)
	})
}

// withTxFixture is the destination struct for the WithTx cases, mapping the
// dedicated table's single row.
type withTxFixture struct {
	ID    int64  `db:"id"`
	Label string `db:"label"`
}

func TestPostgresWithTx(t *testing.T) {
	requireDatabase(t)
	ctx := context.Background()

	// WithTx requires a *sql.DB, which the per-test *sql.Tx is not. Operate on a
	// dedicated table created and dropped here so committed rows never leak into
	// the rest of the suite.
	const create = `CREATE TABLE postgres_with_tx (id BIGINT PRIMARY KEY, label TEXT NOT NULL)`
	const drop = `DROP TABLE IF EXISTS postgres_with_tx`
	_, err := sharedDB.ExecContext(ctx, drop)
	noError(t, "drop pre-existing fixture table", err)
	_, err = sharedDB.ExecContext(ctx, create)
	noError(t, "create fixture table", err)
	t.Cleanup(func() { _, _ = sharedDB.ExecContext(ctx, drop) })

	insertRow := func(tx *sql.Tx, id int64, label string) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO postgres_with_tx (id, label) VALUES ($1, $2)`, id, label)
		return err
	}

	countRows := func(id int64) int {
		var count int
		noError(t, "count fixture rows", sharedDB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM postgres_with_tx WHERE id = $1`, id).Scan(&count))
		return count
	}

	t.Run("commits the transaction when the function returns nil", func(t *testing.T) {
		const id = int64(1)
		err := gooq.WithTx(ctx, sharedDB, func(tx *sql.Tx) error {
			return insertRow(tx, id, "Committed")
		})
		noError(t, "with tx commit", err)
		equal(t, "committed row present", countRows(id), 1)
	})

	t.Run("rolls back the transaction when the function returns an error", func(t *testing.T) {
		const id = int64(2)
		sentinel := errors.New("intentional failure")
		err := gooq.WithTx(ctx, sharedDB, func(tx *sql.Tx) error {
			noError(t, "insert inside doomed transaction", insertRow(tx, id, "Discarded"))
			return sentinel
		})
		isError(t, "with tx rollback", err, sentinel)
		equal(t, "rolled-back row absent", countRows(id), 0)
	})

	t.Run("rolls back and re-raises when the function panics", func(t *testing.T) {
		const id = int64(3)
		panicked := func() (recovered bool) {
			defer func() {
				if r := recover(); r != nil {
					recovered = true
				}
			}()
			_ = gooq.WithTx(ctx, sharedDB, func(tx *sql.Tx) error {
				noError(t, "insert before panic", insertRow(tx, id, "Panicked"))
				panic("boom")
			})
			return false
		}()
		equal(t, "panic propagated", panicked, true)
		equal(t, "panicked row absent", countRows(id), 0)
	})
}
