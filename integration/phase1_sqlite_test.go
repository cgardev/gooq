package integration

import (
	"testing"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// TestPhase1FieldComparatorExec verifies the field-to-field comparators execute
// against SQLite, comparing two columns of the same row rather than a column
// against a bound value.
func TestPhase1FieldComparatorExec(t *testing.T) {
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

// TestPhase1NumericArithFieldExec verifies field-operand arithmetic executes
// against SQLite by deriving a value from two columns of the same row.
func TestPhase1NumericArithFieldExec(t *testing.T) {
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

// TestPhase1NumericNegExec verifies unary negation executes against SQLite.
func TestPhase1NumericNegExec(t *testing.T) {
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

// TestPhase1ConcatExec verifies string concatenation executes against SQLite,
// mixing bound literals with a column operand. The column renders as an
// identifier while the surrounding literals bind.
func TestPhase1ConcatExec(t *testing.T) {
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
