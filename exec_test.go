package gooq

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

func TestFetchReturnsTypedRows(t *testing.T) {
	resetFake()
	queueRows(
		[]string{"title", "price"},
		[]driver.Value{"The Go Programming Language", 39.99},
		[]driver.Value{"Effective Go", 0.0},
	)
	db := openFakeDB()
	defer db.Close()

	rows, err := Select2(Book.Title, Book.Price).From(Book).
		Where(Book.Price.GE(0)).
		Fetch(context.Background(), db)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].V1 != "The Go Programming Language" || rows[0].V2 != 39.99 {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[1].V1 != "Effective Go" || rows[1].V2 != 0.0 {
		t.Errorf("row1 = %+v", rows[1])
	}

	// The driver must have seen the rendered SQL and the bound argument.
	q, args := lastQuery()
	if want := `SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" >= $1`; q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
	if len(args) != 1 || args[0] != int64(0) && args[0] != float64(0) {
		t.Errorf("args = %#v", args)
	}
}

func TestFetchOneCardinality(t *testing.T) {
	db := openFakeDB()
	defer db.Close()
	ctx := context.Background()

	// Exactly one row.
	resetFake()
	queueRows([]string{"id"}, []driver.Value{int64(7)})
	got, err := Select1(Book.ID).From(Book).FetchOne(ctx, db)
	if err != nil || got.V1 != 7 {
		t.Fatalf("one row: got=%v err=%v", got, err)
	}

	// No rows -> zero value, nil error (jOOQ fetchOne semantics).
	resetFake()
	queueRows([]string{"id"})
	got, err = Select1(Book.ID).From(Book).FetchOne(ctx, db)
	if err != nil {
		t.Fatalf("no rows: unexpected error %v", err)
	}
	if got.V1 != 0 {
		t.Fatalf("no rows: got %v, want zero", got.V1)
	}

	// More than one row -> ErrTooManyRows.
	resetFake()
	queueRows([]string{"id"}, []driver.Value{int64(1)}, []driver.Value{int64(2)})
	_, err = Select1(Book.ID).From(Book).FetchOne(ctx, db)
	if !errors.Is(err, ErrTooManyRows) {
		t.Fatalf("many rows: err = %v, want ErrTooManyRows", err)
	}
}

func TestFetchSingleCardinality(t *testing.T) {
	db := openFakeDB()
	defer db.Close()
	ctx := context.Background()

	// No rows -> sql.ErrNoRows.
	resetFake()
	queueRows([]string{"id"})
	_, err := Select1(Book.ID).From(Book).FetchSingle(ctx, db)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("no rows: err = %v, want sql.ErrNoRows", err)
	}

	// Exactly one row.
	resetFake()
	queueRows([]string{"id"}, []driver.Value{int64(9)})
	got, err := Select1(Book.ID).From(Book).FetchSingle(ctx, db)
	if err != nil || got.V1 != 9 {
		t.Fatalf("one row: got=%v err=%v", got, err)
	}

	// More than one -> ErrTooManyRows.
	resetFake()
	queueRows([]string{"id"}, []driver.Value{int64(1)}, []driver.Value{int64(2)})
	_, err = Select1(Book.ID).From(Book).FetchSingle(ctx, db)
	if !errors.Is(err, ErrTooManyRows) {
		t.Fatalf("many rows: err = %v, want ErrTooManyRows", err)
	}
}
