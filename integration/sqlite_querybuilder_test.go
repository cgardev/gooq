package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// sqlite_querybuilder_test.go exercises the query-builder quick wins end to end
// against the pure-Go SQLite database: SELECT DISTINCT, the free-function And and
// Or combinators, NULLS ordering, and the verbatim RawCondition escape hatch.

// TestSQLiteDistinctExec verifies that SELECT DISTINCT executes against SQLite
// and collapses duplicate author identifiers across the seeded books.
func TestSQLiteDistinctExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	rows, err := gooq.Select1(db.Book.AuthorId).
		Distinct().
		From(db.Book).
		OrderBy(db.Book.AuthorId.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "select distinct author ids", err)

	// The three seeded books are written by two distinct authors.
	equal(t, "distinct author count", len(rows), 2)
}

// TestSQLiteCombinatorsExec verifies the free-function And and Or combinators
// against SQLite.
func TestSQLiteCombinatorsExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	cond := gooq.And(
		db.Book.AuthorId.EQ(authorKernighan),
		gooq.Or(db.Book.InPrint.EQ(true), db.Book.Price.GT(40.00)),
	)

	rows, err := gooq.Select1(db.Book.Id).
		From(db.Book).
		Where(cond).
		OrderBy(db.Book.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "combinator filter", err)

	// Kernighan wrote two seeded books: The Practice of Programming (not in
	// print, 29.50) and The C Programming Language (in print, 45.00). Only the
	// latter satisfies the inner Or.
	equal(t, "combinator rows", len(rows), 1)
	equal(t, "the matching book", rows[0].V1, bookC)
}

// TestSQLiteOrderNullsExec verifies that NULLS ordering executes against SQLite,
// placing the rows with a NULL published_at first.
func TestSQLiteOrderNullsExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	rows, err := gooq.Select2(db.Book.Id, db.Book.PublishedAt).
		From(db.Book).
		OrderBy(db.Book.PublishedAt.Asc().NullsFirst()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "order by nulls first", err)

	equal(t, "row count", len(rows), 3)
	// Two seeded books have a NULL published_at; they must sort ahead of the one
	// with a concrete timestamp.
	equal(t, "first row null", rows[0].V2.Valid, false)
	equal(t, "second row null", rows[1].V2.Valid, false)
	equal(t, "last row present", rows[2].V2.Valid, true)
}

// TestSQLiteRawConditionExec verifies that a verbatim RawCondition executes
// against SQLite.
func TestSQLiteRawConditionExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	rows, err := gooq.Select1(db.Book.Id).
		From(db.Book).
		Where(gooq.RawCondition(`"book"."price" > 40`)).
		OrderBy(db.Book.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "raw condition filter", err)

	// Only The C Programming Language (45.00) exceeds 40.
	equal(t, "raw condition rows", len(rows), 1)
	equal(t, "the matching book", rows[0].V1, bookC)
}
