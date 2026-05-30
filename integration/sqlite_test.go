package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	// The modernc.org/sqlite driver is a pure-Go, CGO-free SQLite
	// implementation. Blank-importing it registers the "sqlite" driver name with
	// database/sql, so these tests compile and run with CGO_ENABLED=0 and need no
	// Docker. They therefore run even under -short.
	_ "modernc.org/sqlite"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// These tests prove the SQLite dialect end to end against a real, pure-Go SQLite
// database. They mirror the readable style of the PostgreSQL suite: each opens a
// seeded library, runs one fluent gooq query bound to the SQLite dialect, and
// asserts the typed result. Because the gooq builders default to PostgreSQL, the
// dialect must be bound explicitly with Using(gooq.SQLite()) before any terminal
// Fetch or Execute, or rendered with SQLFor(gooq.SQLite()) and run through the
// *sql.DB directly. The SQLite dialect emits "?" placeholders, which modernc
// accepts.

// createdAt is the explicit creation instant supplied to every seeded SQLite
// row. SQLite has no now() default, so the tests provide the timestamp directly.
// The schema declares the timestamp columns as TIMESTAMP, which lets the modernc
// driver round-trip the value back into the generated time.Time and
// sql.Null[time.Time] columns.
var createdAt = time.Date(2024, time.January, 2, 9, 30, 0, 0, time.UTC)

// sqliteSchemaFilePath resolves the SQLite schema definition file relative to
// this source.
func sqliteSchemaFilePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "schema_sqlite.sql")
}

// sqliteDB opens a fresh, isolated SQLite database backed by a temporary file,
// applies the SQLite schema, and registers cleanup. A new database per test
// gives the same clean-slate isolation the PostgreSQL suite gets from its
// per-test transaction rollback. Foreign key enforcement is enabled so the
// schema's references behave like PostgreSQL's.
func sqliteDB(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	ctx := context.Background()

	// A temporary file (rather than ":memory:") keeps the schema visible across
	// every pooled connection the *sql.DB may open.
	dsn := filepath.Join(t.TempDir(), "library.db") + "?_pragma=foreign_keys(1)"
	conn, err := sql.Open("sqlite", dsn)
	noError(t, "open sqlite database", err)
	t.Cleanup(func() { _ = conn.Close() })

	schema, err := os.ReadFile(sqliteSchemaFilePath())
	noError(t, "read sqlite schema", err)
	_, err = conn.ExecContext(ctx, string(schema))
	noError(t, "apply sqlite schema", err)

	return ctx, conn
}

// sqliteLibrary opens a fresh SQLite database already populated with the standard
// fixtures. Unlike the PostgreSQL schema, SQLite has no gen_random_uuid() or
// now() defaults, so every row carries an explicit id and creation timestamp.
func sqliteLibrary(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	ctx, conn := sqliteDB(t)
	sqliteSeed(ctx, t, conn)
	return ctx, conn
}

// sqliteSeed inserts the canonical fixtures through the gooq insert builder bound
// to the SQLite dialect. It mirrors the PostgreSQL seed but supplies explicit
// identifiers and timestamps for every row, since SQLite defines no defaults.
func sqliteSeed(ctx context.Context, t *testing.T, conn gooq.Querier) {
	t.Helper()

	_, err := gooq.InsertInto(db.Author).
		Columns(db.Author.Id, db.Author.Name, db.Author.Email, db.Author.Metadata, db.Author.CreatedAt).
		Values(authorDonovan, "Alan Donovan", "donovan@example.com",
			doc(t, map[string]any{"country": "US", "awards": []any{"Hugo"}}), createdAt).
		Values(authorKernighan, "Brian Kernighan", "kernighan@example.com", noDoc(), createdAt).
		Using(gooq.SQLite()).
		Execute(ctx, conn)
	noError(t, "seed authors", err)

	_, err = gooq.InsertInto(db.Book).
		Columns(
			db.Book.Id, db.Book.AuthorId, db.Book.EditorId, db.Book.Title,
			db.Book.Subtitle, db.Book.Price, db.Book.PageCount, db.Book.InPrint,
			db.Book.Attributes, db.Book.PublishedAt, db.Book.CreatedAt,
		).
		Values(bookGo, authorDonovan, text(authorKernighan), "The Go Programming Language",
			text("An Idiomatic Guide"), 39.99, int64(380), true,
			doc(t, map[string]any{"format": "paperback", "languages": []any{"en", "es"}}),
			moment(publishedGo), createdAt).
		Values(bookPractice, authorKernighan, noText(), "The Practice of Programming",
			noText(), 29.50, int64(267), false,
			doc(t, map[string]any{"format": "hardcover", "languages": []any{"en"}}),
			noMoment(), createdAt).
		Values(bookC, authorKernighan, noText(), "The C Programming Language",
			noText(), 45.00, int64(272), true,
			doc(t, map[string]any{"format": "paperback", "languages": []any{"en", "de"}}),
			noMoment(), createdAt).
		Using(gooq.SQLite()).
		Execute(ctx, conn)
	noError(t, "seed books", err)

	_, err = gooq.InsertInto(db.Review).
		Columns(db.Review.Id, db.Review.BookId, db.Review.Reviewer, db.Review.Rating, db.Review.Body, db.Review.PostedAt).
		Values(reviewGoFirst, bookGo, "Ada", int64(5), text("A modern classic."), createdAt).
		Values(reviewGoSecond, bookGo, "Linus", int64(4), text("Solid and pragmatic."), createdAt).
		Values(reviewPractice, bookPractice, "Grace", int64(5), text("Timeless advice."), createdAt).
		Values(reviewCAnonymous, bookC, "Anonymous", int64(3), noText(), createdAt).
		Using(gooq.SQLite()).
		Execute(ctx, conn)
	noError(t, "seed reviews", err)
}

