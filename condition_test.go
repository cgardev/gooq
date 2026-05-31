package gooq

import (
	"reflect"
	"testing"
)

func TestConditionCombinators(t *testing.T) {
	tests := []struct {
		name string
		cond Condition
		sql  string
		args []any
	}{
		{
			"and",
			Book.Title.EQ("Go").And(Book.Price.GT(10)),
			`("book"."title" = $1 AND "book"."price" > $2)`,
			[]any{"Go", float64(10)},
		},
		{
			"or",
			Book.ID.EQ(1).Or(Book.ID.EQ(2)),
			`("book"."id" = $1 OR "book"."id" = $2)`,
			[]any{int64(1), int64(2)},
		},
		{
			"not",
			Book.Title.EQ("Go").Not(),
			`NOT ("book"."title" = $1)`,
			[]any{"Go"},
		},
		{
			"nested and/or",
			Book.Title.EQ("Go").And(Book.Price.GT(10).Or(Book.Price.LT(2))),
			`("book"."title" = $1 AND ("book"."price" > $2 OR "book"."price" < $3))`,
			[]any{"Go", float64(10), float64(2)},
		},
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

// TestConditionIsField verifies that a Condition is usable as a Field[bool],
// the jOOQ parity property: a stored condition can be combined and reused.
func TestConditionIsField(t *testing.T) {
	var f Field[bool] = Book.Price.GT(10)
	if f.Name() != "" {
		t.Errorf("anonymous condition Name = %q, want empty", f.Name())
	}
	c := Book.Price.GT(10)
	combined := c.And(c) // reuse the same condition value twice
	sql, _ := renderNode(Postgres(), combined)
	if want := `("book"."price" > $1 AND "book"."price" > $2)`; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
}

// TestBooleanCombinators verifies the free-function And, Or, and Not, including
// the variadic flattening that yields one flat group rather than nested pairs.
func TestBooleanCombinators(t *testing.T) {
	a := Book.ID.EQ(1)
	b := Book.Title.EQ("x")
	c := Book.AuthorID.EQ(2)

	tests := []struct {
		name string
		cond Condition
		want string
	}{
		{
			name: "and flattened",
			cond: And(a, b, c),
			want: `("book"."id" = $1 AND "book"."title" = $2 AND "book"."author_id" = $3)`,
		},
		{
			name: "or flattened",
			cond: Or(a, b, c),
			want: `("book"."id" = $1 OR "book"."title" = $2 OR "book"."author_id" = $3)`,
		},
		{
			name: "and single returns as-is",
			cond: And(a),
			want: `"book"."id" = $1`,
		},
		{
			name: "or single returns as-is",
			cond: Or(b),
			want: `"book"."title" = $1`,
		},
		{
			name: "and empty is true",
			cond: And(),
			want: `1 = 1`,
		},
		{
			name: "or empty is false",
			cond: Or(),
			want: `1 = 0`,
		},
		{
			name: "not",
			cond: Not(a),
			want: `NOT ("book"."id" = $1)`,
		},
		{
			name: "true constant",
			cond: True(),
			want: `1 = 1`,
		},
		{
			name: "false constant",
			cond: False(),
			want: `1 = 0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := renderNode(Postgres(), tt.cond)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestConstantConditionDialects verifies that True and False render identically
// across both dialects, since they emit no placeholders.
func TestConstantConditionDialects(t *testing.T) {
	for _, d := range []Dialect{Postgres(), SQLite()} {
		t.Run(d.Name(), func(t *testing.T) {
			if got, _ := renderNode(d, True()); got != "1 = 1" {
				t.Errorf("True(): got %q, want %q", got, "1 = 1")
			}
			if got, _ := renderNode(d, False()); got != "1 = 0" {
				t.Errorf("False(): got %q, want %q", got, "1 = 0")
			}
		})
	}
}
