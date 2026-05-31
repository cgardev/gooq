package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// phase2_sqlite_test.go exercises the function and expression catalog end to end
// against a real, pure-Go SQLite database. Each test seeds the standard library
// fixtures, runs one fluent gooq query bound to the SQLite dialect, and asserts
// the typed result, mirroring the readable style of the rest of the SQLite suite.

// TestPhase2AggregateGroupByHaving verifies that aggregates compose with GROUP BY
// and HAVING: it counts and sums the ratings per book, keeping only the books
// with more than one review, and checks the counts and totals against the seed.
func TestPhase2AggregateGroupByHaving(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// The seed gives the Go book two reviews (ratings 5 and 4) and every other
	// book a single review; restricting to "more than one review" therefore keeps
	// only the Go book, whose ratings sum to 9.
	rows, err := gooq.Select3(db.Review.BookId, gooq.Count(db.Review.Id), gooq.Sum(db.Review.Rating)).
		From(db.Review).
		GroupBy(db.Review.BookId).
		Having(gooq.Count(db.Review.Id).GT(1)).
		OrderBy(db.Review.BookId.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "aggregate group by having", err)

	equal(t, "grouped rows", len(rows), 1)
	equal(t, "book id", rows[0].V1, bookGo)
	equal(t, "review count", rows[0].V2, int64(2))
	equal(t, "rating sum", rows[0].V3, int64(9))

	// Without the HAVING restriction every book appears, proving the aggregate
	// itself counts correctly across the whole grouped set.
	all, err := gooq.Select2(db.Review.BookId, gooq.Count(db.Review.Id)).
		From(db.Review).
		GroupBy(db.Review.BookId).
		OrderBy(db.Review.BookId.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "aggregate group by", err)
	equal(t, "all grouped rows", len(all), 3)
}

// TestPhase2Coalesce verifies COALESCE on a nullable column: books with a NULL
// subtitle fall back to the supplied default, while books with a subtitle keep
// their own value. Because COALESCE never returns NULL here, the result is always
// a valid sql.Null[string].
func TestPhase2Coalesce(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// The Go book has the subtitle "An Idiomatic Guide"; the Practice book has a
	// NULL subtitle and therefore takes the default.
	subtitle := gooq.Coalesce(db.Book.Subtitle, "no subtitle")

	withSubtitle, err := gooq.Select1(subtitle).
		From(db.Book).
		Where(db.Book.Id.EQ(bookGo)).
		Using(gooq.SQLite()).
		FetchSingle(ctx, conn)
	noError(t, "coalesce with subtitle", err)
	equal(t, "subtitle valid", withSubtitle.V1.Valid, true)
	equal(t, "subtitle value", withSubtitle.V1.V, "An Idiomatic Guide")

	withoutSubtitle, err := gooq.Select1(subtitle).
		From(db.Book).
		Where(db.Book.Id.EQ(bookPractice)).
		Using(gooq.SQLite()).
		FetchSingle(ctx, conn)
	noError(t, "coalesce without subtitle", err)
	equal(t, "default valid", withoutSubtitle.V1.Valid, true)
	equal(t, "default value", withoutSubtitle.V1.V, "no subtitle")
}

// TestPhase2Case verifies a searched CASE expression classifies each book by its
// price into a textual bucket, computed entirely in the database.
func TestPhase2Case(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// Prices in the seed: Go 39.99, Practice 29.50, C 45.00. The thresholds map
	// those to "premium", "standard", and "premium" respectively.
	bucket := gooq.Case[string]().
		When(db.Book.Price.GE(40), "premium").
		When(db.Book.Price.GE(30), "standard").
		Else("budget").
		End()

	rows, err := gooq.Select2(db.Book.Id, bucket).
		From(db.Book).
		OrderBy(db.Book.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "case price buckets", err)

	equal(t, "row count", len(rows), 3)
	equal(t, "go bucket", rows[0].V2, "standard")
	equal(t, "practice bucket", rows[1].V2, "budget")
	equal(t, "c bucket", rows[2].V2, "premium")
}