func TestSQLiteSelect(t *testing.T) {
	t.Run("round-trips a typed two-column select bound to the SQLite dialect", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		rows, err := gooq.Select2(db.Book.Title, db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "select title and price", err)

		equal(t, "count", len(rows), 1)
		equal(t, "title", rows[0].V1, "The Go Programming Language")
		equal(t, "price", rows[0].V2, 39.99)
	})

	t.Run("emits ? placeholders for the SQLite dialect", func(t *testing.T) {
		query, _, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			SQLFor(gooq.SQLite())
		noError(t, "render SQLite SQL", err)
		if !strings.Contains(query, "?") {
			t.Errorf("expected a ? placeholder in the SQLite SQL, got: %s", query)
		}
		if strings.Contains(query, "$1") {
			t.Errorf("did not expect a $1 placeholder in the SQLite SQL, got: %s", query)
		}
	})

	t.Run("joins book to author on the foreign key", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		rows, err := gooq.Select2(db.Book.Title, db.Author.Name).
			From(db.Book).
			InnerJoin(db.Author).On(db.Book.AuthorId.EQField(db.Author.Id)).
			OrderBy(db.Book.Id.Asc()).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "join on foreign key", err)

		equal(t, "count", len(rows), 3)
		equal(t, "first book", rows[0].V1, "The Go Programming Language")
		equal(t, "first author", rows[0].V2, "Alan Donovan")
		equal(t, "third author", rows[2].V2, "Brian Kernighan")
	})

	t.Run("round-trips a nullable timestamp through a TEXT column", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		row, err := gooq.Select1(db.Book.PublishedAt).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		noError(t, "fetch published_at", err)

		equal(t, "valid", row.V1.Valid, true)
		if !row.V1.V.Equal(publishedGo) {
			t.Errorf("published at = %v, want %v", row.V1.V, publishedGo)
		}
	})
}

func TestSQLiteIsNull(t *testing.T) {
	t.Run("partitions books by whether the editor is null", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		withoutEditor, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.EditorId.IsNull()).
			OrderBy(db.Book.Id.Asc()).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "books with no editor", err)
		equal(t, "count without editor", len(withoutEditor), 2)
		equal(t, "first without editor", withoutEditor[0].V1, bookPractice)
		equal(t, "second without editor", withoutEditor[1].V1, bookC)

		withEditor, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.EditorId.IsNotNull()).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "books with an editor", err)
		equal(t, "count with editor", len(withEditor), 1)
		equal(t, "the book with an editor", withEditor[0].V1, bookGo)
	})

	t.Run("filters published_at IS NULL", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		unpublished, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.PublishedAt.IsNull()).
			OrderBy(db.Book.Id.Asc()).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "fetch unpublished", err)
		equal(t, "unpublished count", len(unpublished), 2)
	})
}

func TestSQLiteInsert(t *testing.T) {
	t.Run("inserts a row and reports it was affected", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		const id = "f0000000-0000-0000-0000-000000000001"
		result, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes, db.Book.CreatedAt,
			).
			Values(id, authorKernighan, "Unix: A History and a Memoir", 19.99,
				int64(192), true, doc(t, map[string]any{"format": "paperback"}), createdAt).
			Using(gooq.SQLite()).
			Execute(ctx, conn)
		noError(t, "insert book", err)

		affected, err := result.RowsAffected()
		noError(t, "rows affected", err)
		equal(t, "rows affected", affected, 1)

		row, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ(id)).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		noError(t, "read back inserted title", err)
		equal(t, "inserted title", row.V1, "Unix: A History and a Memoir")
	})

	t.Run("returns the inserted id through RETURNING and QueryRowContext", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		const id = "f0000000-0000-0000-0000-000000000002"
		query, args, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes, db.Book.CreatedAt,
			).
			Values(id, authorKernighan, "Software Tools", 24.99,
				int64(338), false, doc(t, map[string]any{"format": "paperback"}), createdAt).
			Returning(db.Book.Id).
			SQLFor(gooq.SQLite())
		noError(t, "render INSERT ... RETURNING for SQLite", err)

		var returned string
		noError(t, "execute insert returning", conn.QueryRowContext(ctx, query, args...).Scan(&returned))
		equal(t, "returned id", returned, id)
	})
}

