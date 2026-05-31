package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// sqlite_expressions_test.go exercises the field-expression operands end to end
// against the pure-Go SQLite database: the NOT BETWEEN predicate, field-to-field
// comparators, field-operand arithmetic, unary negation, and string
// concatenation that mixes bound literals with a column operand.

// TestSQLiteNotBetweenExec verifies the NOT BETWEEN predicate against SQLite.
func TestSQLiteNotBetweenExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	rows, err := gooq.Select1(db.Book.Id).
		From(db.Book).
		Where(db.Book.Price.NotBetween(35.00, 50.00)).
		OrderBy(db.Book.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "not between price range", err)

	// Seeded prices are 39.99, 29.50, and 45.00; only 29.50 falls outside
	// [35.00, 50.00].
	equal(t, "rows outside range", len(rows), 1)
	equal(t, "the matching book", rows[0].V1, bookPractice)
}

// TestSQLiteFieldComparatorExec verifies the field-to-field comparators execute
// against SQLite, comparing two columns of the same row rather than a column
// against a bound value.
func TestSQLiteFieldComparatorExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// The seeded "Go" book is the only one whose editor matches a specific
	// author; here the comparison joins book to author by identity so the
	// predicate references two identifiers, not a bound value.
	rows, err := gooq.Select1(db.Book.Title).
		From(db.Book).
		InnerJoin(db.Author).On(db.Book.AuthorId.EQField(db.Author.Id)).
		Where(db.Author.Id.NEField(db.Book.AuthorId).Not()).
		OrderBy(db.Book.Id.Asc()).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "field comparator join", err)

	// Every seeded book has a matching author, so all three rows satisfy the
	// "author id is not different from the book's author id" predicate.
	equal(t, "matched rows", len(rows), 3)
	equal(t, "first matched book", rows[0].V1, "The Go Programming Language")
}

// TestSQLiteNumericArithFieldExec verifies field-operand arithmetic executes
// against SQLite by deriving a value from two columns of the same row.
func TestSQLiteNumericArithFieldExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// rating + rating doubles the seeded rating; selecting it back proves the
	// arithmetic expression renders both operands as identifiers and computes in
	// the database.
	doubled := db.Review.Rating.AddField(db.Review.Rating)
	rows, err := gooq.Select1(doubled).
		From(db.Review).
		Where(db.Review.Id.EQ(reviewGoFirst)).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "doubled rating", err)

	// The first Go review has a rating of 5, so the doubled value is 10.
	equal(t, "count", len(rows), 1)
	equal(t, "doubled rating", rows[0].V1, int64(10))
}

// TestSQLiteNumericNegExec verifies unary negation executes against SQLite.
func TestSQLiteNumericNegExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	negated := db.Review.Rating.Neg()
	rows, err := gooq.Select1(negated).
		From(db.Review).
		Where(db.Review.Id.EQ(reviewGoFirst)).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "negated rating", err)

	equal(t, "count", len(rows), 1)
	equal(t, "negated rating", rows[0].V1, int64(-5))
}

// TestSQLiteConcatExec verifies string concatenation executes against SQLite,
// mixing bound literals with a column operand. The column renders as an
// identifier while the surrounding literals bind.
func TestSQLiteConcatExec(t *testing.T) {
	ctx, conn := sqliteLibrary(t)

	// "Author: <name>!" wraps the name column between two bound literals.
	greeting := gooq.Concat("Author: ", db.Author.Name, "!")
	rows, err := gooq.Select1(greeting).
		From(db.Author).
		Where(db.Author.Id.EQ(authorDonovan)).
		Using(gooq.SQLite()).
		Fetch(ctx, conn)
	noError(t, "concatenated name", err)

	equal(t, "count", len(rows), 1)
	equal(t, "concatenation", rows[0].V1, "Author: Alan Donovan!")
}
