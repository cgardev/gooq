package gooq

import (
	"reflect"
	"testing"
)

// phase1_test.go contains golden-SQL tests for expression operands: field-to-field
// comparators, field-operand arithmetic, unary negation, and string concatenation.
// They prove that field operands render as identifiers (no bind) while value
// operands still bind, across both supported dialects.

// assertSQL renders a node against a dialect and asserts both the SQL text and
// the bound argument values. A nil wantArgs (passing none) asserts that no
// arguments were bound.
func assertSQL(t *testing.T, d Dialect, n node, wantSQL string, wantArgs ...any) {
	t.Helper()
	sql, args := renderNode(d, n)
	if sql != wantSQL {
		t.Errorf("[%s] sql = %q, want %q", d.Name(), sql, wantSQL)
	}
	if len(args) != len(wantArgs) {
		t.Fatalf("[%s] got %d args %#v, want %d %#v", d.Name(), len(args), args, len(wantArgs), wantArgs)
	}
	for i := range args {
		if !reflect.DeepEqual(args[i], wantArgs[i]) {
			t.Errorf("[%s] arg %d = %#v, want %#v", d.Name(), i, args[i], wantArgs[i])
		}
	}
}

// TestFieldComparatorsRenderIdentifiers verifies that the *Field comparators
// render the right operand as an identifier with no bound argument, identically
// in both dialects.
func TestFieldComparatorsRenderIdentifiers(t *testing.T) {
	cases := []struct {
		name string
		cond Condition
		want string
	}{
		{"EQField", Book.AuthorID.EQField(Author.ID), `"book"."author_id" = "author"."id"`},
		{"NEField", Book.AuthorID.NEField(Author.ID), `"book"."author_id" <> "author"."id"`},
		{"GTField", Book.AuthorID.GTField(Author.ID), `"book"."author_id" > "author"."id"`},
		{"LTField", Book.AuthorID.LTField(Author.ID), `"book"."author_id" < "author"."id"`},
		{"GEField", Book.AuthorID.GEField(Author.ID), `"book"."author_id" >= "author"."id"`},
		{"LEField", Book.AuthorID.LEField(Author.ID), `"book"."author_id" <= "author"."id"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSQL(t, Postgres(), tc.cond, tc.want)
			assertSQL(t, SQLite(), tc.cond, tc.want)
		})
	}
}

// TestValueComparatorsStillBind confirms the value-taking comparators continue
// to bind their argument, distinguishing them from the *Field variants.
func TestValueComparatorsStillBind(t *testing.T) {
	assertSQL(t, Postgres(), Book.ID.EQ(5), `"book"."id" = $1`, int64(5))
	assertSQL(t, SQLite(), Book.ID.EQ(5), `"book"."id" = ?`, int64(5))
}

// TestNumericArithFieldRendersIdentifiers verifies field-operand arithmetic
// renders the right operand as an identifier with no bound argument.
func TestNumericArithFieldRendersIdentifiers(t *testing.T) {
	// A distinct numeric column proves the right operand renders as its own
	// identifier rather than binding a value.
	other := NewNumericField[float64](NewTable("sale"), "discount")
	cases := []struct {
		name  string
		field Field[float64]
		want  string
	}{
		{"AddField", Book.Price.AddField(other), `("book"."price" + "sale"."discount")`},
		{"SubField", Book.Price.SubField(other), `("book"."price" - "sale"."discount")`},
		{"MulField", Book.Price.MulField(other), `("book"."price" * "sale"."discount")`},
		{"DivField", Book.Price.DivField(other), `("book"."price" / "sale"."discount")`},
		{"ModField", Book.Price.ModField(other), `("book"."price" % "sale"."discount")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSQL(t, Postgres(), tc.field, tc.want)
			assertSQL(t, SQLite(), tc.field, tc.want)
		})
	}
}

// TestNumericArithValueStillBinds confirms the value-taking arithmetic operators
// continue to bind their argument.
func TestNumericArithValueStillBinds(t *testing.T) {
	cases := []struct {
		name  string
		field Field[float64]
		pg    string
		lite  string
	}{
		{"Add", Book.Price.Add(2), `("book"."price" + $1)`, `("book"."price" + ?)`},
		{"Sub", Book.Price.Sub(2), `("book"."price" - $1)`, `("book"."price" - ?)`},
		{"Mul", Book.Price.Mul(2), `("book"."price" * $1)`, `("book"."price" * ?)`},
		{"Div", Book.Price.Div(2), `("book"."price" / $1)`, `("book"."price" / ?)`},
		{"ModVal", Book.Price.ModVal(2), `("book"."price" % $1)`, `("book"."price" % ?)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSQL(t, Postgres(), tc.field, tc.pg, float64(2))
			assertSQL(t, SQLite(), tc.field, tc.lite, float64(2))
		})
	}
}

// TestNumericNeg verifies unary negation renders "-(expr)" with no bound
// argument in both dialects.
func TestNumericNeg(t *testing.T) {
	assertSQL(t, Postgres(), Book.Price.Neg(), `-("book"."price")`)
	assertSQL(t, SQLite(), Book.Price.Neg(), `-("book"."price")`)
}

// TestStringConcatMethod verifies StringField.Concat mixes bound literals and
// field operands, rendering "||" chains with field operands as identifiers.
func TestStringConcatMethod(t *testing.T) {
	// field || literal || field: the literal binds, the fields render as columns.
	c1 := Author.Name.Concat(" ", Book.Title)
	assertSQL(t, Postgres(), c1, `("author"."name" || $1 || "book"."title")`, " ")
	assertSQL(t, SQLite(), c1, `("author"."name" || ? || "book"."title")`, " ")

	// field || field: both render as columns, nothing binds.
	c2 := Author.Name.Concat(Book.Title)
	assertSQL(t, Postgres(), c2, `("author"."name" || "book"."title")`)
	assertSQL(t, SQLite(), c2, `("author"."name" || "book"."title")`)
}

// TestStringConcatFunction verifies the free Concat function mixes value and
// field operands the same way as the method.
func TestStringConcatFunction(t *testing.T) {
	c := Concat("Mr. ", Author.Name, "!")
	assertSQL(t, Postgres(), c, `($1 || "author"."name" || $2)`, "Mr. ", "!")
	assertSQL(t, SQLite(), c, `(? || "author"."name" || ?)`, "Mr. ", "!")
}

// TestConcatAcceptsFieldString verifies Concat accepts a plain Field[string]
// (not only StringField) as a field operand.
func TestConcatAcceptsFieldString(t *testing.T) {
	var plain Field[string] = NewField[string](NewTable("t"), "code")
	c := Concat("X-", plain)
	assertSQL(t, Postgres(), c, `($1 || "t"."code")`, "X-")
	assertSQL(t, SQLite(), c, `(? || "t"."code")`, "X-")
}
