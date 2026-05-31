package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// sqlite_relational_test.go exercises subqueries and set operations against a
// real, pure-Go SQLite database, confirming that the rendered SQL executes and
// returns the expected rows. It reuses the SQLite harness from sqlite_test.go
// (sqliteLibrary, the fixture identifiers, and the assertion helpers).

// TestSQLiteInSubquery confirms that an IN (SELECT ...) predicate filters to the
// correct rows. It selects the authors who wrote at least one book priced above
// forty, which matches only Brian Kernighan (the author of The C Programming
// Language at 45.00).
func TestSQLiteInSubquery(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// Subquery: the author_id of every book priced above forty.
	expensiveAuthors := gooq.Select1(db.Book.AuthorId).
		From(db.Book).
		Where(db.Book.Price.GT(40.00))

	rows, err := gooq.Select1(db.Author.Id).
		From(db.Author).
		Where(db.Author.Id.InSubquery(expensiveAuthors)).
		OrderBy(db.Author.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select authors with an expensive book", err)

	equal(t, "row count", len(rows), 1)
	equal(t, "the matching author", rows[0].V1, authorKernighan)
}

// TestSQLiteExists confirms that an EXISTS subquery filters to the correct rows.
// It selects every author that has at least one book; both seeded authors do.
func TestSQLiteExists(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// Correlated subquery: a book exists whose author_id equals the author id.
	hasBook := gooq.Select1(db.Book.Id).
		From(db.Book).
		Where(db.Book.AuthorId.EQField(db.Author.Id))

	rows, err := gooq.Select1(db.Author.Id).
		From(db.Author).
		Where(gooq.Exists(hasBook)).
		OrderBy(db.Author.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select authors that have a book", err)

	equal(t, "row count", len(rows), 2)
	equal(t, "first author", rows[0].V1, authorDonovan)
	equal(t, "second author", rows[1].V1, authorKernighan)
}

// TestSQLiteNotExists confirms that NOT EXISTS filters out authors that have a
// book, leaving none, since both seeded authors are credited with at least one.
func TestSQLiteNotExists(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	hasBook := gooq.Select1(db.Book.Id).
		From(db.Book).
		Where(db.Book.AuthorId.EQField(db.Author.Id))

	rows, err := gooq.Select1(db.Author.Id).
		From(db.Author).
		Where(gooq.NotExists(hasBook)).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select authors without any book", err)

	equal(t, "row count", len(rows), 0)
}

// TestSQLiteUnionDedupVsUnionAll confirms that UNION removes duplicate rows while
// UNION ALL keeps them, fetching the merged RecordN slice from both forms.
func TestSQLiteUnionDedupVsUnionAll(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// Left side selects the author_id of all three seeded books, which yields
	// [Donovan, Kernighan, Kernighan] (Kernighan wrote two of them). Right side
	// selects the author_id of the single Go book, which yields [Donovan].
	left := gooq.Select1(db.Book.AuthorId).
		From(db.Book)
	right := gooq.Select1(db.Book.AuthorId).
		From(db.Book).
		Where(db.Book.Id.EQ(bookGo))

	// UNION ALL keeps every duplicate row: three from the left plus one from the
	// right makes four.
	all, err := left.UnionAll(right).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "union all fetch", err)
	equal(t, "union all row count", len(all), 4)

	// UNION removes duplicates, leaving the two distinct author identifiers.
	deduped, err := left.Union(right).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "union fetch", err)
	equal(t, "union row count", len(deduped), 2)
	for _, r := range deduped {
		if r.V1 != authorDonovan && r.V1 != authorKernighan {
			t.Errorf("unexpected author id %q in union result", r.V1)
		}
	}
}
