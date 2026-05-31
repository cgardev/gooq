package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// postgres_expressions_test.go exercises the field-expression operands end to end
// against the real PostgreSQL container: the NOT BETWEEN predicate, field-to-field
// comparators, field-operand arithmetic, unary negation, and string
// concatenation that mixes bound literals with a column operand. All tests bind
// the default (PostgreSQL) dialect, so they never call Using.

func TestPostgresNotBetween(t *testing.T) {
	t.Run("keeps only the prices outside the range", func(t *testing.T) {
		ctx, tx := library(t)

		rows, err := gooq.Select1(db.Book.Id).
			From(db.Book).
			Where(db.Book.Price.NotBetween(35.00, 50.00)).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "not between price range", err)

		// Seeded prices are 39.99, 29.50, and 45.00; only 29.50 falls outside
		// [35.00, 50.00].
		equal(t, "rows outside range", len(rows), 1)
		equal(t, "the matching book", rows[0].V1, bookPractice)
	})
}

func TestPostgresFieldComparator(t *testing.T) {
	t.Run("compares two columns of the same row", func(t *testing.T) {
		ctx, tx := library(t)

		// Joining book to author by identity, the predicate references two
		// identifiers rather than a bound value; every seeded book has a matching
		// author, so all three rows satisfy "author id is not different from the
		// book's author id".
		rows, err := gooq.Select1(db.Book.Title).
			From(db.Book).
			InnerJoin(db.Author).On(db.Book.AuthorId.EQField(db.Author.Id)).
			Where(db.Author.Id.NEField(db.Book.AuthorId).Not()).
			OrderBy(db.Book.Id.Asc()).
			Fetch(ctx, tx)
		noError(t, "field comparator join", err)

		equal(t, "matched rows", len(rows), 3)
		equal(t, "first matched book", rows[0].V1, "The Go Programming Language")
	})
}

func TestPostgresNumericArithField(t *testing.T) {
	t.Run("derives a value from two columns of the same row", func(t *testing.T) {
		ctx, tx := library(t)

		// rating + rating doubles the seeded rating; selecting it back proves the
		// arithmetic renders both operands as identifiers and computes in the
		// database. The first Go review has a rating of 5, so the doubled value is
		// 10.
		doubled := db.Review.Rating.AddField(db.Review.Rating)
		rows, err := gooq.Select1(doubled).
			From(db.Review).
			Where(db.Review.Id.EQ(reviewGoFirst)).
			Fetch(ctx, tx)
		noError(t, "doubled rating", err)

		equal(t, "count", len(rows), 1)
		equal(t, "doubled rating", rows[0].V1, int64(10))
	})
}

func TestPostgresNumericNeg(t *testing.T) {
	t.Run("negates a numeric column", func(t *testing.T) {
		ctx, tx := library(t)

		negated := db.Review.Rating.Neg()
		rows, err := gooq.Select1(negated).
			From(db.Review).
			Where(db.Review.Id.EQ(reviewGoFirst)).
			Fetch(ctx, tx)
		noError(t, "negated rating", err)

		equal(t, "count", len(rows), 1)
		equal(t, "negated rating", rows[0].V1, int64(-5))
	})
}

func TestPostgresConcat(t *testing.T) {
	t.Run("concatenates a column between two bound literals", func(t *testing.T) {
		ctx, tx := library(t)

		// "Author: <name>!" wraps the name column between two bound literals.
		greeting := gooq.Concat("Author: ", db.Author.Name, "!")
		rows, err := gooq.Select1(greeting).
			From(db.Author).
			Where(db.Author.Id.EQ(authorDonovan)).
			Fetch(ctx, tx)
		noError(t, "concatenated name", err)

		equal(t, "count", len(rows), 1)
		equal(t, "concatenation", rows[0].V1, "Author: Alan Donovan!")
	})
}
