package gooq

import "testing"

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
