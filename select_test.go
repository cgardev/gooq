package gooq

import (
	"reflect"
	"testing"
)

func TestSelectGoldenPostgres(t *testing.T) {
	tests := []struct {
		name  string
		query interface {
			SQL() (string, []any, error)
		}
		sql  string
		args []any
	}{
		{
			"select all columns",
			Select3(Book.ID, Book.Title, Book.Price).From(Book),
			`SELECT "book"."id", "book"."title", "book"."price" FROM "book"`,
			nil,
		},
		{
			"where",
			Select1(Book.Title).From(Book).Where(Book.Price.GT(10)),
			`SELECT "book"."title" FROM "book" WHERE "book"."price" > $1`,
			[]any{float64(10)},
		},
		{
			"where and",
			Select1(Book.Title).From(Book).Where(Book.Price.GT(10)).And(Book.Title.Like("Go%")),
			`SELECT "book"."title" FROM "book" WHERE ("book"."price" > $1 AND "book"."title" LIKE $2)`,
			[]any{float64(10), "Go%"},
		},
		{
			"where or",
			Select1(Book.ID).From(Book).Where(Book.ID.EQ(1)).Or(Book.ID.EQ(2)),
			`SELECT "book"."id" FROM "book" WHERE ("book"."id" = $1 OR "book"."id" = $2)`,
			[]any{int64(1), int64(2)},
		},
		{
			"order by and limit offset",
			Select2(Book.Title, Book.Price).From(Book).
				Where(Book.Price.GT(10)).
				OrderBy(Book.Title.Asc(), Book.Price.Desc()).
				Limit(20).Offset(40),
			`SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" > $1 ORDER BY "book"."title" ASC, "book"."price" DESC LIMIT 20 OFFSET 40`,
			[]any{float64(10)},
		},
		{
			"group by having",
			Select1(Book.AuthorID).From(Book).
				GroupBy(Book.AuthorID).
				Having(Book.AuthorID.GT(0)),
			`SELECT "book"."author_id" FROM "book" GROUP BY "book"."author_id" HAVING "book"."author_id" > $1`,
			[]any{int64(0)},
		},
		{
			"inner join on",
			Select2(Book.Title, Author.Name).From(Book).
				Join(Author).On(Book.AuthorID.EQField(Author.ID)),
			`SELECT "book"."title", "author"."name" FROM "book" JOIN "author" ON "book"."author_id" = "author"."id"`,
			nil,
		},
		{
			"left join with alias",
			func() interface {
				SQL() (string, []any, error)
			} {
				a := Author.As("a")
				return Select2(Book.Title, a.Name).From(Book).LeftJoin(a).On(Book.AuthorID.EQField(a.ID))
			}(),
			`SELECT "book"."title", "a"."name" FROM "book" LEFT JOIN "author" "a" ON "book"."author_id" = "a"."id"`,
			nil,
		},
		{
			"projection alias",
			Select1(Book.Title.As("t")).From(Book),
			`SELECT "book"."title" AS "t" FROM "book"`,
			nil,
		},
		{
			"offset only",
			Select1(Book.ID).From(Book).Offset(5),
			`SELECT "book"."id" FROM "book" OFFSET 5`,
			nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sql, args, err := tc.query.SQL()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.sql {
				t.Errorf("sql  = %q\nwant = %q", sql, tc.sql)
			}
			if !reflect.DeepEqual(args, tc.args) {
				t.Errorf("args = %#v, want %#v", args, tc.args)
			}
		})
	}
}

// TestSelectClauseSkipping confirms optional clauses can be omitted thanks to
// the embedded step interfaces: FROM straight to ORDER BY, FROM straight to
// LIMIT, and a bare projection are all legal.
func TestSelectClauseSkipping(t *testing.T) {
	q := Select1(Book.ID).From(Book).OrderBy(Book.ID.Asc()).Limit(1)
	sql, _, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT "book"."id" FROM "book" ORDER BY "book"."id" ASC LIMIT 1`; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
}

func TestSelectUsingDialectOverride(t *testing.T) {
	q := Select1(Book.ID).From(Book).Where(Book.ID.EQ(1))

	pgSQL, _, _ := q.SQL()
	if want := `SELECT "book"."id" FROM "book" WHERE "book"."id" = $1`; pgSQL != want {
		t.Errorf("postgres = %q, want %q", pgSQL, want)
	}

	sqliteSQL, _, _ := q.SQLFor(SQLite())
	if want := `SELECT "book"."id" FROM "book" WHERE "book"."id" = ?`; sqliteSQL != want {
		t.Errorf("sqlite = %q, want %q", sqliteSQL, want)
	}
}

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
