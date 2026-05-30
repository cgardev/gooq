package gooq

import (
	"reflect"
	"testing"
)

// TestCrossDialectRendering renders one and the same query AST against every
// dialect and asserts the dialect-specific differences. This is the core
// promise of the library: a single tree, translated at render time.
func TestCrossDialectRendering(t *testing.T) {
	query := Select2(Book.Title, Book.Price).From(Book).
		Where(Book.Price.GT(10)).
		OrderBy(Book.Title.Asc()).
		Limit(20).Offset(40)

	tests := []struct {
		dialect Dialect
		want    string
	}{
		{
			Postgres(),
			`SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" > $1 ORDER BY "book"."title" ASC LIMIT 20 OFFSET 40`,
		},
		{
			SQLite(),
			`SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" > ? ORDER BY "book"."title" ASC LIMIT 20 OFFSET 40`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.dialect.Name(), func(t *testing.T) {
			sql, args, err := query.SQLFor(tc.dialect)
			if err != nil {
				t.Fatal(err)
			}
			if sql != tc.want {
				t.Errorf("sql  = %q\nwant = %q", sql, tc.want)
			}
			if !reflect.DeepEqual(args, []any{float64(10)}) {
				t.Errorf("args = %#v", args)
			}
		})
	}
}

func TestOffsetOnlyPerDialect(t *testing.T) {
	query := Select1(Book.ID).From(Book).Offset(5)
	tests := []struct {
		dialect Dialect
		want    string
	}{
		{Postgres(), `SELECT "book"."id" FROM "book" OFFSET 5`},
		{SQLite(), `SELECT "book"."id" FROM "book" LIMIT -1 OFFSET 5`},
	}
	for _, tc := range tests {
		t.Run(tc.dialect.Name(), func(t *testing.T) {
			sql, _, err := query.SQLFor(tc.dialect)
			if err != nil {
				t.Fatal(err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

func TestDialectBoolLiteral(t *testing.T) {
	cases := []struct {
		d    Dialect
		t, f string
	}{
		{Postgres(), "TRUE", "FALSE"},
		{SQLite(), "1", "0"},
	}
	for _, c := range cases {
		if got := c.d.boolLiteral(true); got != c.t {
			t.Errorf("%s boolLiteral(true) = %q, want %q", c.d.Name(), got, c.t)
		}
		if got := c.d.boolLiteral(false); got != c.f {
			t.Errorf("%s boolLiteral(false) = %q, want %q", c.d.Name(), got, c.f)
		}
	}
}

func TestSQLiteUpsert(t *testing.T) {
	q := InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go").
		OnConflict(Book.ID).DoUpdateSet(SetToExcluded(Book.Title))
	sql, _, err := q.SQLFor(SQLite())
	if err != nil {
		t.Fatal(err)
	}
	want := `INSERT INTO "book" ("id", "title") VALUES (?, ?) ON CONFLICT ("id") DO UPDATE SET "title" = excluded."title"`
	if sql != want {
		t.Errorf("sql  = %q\nwant = %q", sql, want)
	}
}

func TestILikePerDialect(t *testing.T) {
	q := Select1(Book.Title).From(Book).Where(Book.Title.ILike("go%"))
	pg, _, _ := q.SQLFor(Postgres())
	if want := `SELECT "book"."title" FROM "book" WHERE "book"."title" ILIKE $1`; pg != want {
		t.Errorf("pg = %q, want %q", pg, want)
	}
	lite, _, _ := q.SQLFor(SQLite())
	if want := `SELECT "book"."title" FROM "book" WHERE "book"."title" LIKE ?`; lite != want {
		t.Errorf("sqlite = %q, want %q", lite, want)
	}
}
