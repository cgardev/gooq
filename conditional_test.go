package gooq

import "testing"

// TestCoalesce verifies COALESCE with a field operand and a bound default, and
// with a second field operand that renders as an identifier.
func TestCoalesce(t *testing.T) {
	// Field then a bound literal default.
	c := Coalesce(Book.Title, "untitled")
	assertSQL(t, Postgres(), c, `COALESCE("book"."title", $1)`, "untitled")
	assertSQL(t, SQLite(), c, `COALESCE("book"."title", ?)`, "untitled")

	// Field then a field operand: nothing binds.
	c2 := Coalesce(Book.Title, Author.Name)
	assertSQL(t, Postgres(), c2, `COALESCE("book"."title", "author"."name")`)
	assertSQL(t, SQLite(), c2, `COALESCE("book"."title", "author"."name")`)
}

// TestNullIf verifies NULLIF binds its comparison value in both dialects.
func TestNullIf(t *testing.T) {
	n := NullIf(Book.Title, "")
	assertSQL(t, Postgres(), n, `NULLIF("book"."title", $1)`, "")
	assertSQL(t, SQLite(), n, `NULLIF("book"."title", ?)`, "")
}

// TestGreatestLeast verifies GREATEST and LEAST mix bound values with field
// operands the same way COALESCE does.
func TestGreatestLeast(t *testing.T) {
	g := Greatest(Book.Price, 10.0)
	assertSQL(t, Postgres(), g, `GREATEST("book"."price", $1)`, 10.0)
	assertSQL(t, SQLite(), g, `GREATEST("book"."price", ?)`, 10.0)

	l := Least(Book.Price, Book.Price.Add(1))
	assertSQL(t, Postgres(), l, `LEAST("book"."price", ("book"."price" + $1))`, float64(1))
	assertSQL(t, SQLite(), l, `LEAST("book"."price", ("book"."price" + ?))`, float64(1))
}

// TestCase verifies the searched CASE builder, including a bound THEN value, a
// field-operand THEN branch, and a bound ELSE.
func TestCase(t *testing.T) {
	// Single WHEN with a bound THEN and a bound ELSE.
	expr := Case[string]().
		When(Book.Price.GT(20), "expensive").
		Else("cheap").
		End()
	assertSQL(t, Postgres(), expr,
		`CASE WHEN "book"."price" > $1 THEN $2 ELSE $3 END`, float64(20), "expensive", "cheap")
	assertSQL(t, SQLite(), expr,
		`CASE WHEN "book"."price" > ? THEN ? ELSE ? END`, float64(20), "expensive", "cheap")

	// Two WHEN branches, the second using a field operand for the THEN result.
	expr2 := Case[string]().
		When(Book.Price.GE(40), "premium").
		WhenField(Book.Price.GE(20), Book.Title).
		Else("budget").
		End()
	assertSQL(t, Postgres(), expr2,
		`CASE WHEN "book"."price" >= $1 THEN $2 WHEN "book"."price" >= $3 THEN "book"."title" ELSE $4 END`,
		float64(40), "premium", float64(20), "budget")
	assertSQL(t, SQLite(), expr2,
		`CASE WHEN "book"."price" >= ? THEN ? WHEN "book"."price" >= ? THEN "book"."title" ELSE ? END`,
		float64(40), "premium", float64(20), "budget")

	// A CASE with no ELSE omits the ELSE fragment entirely.
	expr3 := Case[int64]().When(Book.ID.EQ(1), 100).End()
	assertSQL(t, Postgres(), expr3, `CASE WHEN "book"."id" = $1 THEN $2 END`, int64(1), int64(100))
	assertSQL(t, SQLite(), expr3, `CASE WHEN "book"."id" = ? THEN ? END`, int64(1), int64(100))
}

// TestCast verifies CAST renders "CAST(expr AS sqltype)" with the verbatim type
// and binds nothing.
func TestCast(t *testing.T) {
	c := Cast[string](Book.ID, "TEXT")
	assertSQL(t, Postgres(), c, `CAST("book"."id" AS TEXT)`)
	assertSQL(t, SQLite(), c, `CAST("book"."id" AS TEXT)`)
}
