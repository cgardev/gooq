package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// These tests prove the result-mapping, typed RETURNING, and transaction helpers
// against a real, pure-Go SQLite database. They reuse the SQLite harness from
// sqlite_test.go (sqliteLibrary, the fixture identifiers, and the assertion
// helpers), so each test starts from the same seeded library.

// bookView is a destination struct for FetchInto. It mixes an explicit db tag,
// a case-insensitive field-name fallback, and a nullable column mapped through
// sql.Null so the NULL handling path is exercised end to end.
type bookView struct {
	ID       string `db:"id"`
	Title    string // matched case-insensitively to the "title" column
	Price    float64
	Subtitle sql.Null[string] `db:"subtitle"`
}

func TestSQLiteFetchInto(t *testing.T) {
	t.Run("maps selected columns into a struct by db tag and field name", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		query := gooq.Select4(db.Book.Id, db.Book.Title, db.Book.Price, db.Book.Subtitle).
			From(db.Book).
			OrderBy(db.Book.Id.Asc()).
			Using(gooq.SQLite())

		books, err := gooq.FetchInto[bookView](ctx, conn, query)
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

func TestSQLiteFetchMapAndGroups(t *testing.T) {
	t.Run("FetchMap indexes books by their unique id", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		query := gooq.Select4(db.Book.Id, db.Book.Title, db.Book.Price, db.Book.Subtitle).
			From(db.Book).
			Using(gooq.SQLite())

		byID, err := gooq.FetchMap[string, bookView](ctx, conn, query, "id")
		noError(t, "fetch map by id", err)
		equal(t, "map size", len(byID), 3)
		equal(t, "go book title", byID[bookGo].Title, "The Go Programming Language")
		equal(t, "c book title", byID[bookC].Title, "The C Programming Language")
	})

	t.Run("FetchGroups groups reviews by the non-unique book_id", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		query := gooq.Select3(db.Review.Id, db.Review.BookId, db.Review.Rating).
			From(db.Review).
			Using(gooq.SQLite())

		byBook, err := gooq.FetchGroups[string, reviewView](ctx, conn, query, "book_id")
		noError(t, "fetch groups by book id", err)

		// The Go book has two reviews; the Practice and C books have one each.
		equal(t, "go book review count", len(byBook[bookGo]), 2)
		equal(t, "practice book review count", len(byBook[bookPractice]), 1)
		equal(t, "c book review count", len(byBook[bookC]), 1)
	})
}

// reviewView is the destination struct for the FetchGroups case.
type reviewView struct {
	ID     string `db:"id"`
	BookID string `db:"book_id"`
	Rating int64  `db:"rating"`
}

func TestSQLiteFetchOptional(t *testing.T) {
	t.Run("returns false when no row matches", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		_, ok, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ("00000000-0000-0000-0000-000000000000")).
			Using(gooq.SQLite()).
			FetchOptional(ctx, conn)
		noError(t, "fetch optional (no rows)", err)
		equal(t, "present", ok, false)
	})

	t.Run("returns the row and true when exactly one matches", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		row, ok, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			Using(gooq.SQLite()).
			FetchOptional(ctx, conn)
		noError(t, "fetch optional (one row)", err)
		equal(t, "present", ok, true)
		equal(t, "title", row.V1, "The Go Programming Language")
	})

	t.Run("reports too many rows when more than one matches", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		_, ok, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Using(gooq.SQLite()).
			FetchOptional(ctx, conn)
		isError(t, "fetch optional (many rows)", err, gooq.ErrTooManyRows)
		equal(t, "present on error", ok, false)
	})
}

// returnedID is the destination struct for the typed RETURNING cases. Its single
// db-tagged field captures the id projected by the RETURNING clause.
type returnedID struct {
	ID string `db:"id"`
}

func TestSQLiteReturningInto(t *testing.T) {
	t.Run("maps the id returned by an INSERT ... RETURNING", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		const id = "f0000000-0000-0000-0000-000000000010"
		step := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes, db.Book.CreatedAt,
			).
			Values(id, authorKernighan, "Returning Test", 12.50,
				int64(100), true, doc(t, map[string]any{"format": "paperback"}), createdAt).
			Returning(db.Book.Id).
			Using(gooq.SQLite())

		returned, err := gooq.ReturningInto[returnedID](ctx, conn, step)
		noError(t, "insert returning into", err)
		equal(t, "returned count", len(returned), 1)
		equal(t, "returned id", returned[0].ID, id)
	})

	t.Run("maps the id returned by a DELETE ... RETURNING", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		// The C book has a single dependent review; remove it first so the foreign
		// key constraint permits deleting the book.
		_, err := gooq.DeleteFrom(db.Review).
			Where(db.Review.BookId.EQ(bookC)).
			Using(gooq.SQLite()).
			Execute(ctx, conn)
		noError(t, "delete dependent reviews", err)

		step := gooq.DeleteFrom(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Returning(db.Book.Id).
			Using(gooq.SQLite())

		deleted, err := gooq.ReturningOneInto[returnedID](ctx, conn, step)
		noError(t, "delete returning one into", err)
		equal(t, "deleted id", deleted.ID, bookC)

		remaining, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "verify deletion", err)
		equal(t, "remaining", len(remaining), 0)
	})
}

func TestSQLiteWithTx(t *testing.T) {
	const newBookID = "f0000000-0000-0000-0000-000000000020"

	// insertNewBook performs the canonical fixture insert inside a transaction so
	// each transaction case differs only in how the callback resolves.
	insertNewBook := func(ctx context.Context, tx *sql.Tx, title string) error {
		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes, db.Book.CreatedAt,
			).
			Values(newBookID, authorKernighan, title, 1.00,
				int64(10), true, doc(t, map[string]any{"format": "paperback"}), createdAt).
			Using(gooq.SQLite()).
			Execute(ctx, tx)
		return err
	}

	t.Run("commits the transaction when the function returns nil", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		err := gooq.WithTx(ctx, conn, func(tx *sql.Tx) error {
			return insertNewBook(ctx, tx, "Committed")
		})
		noError(t, "with tx commit", err)

		found, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ(newBookID)).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "read committed row", err)
		equal(t, "committed row present", len(found), 1)
	})

	t.Run("rolls back the transaction when the function returns an error", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		sentinel := errors.New("intentional failure")
		err := gooq.WithTx(ctx, conn, func(tx *sql.Tx) error {
			noError(t, "insert inside doomed transaction", insertNewBook(ctx, tx, "Discarded"))
			return sentinel
		})
		isError(t, "with tx rollback", err, sentinel)

		found, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(newBookID)).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "read rolled-back row", err)
		equal(t, "rolled-back row absent", len(found), 0)
	})

	t.Run("rolls back and re-raises when the function panics", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		panicked := func() (recovered bool) {
			defer func() {
				if r := recover(); r != nil {
					recovered = true
				}
			}()
			_ = gooq.WithTx(ctx, conn, func(tx *sql.Tx) error {
				noError(t, "insert before panic", insertNewBook(ctx, tx, "Panicked"))
				panic("boom")
			})
			return false
		}()
		equal(t, "panic propagated", panicked, true)

		found, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(newBookID)).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "read after panic", err)
		equal(t, "panicked row absent", len(found), 0)
	})
}
