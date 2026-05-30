package gooq

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
)

type sqlProducer interface {
	SQL() (string, []any, error)
}

func checkSQL(t *testing.T, q sqlProducer, wantSQL string, wantArgs []any) {
	t.Helper()
	sql, args, err := q.SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != wantSQL {
		t.Errorf("sql  = %q\nwant = %q", sql, wantSQL)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestInsertGolden(t *testing.T) {
	t.Run("columns values", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go"),
			`INSERT INTO "book" ("id", "title") VALUES ($1, $2)`,
			[]any{int64(1), "Go"},
		)
	})

	t.Run("multi row", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Columns(Book.ID, Book.Title).
				Values(int64(1), "Go").
				Values(int64(2), "Rust"),
			`INSERT INTO "book" ("id", "title") VALUES ($1, $2), ($3, $4)`,
			[]any{int64(1), "Go", int64(2), "Rust"},
		)
	})

	t.Run("set form", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Set(Book.Title.Set("Go")).Set(Book.Price.Set(10)),
			`INSERT INTO "book" ("title", "price") VALUES ($1, $2)`,
			[]any{"Go", float64(10)},
		)
	})

	t.Run("default values", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).DefaultValues(),
			`INSERT INTO "book" DEFAULT VALUES`,
			nil,
		)
	})

	t.Run("on conflict do nothing", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go").
				OnConflict(Book.ID).DoNothing(),
			`INSERT INTO "book" ("id", "title") VALUES ($1, $2) ON CONFLICT ("id") DO NOTHING`,
			[]any{int64(1), "Go"},
		)
	})

	t.Run("on conflict do update set excluded", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go").
				OnConflict(Book.ID).DoUpdateSet(SetToExcluded(Book.Title)),
			`INSERT INTO "book" ("id", "title") VALUES ($1, $2) ON CONFLICT ("id") DO UPDATE SET "title" = EXCLUDED."title"`,
			[]any{int64(1), "Go"},
		)
	})

	t.Run("on conflict do update set value", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go").
				OnConflict(Book.ID).DoUpdateSet(Book.Title.Set("Updated")),
			`INSERT INTO "book" ("id", "title") VALUES ($1, $2) ON CONFLICT ("id") DO UPDATE SET "title" = $3`,
			[]any{int64(1), "Go", "Updated"},
		)
	})

	t.Run("returning", func(t *testing.T) {
		checkSQL(t,
			InsertInto(Book).Columns(Book.Title).Values("Go").Returning(Book.ID),
			`INSERT INTO "book" ("title") VALUES ($1) RETURNING "id"`,
			[]any{"Go"},
		)
	})
}

func TestInsertMySQLUpsert(t *testing.T) {
	q := InsertInto(Book).Columns(Book.ID, Book.Title).Values(int64(1), "Go").
		OnDuplicateKeyUpdate(SetToExcluded(Book.Title))
	sql, args, err := q.SQLFor(MySQL())
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO `book` (`id`, `title`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `title` = VALUES(`title`)"
	if sql != want {
		t.Errorf("sql  = %q\nwant = %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any{int64(1), "Go"}) {
		t.Errorf("args = %#v", args)
	}
}

func TestReturningUnsupportedOnMySQL(t *testing.T) {
	q := InsertInto(Book).Columns(Book.Title).Values("Go").Returning(Book.ID)
	if _, _, err := q.SQLFor(MySQL()); !errors.Is(err, ErrReturningUnsupported) {
		t.Fatalf("err = %v, want ErrReturningUnsupported", err)
	}
}

func TestUpdateGolden(t *testing.T) {
	t.Run("set where", func(t *testing.T) {
		checkSQL(t,
			Update(Book).Set(Book.Title.Set("Go")).Where(Book.ID.EQ(1)),
			`UPDATE "book" SET "title" = $1 WHERE "book"."id" = $2`,
			[]any{"Go", int64(1)},
		)
	})

	t.Run("multiple set and condition", func(t *testing.T) {
		checkSQL(t,
			Update(Book).Set(Book.Title.Set("Go")).Set(Book.Price.Set(10)).
				Where(Book.ID.EQ(1)).And(Book.AuthorID.EQ(2)),
			`UPDATE "book" SET "title" = $1, "price" = $2 WHERE ("book"."id" = $3 AND "book"."author_id" = $4)`,
			[]any{"Go", float64(10), int64(1), int64(2)},
		)
	})

	t.Run("returning", func(t *testing.T) {
		checkSQL(t,
			Update(Book).Set(Book.Price.Set(10)).Where(Book.ID.EQ(1)).Returning(Book.ID),
			`UPDATE "book" SET "price" = $1 WHERE "book"."id" = $2 RETURNING "id"`,
			[]any{float64(10), int64(1)},
		)
	})
}

func TestDeleteGolden(t *testing.T) {
	t.Run("where", func(t *testing.T) {
		checkSQL(t,
			DeleteFrom(Book).Where(Book.ID.EQ(1)),
			`DELETE FROM "book" WHERE "book"."id" = $1`,
			[]any{int64(1)},
		)
	})

	t.Run("where or returning", func(t *testing.T) {
		checkSQL(t,
			DeleteFrom(Book).Where(Book.ID.EQ(1)).Or(Book.ID.EQ(2)).Returning(Book.ID),
			`DELETE FROM "book" WHERE ("book"."id" = $1 OR "book"."id" = $2) RETURNING "id"`,
			[]any{int64(1), int64(2)},
		)
	})
}

func TestDMLExecute(t *testing.T) {
	db := openFakeDB()
	defer db.Close()
	ctx := context.Background()

	resetFake()
	res, err := InsertInto(Book).Columns(Book.Title).Values("Go").Execute(ctx, db)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Errorf("rows affected = %d, want 1", n)
	}
	q, args := lastQuery()
	if want := `INSERT INTO "book" ("title") VALUES ($1)`; q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
	if len(args) != 1 || args[0].(string) != "Go" {
		t.Errorf("args = %#v", args)
	}

	// Confirm the driver received a driver.Value-compatible argument.
	var _ driver.Value = args[0]
}
