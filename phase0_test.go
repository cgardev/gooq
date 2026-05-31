package gooq

import "testing"

// TestDistinct verifies that SELECT DISTINCT renders in both dialects, with the
// placeholder spelling being the only dialect-dependent difference.
func TestDistinct(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{
			name:    "postgres",
			dialect: Postgres(),
			want:    `SELECT DISTINCT "book"."title" FROM "book" WHERE "book"."id" > $1`,
		},
		{
			name:    "sqlite",
			dialect: SQLite(),
			want:    `SELECT DISTINCT "book"."title" FROM "book" WHERE "book"."id" > ?`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := Select1(Book.Title).Distinct().From(Book).Where(Book.ID.GT(1))
			got, _, err := stmt.SQLFor(tt.dialect)
			if err != nil {
				t.Fatalf("SQLFor error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestOrderByNulls verifies the NULLS FIRST and NULLS LAST qualifiers. Their
// rendering is dialect-independent, so a single dialect suffices for assertion.
func TestOrderByNulls(t *testing.T) {
	tests := []struct {
		name string
		stmt interface{ SQL() (string, []any, error) }
		want string
	}{
		{
			name: "asc nulls first",
			stmt: Select1(Book.ID).From(Book).OrderBy(Book.ID.Asc().NullsFirst()),
			want: `SELECT "book"."id" FROM "book" ORDER BY "book"."id" ASC NULLS FIRST`,
		},
		{
			name: "desc nulls last",
			stmt: Select1(Book.ID).From(Book).OrderBy(Book.ID.Desc().NullsLast()),
			want: `SELECT "book"."id" FROM "book" ORDER BY "book"."id" DESC NULLS LAST`,
		},
		{
			name: "mixed terms",
			stmt: Select1(Book.ID).From(Book).OrderBy(Book.Title.Asc().NullsFirst(), Book.ID.Desc()),
			want: `SELECT "book"."id" FROM "book" ORDER BY "book"."title" ASC NULLS FIRST, "book"."id" DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := tt.stmt.SQL()
			if err != nil {
				t.Fatalf("SQL error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
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

// TestRaw verifies the verbatim Raw field and RawCondition escape hatches, which
// bind no arguments and render identically across dialects.
func TestRaw(t *testing.T) {
	for _, d := range []Dialect{Postgres(), SQLite()} {
		t.Run(d.Name(), func(t *testing.T) {
			f := Raw[int64]("COUNT(*)")
			if got, args := renderNode(d, f); got != "COUNT(*)" || len(args) != 0 {
				t.Errorf("Raw: got %q args=%v, want %q with no args", got, args, "COUNT(*)")
			}

			c := RawCondition("1 = 1")
			if got, args := renderNode(d, c); got != "1 = 1" || len(args) != 0 {
				t.Errorf("RawCondition: got %q args=%v, want %q with no args", got, args, "1 = 1")
			}
		})
	}
}

// TestRawValue verifies that RawValue interleaves dialect placeholders for each
// '?' marker and binds the supplied arguments in order.
func TestRawValue(t *testing.T) {
	tests := []struct {
		name     string
		dialect  Dialect
		want     string
		wantArgs []any
	}{
		{
			name:     "postgres two markers",
			dialect:  Postgres(),
			want:     `id + $1 > $2`,
			wantArgs: []any{int64(5), int64(10)},
		},
		{
			name:     "sqlite two markers",
			dialect:  SQLite(),
			want:     `id + ? > ?`,
			wantArgs: []any{int64(5), int64(10)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := RawValue[int64]("id + ? > ?", int64(5), int64(10))
			got, args := renderNode(tt.dialect, f)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("got %d args, want %d", len(args), len(tt.wantArgs))
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Errorf("arg %d: got %v, want %v", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

// TestRawValueNoMarkers verifies that RawValue with no markers and no arguments
// behaves like a plain literal.
func TestRawValueNoMarkers(t *testing.T) {
	f := RawValue[int64]("42")
	if got, args := renderNode(Postgres(), f); got != "42" || len(args) != 0 {
		t.Errorf("got %q args=%v, want %q with no args", got, args, "42")
	}
}
