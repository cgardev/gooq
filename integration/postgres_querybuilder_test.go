package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_querybuilder_test.go exercises the query-builder quick wins end to end
// against the real PostgreSQL container: SELECT DISTINCT, the free-function And,
// Or, and Not combinators, NULLS ordering, and the verbatim RawCondition escape
// hatch. All tests bind the default (PostgreSQL) dialect, so they never call
// Using.

func TestPostgresDistinct(t *testing.T) {
	t.Run("collapses duplicate author identifiers across the seeded books", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.AuthorId).
			Distinct().
			From(db.Book).
			OrderBy(db.Book.AuthorId.Asc()).
			Fetch(ctx, tx)
		noError(t, "select distinct author ids", err)

		// The three seeded books are written by two distinct authors.
		equal(t, "distinct author count", len(rows), 2)
	})
}

func TestPostgresCombinators(t *testing.T) {
	t.Run("composes And, Or, and Not free functions", func(t *testing.T) {
		ctx, tx := library(t)

		// Kernighan wrote two seeded books: The Practice of Programming (not in
		// print, 29.50) and The C Programming Language (in print, 45.00). The inner
		// Or keeps the C book; Not excludes the cheap Practice book either way.
		cond := gooq.And(
			db.Book.AuthorId.EQ(authorKernighan),
			gooq.Or(db.Book.InPrint.EQ(true), db.Book.Price.GT(40.00)),
			gooq.Not(db.Book.Price.LT(30.00)),
		)

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(cond).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "combinator filter", err)

		equal(t, "combinator rows", len(rows), 1)
		equal(t, "the matching book", rows[0].V1, bookC)
	})
}

func TestPostgresOrderNulls(t *testing.T) {
	t.Run("places NULL published_at rows first", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select2(db.Book.Id, db.Book.PublishedAt).
			From(db.Book).
			OrderBy(db.Book.PublishedAt.Asc().NullsFirst()).
			Fetch(ctx, tx)
		noError(t, "order by nulls first", err)

		equal(t, "row count", len(rows), 3)
		// Two seeded books have a NULL published_at; they sort ahead of the one with
		// a concrete timestamp.
		equal(t, "first row null", rows[0].V2.Valid, false)
		equal(t, "second row null", rows[1].V2.Valid, false)
		equal(t, "last row present", rows[2].V2.Valid, true)
	})

	t.Run("places NULL published_at rows last", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select2(db.Book.Id, db.Book.PublishedAt).
			From(db.Book).
			OrderBy(db.Book.PublishedAt.Asc().NullsLast()).
			Fetch(ctx, tx)
		noError(t, "order by nulls last", err)

		equal(t, "row count", len(rows), 3)
		// The single dated book sorts ahead of the two NULL rows.
		equal(t, "first row present", rows[0].V2.Valid, true)
		equal(t, "second row null", rows[1].V2.Valid, false)
		equal(t, "last row null", rows[2].V2.Valid, false)
	})
}

func TestPostgresRawCondition(t *testing.T) {
	t.Run("filters with a verbatim raw condition", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(gooq.RawCondition(`"book"."price" > 40`)).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "raw condition filter", err)

		// Only The C Programming Language (45.00) exceeds 40.
		equal(t, "raw condition rows", len(rows), 1)
		equal(t, "the matching book", rows[0].V1, bookC)
	})
}