func TestSQLiteUpdate(t *testing.T) {
	t.Run("updates a row and reports it was affected", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		result, err := gooq.Update(db.Book).
			Set(db.Book.Price.Set(9.99)).
			Where(db.Book.Id.EQ(bookPractice)).
			Using(gooq.SQLite()).
			Execute(ctx, conn)
		noError(t, "update price", err)

		affected, err := result.RowsAffected()
		noError(t, "rows affected", err)
		equal(t, "rows affected", affected, 1)

		row, err := gooq.Select1(db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookPractice)).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		noError(t, "read back price", err)
		equal(t, "updated price", row.V1, 9.99)
	})
}

func TestSQLiteDelete(t *testing.T) {
	t.Run("deletes a row and returns its id through RETURNING", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		// The C book has one review that references it; remove the review first so
		// the foreign key constraint permits deleting the book.
		_, err := gooq.DeleteFrom(db.Review).
			Where(db.Review.BookId.EQ(bookC)).
			Using(gooq.SQLite()).
			Execute(ctx, conn)
		noError(t, "delete dependent reviews", err)

		query, args, err := gooq.DeleteFrom(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Returning(db.Book.Id).
			SQLFor(gooq.SQLite())
		noError(t, "render DELETE ... RETURNING for SQLite", err)

		var returned string
		noError(t, "execute delete returning", conn.QueryRowContext(ctx, query, args...).Scan(&returned))
		equal(t, "returned id", returned, bookC)

		remaining, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ(bookC)).
			Using(gooq.SQLite()).
			Fetch(ctx, conn)
		noError(t, "verify deletion", err)
		equal(t, "remaining", len(remaining), 0)
	})
}

func TestSQLiteUpsert(t *testing.T) {
	t.Run("ON CONFLICT DO UPDATE overwrites the conflicting row with SetToExcluded", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Attributes, db.Book.CreatedAt,
			).
			Values(bookGo, authorDonovan, "The Go Programming Language, 2nd Edition", 49.99,
				int64(400), true, doc(t, map[string]any{"format": "hardcover"}), createdAt).
			OnConflict(db.Book.Id).
			DoUpdateSet(gooq.SetToExcluded(db.Book.Title), gooq.SetToExcluded(db.Book.Price)).
			Using(gooq.SQLite()).
			Execute(ctx, conn)
		noError(t, "upsert do update", err)

		row, err := gooq.Select2(db.Book.Title, db.Book.Price).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		noError(t, "read back upserted row", err)
		equal(t, "updated title", row.V1, "The Go Programming Language, 2nd Edition")
		equal(t, "updated price", row.V2, 49.99)
	})
}

func TestSQLiteJSON(t *testing.T) {
	t.Run("round-trips a JSON document stored as TEXT", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		row, err := gooq.Select1(db.Author.Metadata).
			From(db.Author).
			Where(db.Author.Id.EQ(authorDonovan)).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		noError(t, "fetch metadata", err)

		var metadata struct {
			Country string   `json:"country"`
			Awards  []string `json:"awards"`
		}
		noError(t, "unmarshal metadata", json.Unmarshal(row.V1, &metadata))
		equal(t, "object key", metadata.Country, "US")
		equal(t, "array length", len(metadata.Awards), 1)
		equal(t, "array element", metadata.Awards[0], "Hugo")
	})
}

func TestSQLiteFetchCardinality(t *testing.T) {
	t.Run("FetchOne returns the zero value when nothing matches", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		row, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			Where(db.Book.Id.EQ("00000000-0000-0000-0000-000000000000")).
			Using(gooq.SQLite()).
			FetchOne(ctx, conn)
		noError(t, "fetch one", err)
		equal(t, "zero title", row.V1, "")
	})

	t.Run("FetchSingle reports no rows", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		_, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Id.EQ("00000000-0000-0000-0000-000000000000")).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		isError(t, "FetchSingle(no rows)", err, sql.ErrNoRows)
	})

	t.Run("FetchSingle reports too many rows", func(t *testing.T) {
		ctx, conn := sqliteLibrary(t)

		_, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Using(gooq.SQLite()).
			FetchSingle(ctx, conn)
		isError(t, "FetchSingle(many rows)", err, gooq.ErrTooManyRows)
	})
}
