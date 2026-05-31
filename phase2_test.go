package gooq

import "testing"

// phase2_test.go contains golden-SQL tests for the function and expression
// catalog: a generic function call, the wildcard star, aggregates, conditional
// and null-handling expressions, the CASE builder, CAST, and the small string
// and math catalog. Constructs that bind arguments are asserted against both the
// PostgreSQL ($1) and SQLite (?) dialects; constructs that bind nothing render
// identically across dialects, which is verified by asserting both.

// TestFunctionGeneric verifies the generic Function constructor renders a call
// with its field arguments as identifiers and binds nothing.
func TestFunctionGeneric(t *testing.T) {
	f := Function[int64]("LENGTH", Book.Title)
	assertSQL(t, Postgres(), f, `LENGTH("book"."title")`)
	assertSQL(t, SQLite(), f, `LENGTH("book"."title")`)

	// A multi-argument call keeps the comma-separated identifier list.
	g := Function[float64]("POWER", Book.Price, Book.ID)
	assertSQL(t, Postgres(), g, `POWER("book"."price", "book"."id")`)
	assertSQL(t, SQLite(), g, `POWER("book"."price", "book"."id")`)
}

// TestStar verifies the wildcard renders as an unqualified "*".
func TestStar(t *testing.T) {
	assertSQL(t, Postgres(), Star(), `*`)
	assertSQL(t, SQLite(), Star(), `*`)
}

// TestAggregates verifies the aggregate constructors, including COUNT(*),
// COUNT(DISTINCT ...), SUM, AVG, MIN, and MAX. None bind arguments, so each
// renders identically across dialects.
func TestAggregates(t *testing.T) {
	cases := []struct {
		name  string
		field node
		want  string
	}{
		{"Count", Count(Book.ID), `COUNT("book"."id")`},
		{"CountStar", CountStar(), `COUNT(*)`},
		{"CountDistinct", CountDistinct(Book.AuthorID), `COUNT(DISTINCT "book"."author_id")`},
		{"Sum", Sum(Book.Price), `SUM("book"."price")`},
		{"Avg", Avg(Book.Price), `AVG("book"."price")`},
		{"Min", Min(Book.Price), `MIN("book"."price")`},
		{"Max", Max(Book.Price), `MAX("book"."price")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSQL(t, Postgres(), tc.field, tc.want)
			assertSQL(t, SQLite(), tc.field, tc.want)
		})
	}
}

// TestAggregateInProjection verifies an aggregate composes into a full SELECT
// projection alongside GROUP BY.
func TestAggregateInProjection(t *testing.T) {
	stmt := Select2(Book.AuthorID, Count(Book.ID)).
		From(Book).
		GroupBy(Book.AuthorID)
	pg, _, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("SQLFor error: %v", err)
	}
	want := `SELECT "book"."author_id", COUNT("book"."id") FROM "book" GROUP BY "book"."author_id"`
	if pg != want {
		t.Errorf("postgres: got %q, want %q", pg, want)
	}
}

// TestAggregateInHaving verifies that an aggregate yields a Field[int64] whose
// comparison methods build a Condition accepted by the HAVING step.
func TestAggregateInHaving(t *testing.T) {
	stmt := Select1(Book.AuthorID).
		From(Book).
		GroupBy(Book.AuthorID).
		Having(Count(Book.ID).GT(1))
	pg, args, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("SQLFor error: %v", err)
	}
	want := `SELECT "book"."author_id" FROM "book" GROUP BY "book"."author_id" HAVING COUNT("book"."id") > $1`
	if pg != want {
		t.Errorf("postgres: got %q, want %q", pg, want)
	}
	if len(args) != 1 || args[0] != int64(1) {
		t.Errorf("postgres args = %#v, want [int64(1)]", args)
	}

	lite, _, err := stmt.SQLFor(SQLite())
	if err != nil {
		t.Fatalf("SQLFor error: %v", err)
	}
	wantLite := `SELECT "book"."author_id" FROM "book" GROUP BY "book"."author_id" HAVING COUNT("book"."id") > ?`
	if lite != wantLite {
		t.Errorf("sqlite: got %q, want %q", lite, wantLite)
	}
}

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

// TestStringMathCatalog verifies the string and math catalog functions, none of
// which bind arguments, so each renders identically across dialects.
func TestStringMathCatalog(t *testing.T) {
	cases := []struct {
		name  string
		field node
		want  string
	}{
		{"Upper", Upper(Book.Title), `UPPER("book"."title")`},
		{"Lower", Lower(Book.Title), `LOWER("book"."title")`},
		{"Length", Length(Book.Title), `LENGTH("book"."title")`},
		{"Trim", Trim(Book.Title), `TRIM("book"."title")`},
		{"Abs", Abs(Book.Price), `ABS("book"."price")`},
		{"Round", Round(Book.Price), `ROUND("book"."price")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSQL(t, Postgres(), tc.field, tc.want)
			assertSQL(t, SQLite(), tc.field, tc.want)
		})
	}
}

// TestNestedFunction verifies the catalog composes: an aggregate over a function
// over a column renders the nested calls correctly.
func TestNestedFunction(t *testing.T) {
	expr := Max(Length(Book.Title))
	assertSQL(t, Postgres(), expr, `MAX(LENGTH("book"."title"))`)
	assertSQL(t, SQLite(), expr, `MAX(LENGTH("book"."title"))`)
}
