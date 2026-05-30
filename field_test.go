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
