package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// sqlite_metadata_test.go exercises the code generator's newer emission against a
// real, pure-Go SQLite database: the tag table (primary key, foreign key, and a
// multi-column unique constraint), the book_status enum column, and the
// book_overview view. It reuses the SQLite harness from sqlite_test.go
// (sqliteLibrary, the fixture identifiers, and the assertion helpers).

// TestSQLiteTagRoundTrip inserts tags referencing a seeded book and reads their
// labels back in order, exercising the generated accessor for the tag table.
func TestSQLiteTagRoundTrip(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	_, err := gooq.InsertInto(db.Tag).
		Columns(db.Tag.Id, db.Tag.BookId, db.Tag.Label).
		Values("d0000000-0000-0000-0000-000000000001", bookGo, "classic").
		Values("d0000000-0000-0000-0000-000000000002", bookGo, "go").
		Using(gooq.SQLite()).
		Execute(ctx, conn)
	noError(t, "insert tags", err)

	rows, err := gooq.Select1(db.Tag.Label).
		From(db.Tag).
		Where(db.Tag.BookId.EQ(bookGo)).
		OrderBy(db.Tag.Label.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select tags", err)

	equal(t, "tag row count", len(rows), 2)
	equal(t, "first label", rows[0].V1, "classic")
	equal(t, "second label", rows[1].V1, "go")
}

// TestSQLiteEnumColumnRoundTrip inserts a book carrying an explicit enum status
// and reads it back through the generated BookStatus named type, exercising the
// enum column emission end to end.
func TestSQLiteEnumColumnRoundTrip(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	const outOfPrintBook = "b0000000-0000-0000-0000-0000000000ff"
	_, err := gooq.InsertInto(db.Book).
		Columns(
			db.Book.Id, db.Book.AuthorId, db.Book.Title, db.Book.Price,
			db.Book.PageCount, db.Book.InPrint, db.Book.Status, db.Book.Attributes,
			db.Book.CreatedAt,
		).
		Values(outOfPrintBook, authorDonovan, "An Out Of Print Title", 1.0,
			int64(10), false, db.BookStatusOutOfPrint,
			doc(t, map[string]any{"format": "paperback"}), createdAt).
		Using(gooq.SQLite()).
		Execute(ctx, conn)
	noError(t, "insert enum book", err)

	rows, err := gooq.Select1(db.Book.Status).
		From(db.Book).
		Where(db.Book.Id.EQ(outOfPrintBook)).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select enum status", err)

	equal(t, "book row count", len(rows), 1)
	equal(t, "status", rows[0].V1, db.BookStatusOutOfPrint)
}

// TestSQLiteViewQuery queries the generated book_overview view, confirming that a
// view accessor is usable in a FROM clause and projects its columns.
func TestSQLiteViewQuery(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	rows, err := gooq.Select2(db.BookOverview.Title, db.BookOverview.AuthorName).
		From(db.BookOverview).
		OrderBy(db.BookOverview.Title.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select view", err)

	equal(t, "view row count", len(rows), 3)
	// View columns are reported as nullable by the catalog, so the generated
	// accessors wrap them in sql.Null. The first title alphabetically is "The C
	// Programming Language" by Brian Kernighan, which the join surfaces through
	// the view's author_name column.
	if !rows[0].V1.Valid || !rows[0].V2.Valid {
		t.Fatalf("view row carries unexpected NULLs: %+v", rows[0])
	}
	equal(t, "first title", rows[0].V1.V, "The C Programming Language")
	equal(t, "first author", rows[0].V2.V, "Brian Kernighan")
}

// TestGeneratedKeyMetadata asserts the static key metadata accessors emitted on
// the generated table types report the primary key, unique constraints, and
// foreign keys discovered during introspection. It needs no database.
func TestGeneratedKeyMetadata(t *testing.T) {
	if got := db.Tag.PrimaryKey(); len(got) != 1 || got[0] != "id" {
		t.Errorf("tag primary key = %v, want [id]", got)
	}

	uniques := db.Tag.Uniques()
	if len(uniques) != 1 || len(uniques[0]) != 2 || uniques[0][0] != "book_id" || uniques[0][1] != "label" {
		t.Errorf("tag uniques = %v, want one constraint over [book_id label]", uniques)
	}

	references := db.Tag.ForeignKeys()
	if len(references) != 1 {
		t.Fatalf("tag references = %v, want one", references)
	}
	ref := references[0]
	if ref.Name != "tag_book_id_fkey" || ref.RefTable != "book" {
		t.Errorf("tag reference = %+v", ref)
	}
	if len(ref.Columns) != 1 || ref.Columns[0] != "book_id" {
		t.Errorf("tag reference columns = %v, want [book_id]", ref.Columns)
	}
	if len(ref.RefColumns) != 1 || ref.RefColumns[0] != "id" {
		t.Errorf("tag reference target columns = %v, want [id]", ref.RefColumns)
	}

	// The book table reports both of its references to author in a deterministic,
	// constraint-name order.
	bookRefs := db.Book.ForeignKeys()
	if len(bookRefs) != 2 {
		t.Fatalf("book references = %v, want two", bookRefs)
	}
	if bookRefs[0].Name != "book_author_id_fkey" || bookRefs[1].Name != "book_editor_id_fkey" {
		t.Errorf("book reference order = %q, %q", bookRefs[0].Name, bookRefs[1].Name)
	}
}
