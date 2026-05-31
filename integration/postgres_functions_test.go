package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_functions_test.go exercises the function catalog end to end against the
// real PostgreSQL container: aggregates with GROUP BY and HAVING, COALESCE on a
// nullable column, a searched CASE expression, and CAST. All tests bind the
// default (PostgreSQL) dialect, so they never call Using.

func TestPostgresAggregateGroupByHaving(t *testing.T) {
	t.Run("counts and sums ratings per book, filtered by HAVING", func(t *testing.T) {
		ctx, tx := library(t)

		// The seed gives the Go book two reviews (ratings 5 and 4) and every other
		// book a single review; restricting to "more than one review" keeps only the
		// Go book, whose ratings sum to 9.
		rows, err := gooq.Select3(db.Review.BookId, gooq.Count(db.Review.Id), gooq.Sum(db.Review.Rating)).
			From(db.Review).
			GroupBy(db.Review.BookId).
			Having(gooq.Count(db.Review.Id).GT(1)).
			OrderBy(db.Review.BookId.Asc()).
			Fetch(ctx, tx)
		noError(t, "aggregate group by having", err)

		equal(t, "grouped rows", len(rows), 1)
		equal(t, "book id", rows[0].V1, bookGo)
		equal(t, "review count", rows[0].V2, int64(2))
		equal(t, "rating sum", rows[0].V3, int64(9))

		// Without the HAVING restriction every book appears.
		all, err := gooq.Select2(db.Review.BookId, gooq.Count(db.Review.Id)).
			From(db.Review).
			GroupBy(db.Review.BookId).
			OrderBy(db.Review.BookId.Asc()).
			Fetch(ctx, tx)
		noError(t, "aggregate group by", err)
		equal(t, "all grouped rows", len(all), 3)
	})

	t.Run("averages ratings per single-review book", func(t *testing.T) {
		ctx, tx := library(t)

		// The Practice and C books each have exactly one review (ratings 5 and 3),
		// so their averages are whole numbers. PostgreSQL's avg(integer) returns
		// NUMERIC, so the average is cast to INTEGER to scan into the int64 field;
		// the cast is exact here because each group holds a single integral value.
		average := gooq.Cast[int64](gooq.Avg(db.Review.Rating), "integer")

		rows, err := gooq.Select2(db.Review.BookId, average).
			From(db.Review).
			Where(db.Review.BookId.In(bookPractice, bookC)).
			GroupBy(db.Review.BookId).
			OrderBy(db.Review.BookId.Asc()).
			Fetch(ctx, tx)
		noError(t, "aggregate average", err)

		equal(t, "grouped rows", len(rows), 2)
		equal(t, "practice average", rows[0].V2, int64(5))
		equal(t, "c average", rows[1].V2, int64(3))
	})
}

func TestPostgresCoalesce(t *testing.T) {
	t.Run("falls back to the default for a NULL subtitle", func(t *testing.T) {
		ctx, tx := library(t)

		// The Go book has the subtitle "An Idiomatic Guide"; the Practice book has a
		// NULL subtitle and therefore takes the default.
		subtitle := gooq.Coalesce(db.Book.Subtitle, "no subtitle")

		withSubtitle, err := gooq.Select1(subtitle).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "coalesce with subtitle", err)
		equal(t, "subtitle valid", withSubtitle.V1.Valid, true)
		equal(t, "subtitle value", withSubtitle.V1.V, "An Idiomatic Guide")

		withoutSubtitle, err := gooq.Select1(subtitle).
			From(db.Book).
			Where(db.Book.Id.EQ(bookPractice)).
			FetchSingle(ctx, tx)
		noError(t, "coalesce without subtitle", err)
		equal(t, "default valid", withoutSubtitle.V1.Valid, true)
		equal(t, "default value", withoutSubtitle.V1.V, "no subtitle")
	})
}

func TestPostgresCase(t *testing.T) {
	t.Run("classifies each book by price into a textual bucket", func(t *testing.T) {
		ctx, tx := library(t)

		// Prices in the seed: Go 39.99, Practice 29.50, C 45.00. The thresholds map
		// those to "standard", "budget", and "premium" respectively.
		bucket := gooq.Case[string]().
			When(db.Book.Price.GE(40), "premium").
			When(db.Book.Price.GE(30), "standard").
			Else("budget").
			End()

		rows, err := gooq.Select2(db.Book.Id, bucket).
			From(db.Book).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "case price buckets", err)

		equal(t, "row count", len(rows), 3)
		equal(t, "go bucket", rows[0].V2, "standard")
		equal(t, "practice bucket", rows[1].V2, "budget")
		equal(t, "c bucket", rows[2].V2, "premium")
	})
}

func TestPostgresCast(t *testing.T) {
	t.Run("casts an integer page count round-trips its value", func(t *testing.T) {
		ctx, tx := library(t)

		// CAST(page_count AS BIGINT) keeps the int64 type while proving the CAST
		// renders and executes against PostgreSQL.
		casted := gooq.Cast[int64](db.Book.PageCount, "bigint")

		row, err := gooq.Select1(casted).
			From(db.Book).
			Where(db.Book.Id.EQ(bookGo)).
			FetchSingle(ctx, tx)
		noError(t, "cast page count", err)
		equal(t, "casted page count", row.V1, int64(380))
	})
}
