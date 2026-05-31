package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_metadata_test.go exercises the code generator's richer emission against
// the real PostgreSQL container: the tag table, the book_status enum column, and
// the book_overview view. The database-free key-metadata assertions live in
// sqlite_metadata_test.go (TestGeneratedKeyMetadata), which is dialect-neutral, so
// they are not repeated here. All tests bind the default (PostgreSQL) dialect, so
// they never call Using.

func TestPostgresTagRoundTrip(t *testing.T) {
	t.Run("inserts tags and reads their labels back in order", func(t *testing.T) {
		ctx, tx := library(t)

		_, err := gooq.InsertInto(db.Tag).
			Columns(db.Tag.Id, db.Tag.BookId, db.Tag.Label).
			Values("d0000000-0000-0000-0000-000000000001", bookGo, "classic").
			Values("d0000000-0000-0000-0000-000000000002", bookGo, "go").
			Execute(ctx, tx)
		noError(t, "insert tags", err)

		rows, err := gooq.Select1(db.Tag.Label).
			From(db.Tag).
			Where(db.Tag.BookId.EQ(bookGo)).
			OrderBy(db.Tag.Label.Asc()).
			Fetch(ctx, tx)
		noError(t, "select tags", err)

		equal(t, "tag row count", len(rows), 2)
		equal(t, "first label", rows[0].V1, "classic")
		equal(t, "second label", rows[1].V1, "go")
	})
}

func TestPostgresEnumColumnRoundTrip(t *testing.T) {
	t.Run("round-trips an explicit enum status through the named type", func(t *testing.T) {
		ctx, tx := library(t)

		const outOfPrintBook = "b0000000-0000-0000-0000-0000000000ff"
		_, err := gooq.InsertInto(db.Book).
			Columns(
				db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
				db.Book.PageCount, db.Book.InPrint, db.Book.Status, db.Book.Attributes,
			).
			Values(outOfPrintBook, authorDonovan, "An Out Of Print Title", 1.0,
				int64(10), false, db.BookStatusOutOfPrint,
				doc(t, map[string]any{"format": "paperback"})).
			Execute(ctx, tx)
		noError(t, "insert enum book", err)

		rows, err := gooq.Select1(db.Book.Status).
			From(db.Book).
			Where(db.Book.Id.EQ(outOfPrintBook)).
			Fetch(ctx, tx)
		noError(t, "select enum status", err)

		equal(t, "book row count", len(rows), 1)
		equal(t, "status", rows[0].V1, db.BookStatusOutOfPrint)

		// The seeded books take the schema default of 'in_print', proving the enum
		// reads back through the named type for the unspecified case too.
		seeded, err := gooq.Select1(db.Book.Status).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "select default enum status", err)
		equal(t, "default status", seeded.V1, db.BookStatusInPrint)
	})
}

func TestPostgresViewQuery(t *testing.T) {
	t.Run("queries the generated book_overview view", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select2(db.BookOverview.Title, db.BookOverview.AuthorName).
			From(db.BookOverview).
			OrderBy(db.BookOverview.Title.Asc()).
			Fetch(ctx, tx)
		noError(t, "select view", err)

		equal(t, "view row count", len(rows), 3)
		// View columns are reported as nullable by the catalog, so the generated
		// accessors wrap them in sql.Null. The first title alphabetically is "The C
		// Programming Language" by Brian Kernighan.
		if !rows[0].V1.Valid || !rows[0].V2.Valid {
			t.Fatalf("view row carries unexpected NULLs: %+v", rows[0])
		}
		equal(t, "first title", rows[0].V1.V, "The C Programming Language")
		equal(t, "first author", rows[0].V2.V, "Brian Kernighan")
	})
}
