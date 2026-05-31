package gooq

import (
	"reflect"
	"testing"
)

// renderNode renders an arbitrary node against a dialect and returns the SQL
// and the bound arguments. It is the workhorse for the golden-SQL tests.
func renderNode(d Dialect, n node) (string, []any) {
	b := newBuilder(d)
	n.render(b)
	return b.sql.String(), b.args
}

func TestFieldComparisonOperators(t *testing.T) {
	tests := []struct {
		name string
		cond Condition
		sql  string
		args []any
	}{
		{"eq", Book.ID.EQ(1), `"book"."id" = $1`, []any{int64(1)}},
		{"ne", Book.ID.NE(1), `"book"."id" <> $1`, []any{int64(1)}},
		{"gt", Book.Price.GT(10), `"book"."price" > $1`, []any{float64(10)}},
		{"lt", Book.Price.LT(10), `"book"."price" < $1`, []any{float64(10)}},
		{"ge", Book.Price.GE(10), `"book"."price" >= $1`, []any{float64(10)}},
		{"le", Book.Price.LE(10), `"book"."price" <= $1`, []any{float64(10)}},
		{"like", Book.Title.Like("Go%"), `"book"."title" LIKE $1`, []any{"Go%"}},
		{"notlike", Book.Title.NotLike("Go%"), `"book"."title" NOT LIKE $1`, []any{"Go%"}},
		{"ilike", Book.Title.ILike("go%"), `"book"."title" ILIKE $1`, []any{"go%"}},
		{"isnull", Book.AuthorID.IsNull(), `"book"."author_id" IS NULL`, nil},
		{"isnotnull", Book.AuthorID.IsNotNull(), `"book"."author_id" IS NOT NULL`, nil},
		{"between", Book.Price.Between(5, 9), `"book"."price" BETWEEN $1 AND $2`, []any{float64(5), float64(9)}},
		{"in", Book.ID.In(1, 2, 3), `"book"."id" IN ($1, $2, $3)`, []any{int64(1), int64(2), int64(3)}},
		{"notin", Book.ID.NotIn(1, 2), `"book"."id" NOT IN ($1, $2)`, []any{int64(1), int64(2)}},
		{"in empty", Book.ID.In(), `1 = 0`, nil},
		{"notin empty", Book.ID.NotIn(), `1 = 1`, nil},
		{"eqfield", Book.AuthorID.EQField(Author.ID), `"book"."author_id" = "author"."id"`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sql, args := renderNode(Postgres(), tc.cond)
			if sql != tc.sql {
				t.Errorf("sql = %q, want %q", sql, tc.sql)
			}
			if !reflect.DeepEqual(args, tc.args) {
				t.Errorf("args = %#v, want %#v", args, tc.args)
			}
		})
	}
}

func TestFieldAliasOnlyDeclaredInProjection(t *testing.T) {
	aliased := Book.Title.As("t")
	if aliased.Name() != "t" {
		t.Errorf("Name = %q, want t", aliased.Name())
	}

	// Outside a projection (declareAlias false) only the underlying column is
	// rendered, keeping the SQL valid when the aliased field is reused.
	if sql, _ := renderNode(Postgres(), aliased); sql != `"book"."title"` {
		t.Errorf("reference render = %q, want bare column", sql)
	}

	// Inside a projection the alias is declared.
	b := newBuilder(Postgres())
	b.declareAlias = true
	aliased.render(b)
	if got, want := b.sql.String(), `"book"."title" AS "t"`; got != want {
		t.Errorf("declaration render = %q, want %q", got, want)
	}
}

func TestNumericFieldArithmetic(t *testing.T) {
	sql, args := renderNode(Postgres(), Book.Price.Add(5).EQField(Book.Price))
	if want := `("book"."price" + $1) = "book"."price"`; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{float64(5)}) {
		t.Errorf("args = %#v", args)
	}
}

func TestFieldSetProducesAssignment(t *testing.T) {
	a := Book.Title.Set("Go")
	sql, args := renderNode(Postgres(), a)
	if want := `"title" = $1`; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{"Go"}) {
		t.Errorf("args = %#v", args)
	}
}

func TestOrderTerms(t *testing.T) {
	if sql, _ := renderNode(Postgres(), Book.Title.Asc()); sql != `"book"."title" ASC` {
		t.Errorf("asc = %q", sql)
	}
	if sql, _ := renderNode(Postgres(), Book.Price.Desc()); sql != `"book"."price" DESC` {
		t.Errorf("desc = %q", sql)
	}
}

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

// TestNotBetween verifies the NOT BETWEEN predicate in both dialects.
func TestNotBetween(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{name: "postgres", dialect: Postgres(), want: `"book"."id" NOT BETWEEN $1 AND $2`},
		{name: "sqlite", dialect: SQLite(), want: `"book"."id" NOT BETWEEN ? AND ?`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := Book.ID.NotBetween(1, 10)
			got, args := renderNode(tt.dialect, cond)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if len(args) != 2 {
				t.Errorf("got %d args, want 2", len(args))
			}
		})
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
