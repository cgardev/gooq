package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_relational_test.go exercises subqueries and set operations against the
// real PostgreSQL container, confirming the rendered SQL executes and returns the
// expected rows. The container, the schema, the seeding, and the per-test
// rollback all live in harness_test.go, and these tests bind the default
// (PostgreSQL) dialect, so they never call Using.

func TestPostgresSubqueries(t *testing.T) {
	t.Run("IN (subquery) keeps only the authors of an expensive book", func(t *testing.T) {
		ctx, tx := library(t)

		// Subquery: the author_id of every book priced above forty. Only The C
		// Programming Language (45.00) qualifies, written by Brian Kernighan.
		expensiveAuthors := gooq.Select1(db.Book.AuthorId).
			From(db.Book).
			Where(db.Book.Price.GT(40.00))

		rows, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(db.Author.Id.InSubquery(expensiveAuthors)).
			OrderBy(db.Author.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "select authors with an expensive book", err)

		equal(t, "row count", len(rows), 1)
		equal(t, "the matching author", rows[0].V1, authorKernighan)
	})

	t.Run("NOT IN (subquery) excludes the authors of an expensive book", func(t *testing.T) {
		ctx, tx := library(t)

		expensiveAuthors := gooq.Select1(db.Book.AuthorId).
			From(db.Book).
			Where(db.Book.Price.GT(40.00))

		rows, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(db.Author.Id.NotInSubquery(expensiveAuthors)).
			OrderBy(db.Author.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "select authors without an expensive book", err)

		equal(t, "row count", len(rows), 1)
		equal(t, "the matching author", rows[0].V1, authorDonovan)
	})

	t.Run("EXISTS keeps every author that has at least one book", func(t *testing.T) {
		ctx, tx := library(t)

		// Correlated subquery: a book exists whose author_id equals the author id.
		hasBook := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.AuthorId.EQField(db.Author.Id))

		rows, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(gooq.Exists(hasBook)).
			OrderBy(db.Author.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "select authors that have a book", err)

		equal(t, "row count", len(rows), 2)
		equal(t, "first author", rows[0].V1, authorDonovan)
		equal(t, "second author", rows[1].V1, authorKernighan)
	})

	t.Run("NOT EXISTS filters out every author that has a book", func(t *testing.T) {
		ctx, tx := library(t)

		hasBook := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.AuthorId.EQField(db.Author.Id))

		rows, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(gooq.NotExists(hasBook)).
			Fetch(ctx, tx)
		noError(t, "select authors without any book", err)

		equal(t, "row count", len(rows), 0)
	})

	t.Run("EXISTS with an uncorrelated subquery keeps every author", func(t *testing.T) {
		ctx, tx := library(t)

		// An uncorrelated subquery referencing no outer column: as long as the book
		// table holds any row the predicate is true for every author.
		anyBook := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Price.GT(0))

		rows, err := gooq.Select1(db.Author.Id).
			From(db.Author).
			Where(gooq.Exists(anyBook)).
			Fetch(ctx, tx)
		noError(t, "select authors with any book in the table", err)

		equal(t, "row count", len(rows), 2)
	})
}

func TestPostgresSetOperations(t *testing.T) {
	t.Run("UNION ALL keeps every duplicate row", func(t *testing.T) {
		ctx, tx := library(t)

		// The left side selects the author_id of all three seeded books, which
		// yields [Donovan, Kernighan, Kernighan]. The right side selects the
		// author_id of the single Go book, [Donovan]. UNION ALL keeps every
		// duplicate: three from the left plus one from the right makes four.
		left := gooq.Select1(db.Book.AuthorId).From(db.Book)
		right := gooq.Select1(db.Book.AuthorId).From(db.Book).Where(db.Book.Id.EQ(bookGo))

		all, err := left.UnionAll(right).Fetch(ctx, tx)
		noError(t, "union all fetch", err)
		equal(t, "union all row count", len(all), 4)
	})

	t.Run("UNION removes duplicate rows", func(t *testing.T) {
		ctx, tx := library(t)

		left := gooq.Select1(db.Book.AuthorId).From(db.Book)
		right := gooq.Select1(db.Book.AuthorId).From(db.Book).Where(db.Book.Id.EQ(bookGo))

		deduped, err := left.Union(right).Fetch(ctx, tx)
		noError(t, "union fetch", err)
		equal(t, "union row count", len(deduped), 2)
		for _, r := range deduped {
			if r.V1 != authorDonovan && r.V1 != authorKernighan {
				t.Errorf("unexpected author id %q in union result", r.V1)
			}
		}
	})

	t.Run("INTERSECT keeps only the rows present on both sides", func(t *testing.T) {
		ctx, tx := library(t)

		// The right side is exactly [Donovan], the only author present on both
		// sides, so INTERSECT yields a single deduplicated row.
		left := gooq.Select1(db.Book.AuthorId).From(db.Book)
		right := gooq.Select1(db.Book.AuthorId).From(db.Book).Where(db.Book.Id.EQ(bookGo))

		common, err := left.Intersect(right).Fetch(ctx, tx)
		noError(t, "intersect fetch", err)
		equal(t, "intersect row count", len(common), 1)
		equal(t, "the common author", common[0].V1, authorDonovan)
	})

	t.Run("EXCEPT removes the rows present on the right side", func(t *testing.T) {
		ctx, tx := library(t)

		// EXCEPT subtracts [Donovan] from the deduplicated left set, leaving only
		// Kernighan.
		left := gooq.Select1(db.Book.AuthorId).From(db.Book)
		right := gooq.Select1(db.Book.AuthorId).From(db.Book).Where(db.Book.Id.EQ(bookGo))

		difference, err := left.Except(right).Fetch(ctx, tx)
		noError(t, "except fetch", err)
		equal(t, "except row count", len(difference), 1)
		equal(t, "the remaining author", difference[0].V1, authorKernighan)
	})
}
