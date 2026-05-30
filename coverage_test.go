package gooq

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
)

// TestErrEmptyInsert confirms that an INSERT with neither columns, values, nor a
// DEFAULT VALUES marker surfaces ErrEmptyInsert, while DefaultValues renders.
func TestErrEmptyInsert(t *testing.T) {
	t.Run("default values renders", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).DefaultValues(),
			`INSERT INTO "book" DEFAULT VALUES`,
			nil,
		)
	})

	t.Run("empty insert errors", func(t *testing.T) {
		_, _, err := InsertInto(Book).(*insertBuilder).SQL()
		if !errors.Is(err, ErrEmptyInsert) {
			t.Fatalf("err = %v, want ErrEmptyInsert", err)
		}
	})
}

// TestErrColumnValueMismatch confirms that a row whose value count differs from
// the column count surfaces ErrColumnValueMismatch.
func TestErrColumnValueMismatch(t *testing.T) {
	q := InsertInto(Book).Columns(Book.ID, Book.Title).
		Values(int64(1), "Go").
		Values(int64(2))
	if _, _, err := q.SQL(); !errors.Is(err, ErrColumnValueMismatch) {
		t.Fatalf("err = %v, want ErrColumnValueMismatch", err)
	}
}

// TestRightJoinGolden renders a RIGHT JOIN with its ON condition.
func TestRightJoinGolden(t *testing.T) {
	checkSQL(t,
		Select2(Book.Title, Author.Name).From(Book).
			RightJoin(Author).On(Book.AuthorID.EQField(Author.ID)),
		`SELECT "book"."title", "author"."name" FROM "book" RIGHT JOIN "author" ON "book"."author_id" = "author"."id"`,
		nil,
	)
}

// TestNotOverCompoundPredicate confirms that negating a compound AND predicate
// keeps the inner parentheses, yielding a doubly-parenthesized NOT.
func TestNotOverCompoundPredicate(t *testing.T) {
	cond := Book.Title.EQ("Go").And(Book.Price.GT(10)).Not()
	sql, args := renderNode(Postgres(), cond)
	if want := `NOT (("book"."title" = $1 AND "book"."price" > $2))`; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{"Go", float64(10)}) {
		t.Errorf("args = %#v", args)
	}
}

// TestAliasedFieldReusedInWhere confirms that an alias is declared only in the
// projection. The same aliased field reused in WHERE renders as the bare column,
// which keeps the SQL valid.
func TestAliasedFieldReusedInWhere(t *testing.T) {
	titleAlias := Book.Title.As("t")
	q := Select1(titleAlias).From(Book).Where(titleAlias.EQ("Go"))
	sql, args, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "book"."title" AS "t" FROM "book" WHERE "book"."title" = $1`
	if sql != want {
		t.Errorf("sql  = %q\nwant = %q", sql, want)
	}
	// AS must appear exactly once, in the projection only.
	if got := countSubstr(sql, " AS "); got != 1 {
		t.Errorf("AS appeared %d times, want 1 (projection only)", got)
	}
	if !reflect.DeepEqual(args, []any{"Go"}) {
		t.Errorf("args = %#v", args)
	}
}

// TestUpdateWithoutWhere renders an UPDATE that omits the WHERE clause.
func TestUpdateWithoutWhere(t *testing.T) {
	checkSQL(t,
		Update(Book).Set(Book.Price.Set(10)),
		`UPDATE "book" SET "price" = $1`,
		[]any{float64(10)},
	)
}

// TestDeleteWithoutWhere renders a DELETE that omits the WHERE clause.
func TestDeleteWithoutWhere(t *testing.T) {
	checkSQL(t,
		DeleteFrom(Book),
		`DELETE FROM "book"`,
		nil,
	)
}

// TestHighAritySelectGolden renders a Select8 to confirm the generated
// higher-arity constructors project every column in order.
func TestHighAritySelectGolden(t *testing.T) {
	q := Select8(
		Book.ID, Book.Title, Book.Price, Book.AuthorID,
		Author.ID, Author.Name, Book.Title, Book.Price,
	).From(Book)
	want := `SELECT "book"."id", "book"."title", "book"."price", "book"."author_id", ` +
		`"author"."id", "author"."name", "book"."title", "book"."price" FROM "book"`
	checkSQL(t, q, want, nil)
}

// TestHighArityRecordRoundTrip queues a row through the fake driver and fetches
// it via the generated Select8, asserting every typed value survives the trip.
func TestHighArityRecordRoundTrip(t *testing.T) {
	resetFake()
	queueRows(
		[]string{"id", "title", "price", "author_id", "aid", "name", "title2", "price2"},
		[]driver.Value{
			int64(1), "Go", 39.99, int64(7),
			int64(7), "Donovan", "Go2", 12.5,
		},
	)
	db := openFakeDB()
	defer db.Close()

	rows, err := Select8(
		Book.ID, Book.Title, Book.Price, Book.AuthorID,
		Author.ID, Author.Name, Book.Title, Book.Price,
	).From(Book).Fetch(context.Background(), db)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.V1 != 1 || r.V2 != "Go" || r.V3 != 39.99 || r.V4 != 7 {
		t.Errorf("book values = %+v", r)
	}
	if r.V5 != 7 || r.V6 != "Donovan" || r.V7 != "Go2" || r.V8 != 12.5 {
		t.Errorf("author/extra values = %+v", r)
	}
}

// TestCrossDialectInsert renders the same INSERT for PostgreSQL and SQLite and
// asserts the placeholder difference between them. Both dialects double-quote
// identifiers, so the rendered SQL differs only in the bind placeholders ($1,
// $2 versus ?, ?) while the arguments stay identical.
func TestCrossDialectInsert(t *testing.T) {
	q := InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go")

	postgresSQL, postgresArgs, err := q.SQLFor(Postgres())
	if err != nil {
		t.Fatal(err)
	}
	if want := `INSERT INTO "book" ("id", "title") VALUES ($1, $2)`; postgresSQL != want {
		t.Errorf("postgres = %q, want %q", postgresSQL, want)
	}

	sqliteSQL, sqliteArgs, err := q.SQLFor(SQLite())
	if err != nil {
		t.Fatal(err)
	}
	if want := `INSERT INTO "book" ("id", "title") VALUES (?, ?)`; sqliteSQL != want {
		t.Errorf("sqlite = %q, want %q", sqliteSQL, want)
	}

	// The two renderings differ in their bind placeholders but share identical
	// arguments.
	if postgresSQL == sqliteSQL {
		t.Error("postgres and sqlite renderings should differ in bind placeholders")
	}
	if !reflect.DeepEqual(postgresArgs, sqliteArgs) {
		t.Errorf("args differ: postgres=%#v sqlite=%#v", postgresArgs, sqliteArgs)
	}
}

// countSubstr counts the non-overlapping occurrences of sub in s.
func countSubstr(s, sub string) int {
	count := 0
	for i := 0; i+len(sub) <= len(s); {
		if s[i:i+len(sub)] == sub {
			count++
			i += len(sub)
		} else {
			i++
		}
	}
	return count
}
