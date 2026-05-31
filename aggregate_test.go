package gooq

import "testing"

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
