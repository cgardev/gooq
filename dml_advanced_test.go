package gooq

import (
	"errors"
	"reflect"
	"testing"
)

func TestInsertSelectPostgres(t *testing.T) {
	stmt := InsertInto(Book).
		Columns(Book.Title).
		Select(Select1(Book.Title).From(Book).Where(Book.ID.GT(5)))
	sql, args, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `INSERT INTO "book" ("title") SELECT "book"."title" FROM "book" WHERE "book"."id" > $1`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{int64(5)}) {
		t.Errorf("args = %v, want [5]", args)
	}
}

func TestInsertSelectSQLite(t *testing.T) {
	stmt := InsertInto(Book).
		Columns(Book.Title).
		Select(Select1(Book.Title).From(Book).Where(Book.ID.GT(5)))
	sql, args, err := stmt.SQLFor(SQLite())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `INSERT INTO "book" ("title") SELECT "book"."title" FROM "book" WHERE "book"."id" > ?`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{int64(5)}) {
		t.Errorf("args = %v, want [5]", args)
	}
}

// TestInsertSelectBindOrdering asserts that the source SELECT renders into the
// same builder as the INSERT, so its placeholders are numbered in render order
// and its arguments accumulate in that order.
func TestInsertSelectBindOrdering(t *testing.T) {
	source := Select1(Book.Title).
		From(Book).
		Where(Book.ID.GT(5)).
		And(Book.Price.LT(20))
	stmt := InsertInto(Book).Columns(Book.Title).Select(source)
	sql, args, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `INSERT INTO "book" ("title") SELECT "book"."title" FROM "book" WHERE ("book"."id" > $1 AND "book"."price" < $2)`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{int64(5), float64(20)}) {
		t.Errorf("args = %v, want [5 20]", args)
	}
}

// TestInsertSelectNestedSubqueryBindOrdering asserts placeholder numbering when
// the source SELECT itself contains a subquery, exercising the bind ordering
// across the outer INSERT, the source SELECT, and the nested subquery, all
// rendered into a single builder.
func TestInsertSelectNestedSubqueryBindOrdering(t *testing.T) {
	nested := Select1(Book.AuthorID).From(Book).Where(Book.Price.GT(100))
	source := Select1(Book.Title).
		From(Book).
		Where(Book.ID.GT(5)).
		And(Book.AuthorID.InSubquery(nested))
	stmt := InsertInto(Book).Columns(Book.Title).Select(source)
	sql, args, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `INSERT INTO "book" ("title") SELECT "book"."title" FROM "book" WHERE ("book"."id" > $1 AND "book"."author_id" IN (SELECT "book"."author_id" FROM "book" WHERE "book"."price" > $2))`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{int64(5), float64(100)}) {
		t.Errorf("args = %v, want [5 100]", args)
	}
}

func TestUpdateFromPostgres(t *testing.T) {
	stmt := Update(Book).
		Set(Book.Price.Set(10)).
		From(Author).
		Where(Book.AuthorID.EQField(Author.ID))
	sql, args, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `UPDATE "book" SET "price" = $1 FROM "author" WHERE "book"."author_id" = "author"."id"`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{float64(10)}) {
		t.Errorf("args = %v, want [10]", args)
	}
}

func TestUpdateFromSQLite(t *testing.T) {
	stmt := Update(Book).
		Set(Book.Price.Set(10)).
		From(Author).
		Where(Book.AuthorID.EQField(Author.ID))
	sql, args, err := stmt.SQLFor(SQLite())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `UPDATE "book" SET "price" = ? FROM "author" WHERE "book"."author_id" = "author"."id"`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{float64(10)}) {
		t.Errorf("args = %v, want [10]", args)
	}
}

func TestDeleteUsingPostgres(t *testing.T) {
	stmt := DeleteFrom(Book).
		UsingTable(Author).
		Where(Book.AuthorID.EQField(Author.ID))
	sql, args, err := stmt.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `DELETE FROM "book" USING "author" WHERE "book"."author_id" = "author"."id"`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want none", args)
	}
}

// TestDeleteUsingSQLiteUnsupported asserts that DELETE ... USING records
// ErrUsingUnsupported under the SQLite dialect, which has no such clause.
func TestDeleteUsingSQLiteUnsupported(t *testing.T) {
	stmt := DeleteFrom(Book).
		UsingTable(Author).
		Where(Book.AuthorID.EQField(Author.ID))
	_, _, err := stmt.SQLFor(SQLite())
	if !errors.Is(err, ErrUsingUnsupported) {
		t.Fatalf("err = %v, want ErrUsingUnsupported", err)
	}
}
