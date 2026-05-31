package gooq

import "testing"

// TestWindowFunctionRendering asserts the rendered SQL of the ranking, value,
// and windowed-aggregate constructors with PARTITION BY and ORDER BY. The
// identifier quoting is identical for both dialects, so a single rendering per
// case covers them; placeholder differences are exercised by the bind-ordering
// tests elsewhere, since window specifications bind no arguments.
func TestWindowFunctionRendering(t *testing.T) {
	tests := []struct {
		name string
		expr node
		sql  string
	}{
		{
			"row_number partition order",
			RowNumber().Over().PartitionBy(Book.AuthorID).OrderBy(Book.Price.Desc()).End(),
			`ROW_NUMBER() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" DESC)`,
		},
		{
			"rank partition order",
			Rank().Over().PartitionBy(Book.AuthorID).OrderBy(Book.Price.Asc()).End(),
			`RANK() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" ASC)`,
		},
		{
			"dense_rank order only",
			DenseRank().Over().OrderBy(Book.ID.Asc()).End(),
			`DENSE_RANK() OVER (ORDER BY "book"."id" ASC)`,
		},
		{
			"lead partition order",
			Lead(Book.Price).Over().PartitionBy(Book.AuthorID).OrderBy(Book.ID.Asc()).End(),
			`LEAD("book"."price") OVER (PARTITION BY "book"."author_id" ORDER BY "book"."id" ASC)`,
		},
		{
			"lag partition order",
			Lag(Book.Price).Over().PartitionBy(Book.AuthorID).OrderBy(Book.ID.Asc()).End(),
			`LAG("book"."price") OVER (PARTITION BY "book"."author_id" ORDER BY "book"."id" ASC)`,
		},
		{
			"first_value",
			FirstValue(Book.Title).Over().PartitionBy(Book.AuthorID).End(),
			`FIRST_VALUE("book"."title") OVER (PARTITION BY "book"."author_id")`,
		},
		{
			"last_value with frame",
			LastValue(Book.Title).Over().PartitionBy(Book.AuthorID).OrderBy(Book.ID.Asc()).Frame("ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING").End(),
			`LAST_VALUE("book"."title") OVER (PARTITION BY "book"."author_id" ORDER BY "book"."id" ASC ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING)`,
		},
		{
			"sum over",
			SumOver(Book.Price).Over().PartitionBy(Book.AuthorID).End(),
			`SUM("book"."price") OVER (PARTITION BY "book"."author_id")`,
		},
		{
			"avg over",
			AvgOver(Book.Price).Over().PartitionBy(Book.AuthorID).End(),
			`AVG("book"."price") OVER (PARTITION BY "book"."author_id")`,
		},
		{
			"count over",
			CountOver(Book.ID).Over().PartitionBy(Book.AuthorID).End(),
			`COUNT("book"."id") OVER (PARTITION BY "book"."author_id")`,
		},
		{
			"empty window",
			RowNumber().Over().End(),
			`ROW_NUMBER() OVER ()`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sql, args := renderNode(Postgres(), tc.expr)
			if sql != tc.sql {
				t.Errorf("sql = %q, want %q", sql, tc.sql)
			}
			if len(args) != 0 {
				t.Errorf("args = %v, want none", args)
			}
		})
	}
}

// TestWindowFunctionSQLite confirms the identical rendering under the SQLite
// dialect, since the only dialect-specific fragment a window expression could
// carry is identifier quoting, which both dialects spell with double quotes.
func TestWindowFunctionSQLite(t *testing.T) {
	expr := RowNumber().Over().PartitionBy(Book.AuthorID).OrderBy(Book.Price.Desc()).End()
	want := `ROW_NUMBER() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" DESC)`
	sql, args := renderNode(SQLite(), expr)
	if sql != want {
		t.Errorf("sqlite sql = %q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("sqlite args = %v, want none", args)
	}
}

// TestWindowAsProjectionColumn asserts that a window expression composes as a
// projection column in a SELECT, rendered with its alias declared, alongside an
// ordinary column.
func TestWindowAsProjectionColumn(t *testing.T) {
	rn := RowNumber().Over().PartitionBy(Book.AuthorID).OrderBy(Book.Price.Desc()).End().As("rn")
	q := Select2(Book.ID, rn).From(Book)
	sql, args, err := q.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "book"."id", ROW_NUMBER() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" DESC) AS "rn" FROM "book"`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want none", args)
	}
}

// TestWindowOrderByNullsKept confirms that a NULLS ordering qualifier set on a
// window ORDER BY term is preserved in the rendered specification.
func TestWindowOrderByNullsKept(t *testing.T) {
	expr := RowNumber().Over().OrderBy(Book.Price.Desc().NullsLast()).End()
	want := `ROW_NUMBER() OVER (ORDER BY "book"."price" DESC NULLS LAST)`
	sql, _ := renderNode(Postgres(), expr)
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
}
