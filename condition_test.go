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
