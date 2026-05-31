package gooq

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"sort"
	"testing"
)

// This file exercises the result-mapping helpers (FetchInto, FetchOneInto,
// FetchMap, FetchGroups) and the typed RETURNING helpers (ReturningInto,
// ReturningOneInto). The golden cases assert the rendered RETURNING SQL for both
// dialects; the mapping cases run against the package's existing in-process
// "jooqfake" driver (see fakedb_test.go) so the reflection-based scanner is
// exercised end to end without any external dependency.

func TestReturningIntoRendersStatementSQL(t *testing.T) {
	tests := []struct {
		name string
		stmt statement
		sql  string
		args []any
	}{
		{
			name: "insert returning postgres",
			stmt: InsertInto(Book).Columns(Book.Title).Values("Go").Returning(Book.ID).Using(Postgres()),
			sql:  `INSERT INTO "book" ("title") VALUES ($1) RETURNING "id"`,
			args: []any{"Go"},
		},
		{
			name: "insert returning sqlite",
			stmt: InsertInto(Book).Columns(Book.Title).Values("Go").Returning(Book.ID).Using(SQLite()),
			sql:  `INSERT INTO "book" ("title") VALUES (?) RETURNING "id"`,
			args: []any{"Go"},
		},
		{
			name: "delete returning postgres",
			stmt: DeleteFrom(Book).Where(Book.ID.EQ(7)).Returning(Book.ID, Book.Title).Using(Postgres()),
			sql:  `DELETE FROM "book" WHERE "book"."id" = $1 RETURNING "id", "title"`,
			args: []any{int64(7)},
		},
		{
			name: "delete returning sqlite",
			stmt: DeleteFrom(Book).Where(Book.ID.EQ(7)).Returning(Book.ID, Book.Title).Using(SQLite()),
			sql:  `DELETE FROM "book" WHERE "book"."id" = ? RETURNING "id", "title"`,
			args: []any{int64(7)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query, args, err := tc.stmt.SQL()
			if err != nil {
				t.Fatalf("SQL() error: %v", err)
			}
			if query != tc.sql {
				t.Errorf("sql = %q, want %q", query, tc.sql)
			}
			if !reflect.DeepEqual(args, tc.args) {
				t.Errorf("args = %#v, want %#v", args, tc.args)
			}
		})
	}
}

// fakeStatement adapts a column list and row data already queued through
// queueRows to the statement interface, so the mapping helpers can run them
// through the jooqfake driver without rendering real SQL.
type fakeStatement struct{}

func (fakeStatement) SQL() (string, []any, error) { return "SELECT mapped", nil, nil }

// scannableID is a custom sql.Scanner used to prove that fields implementing the
// standard library Scanner interface are honored by the mapper.
type scannableID struct {
	value int64
}

func (s *scannableID) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		s.value = v
		return nil
	case nil:
		s.value = -1
		return nil
	default:
		return errors.New("scannableID: unsupported source")
	}
}

// mappedRow is the destination struct for the mapping cases. It combines a db
// tag, a field-name fallback, a pointer field, a custom Scanner field, and a
// nullable column to cover every supported mapping path in one type.
type mappedRow struct {
	ID       int64  `db:"id"`
	Title    string // matched case-insensitively to the "title" column
	Pages    *int64 `db:"pages"`
	Custom   scannableID
	Subtitle sql.Null[string] `db:"subtitle"`
}

func TestFetchIntoMapsColumnsByTagAndName(t *testing.T) {
	resetFake()
	queueRows(
		[]string{"id", "title", "pages", "custom", "subtitle", "ignored"},
		[]driver.Value{int64(1), "The Go Programming Language", int64(380), int64(99), "An Idiomatic Guide", "discard me"},
		[]driver.Value{int64(2), "The Practice of Programming", nil, nil, nil, "discard me too"},
	)
	db := openFakeDB()
	defer db.Close()

	got, err := FetchInto[mappedRow](context.Background(), db, fakeStatement{})
	if err != nil {
		t.Fatalf("FetchInto error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}

	first := got[0]
	if first.ID != 1 {
		t.Errorf("ID = %d, want 1", first.ID)
	}
	if first.Title != "The Go Programming Language" {
		t.Errorf("Title = %q, want the Go book", first.Title)
	}
	if first.Pages == nil || *first.Pages != 380 {
		t.Errorf("Pages = %v, want pointer to 380", first.Pages)
	}
	if first.Custom.value != 99 {
		t.Errorf("Custom = %d, want 99 via Scanner", first.Custom.value)
	}
	if !first.Subtitle.Valid || first.Subtitle.V != "An Idiomatic Guide" {
		t.Errorf("Subtitle = %#v, want valid subtitle", first.Subtitle)
	}

	second := got[1]
	if second.Pages != nil {
		t.Errorf("Pages = %v, want nil for NULL column", second.Pages)
	}
	if second.Custom.value != -1 {
		t.Errorf("Custom = %d, want -1 for NULL via Scanner", second.Custom.value)
	}
	if second.Subtitle.Valid {
		t.Errorf("Subtitle = %#v, want NULL", second.Subtitle)
	}
}

func TestFetchOneIntoCardinality(t *testing.T) {
	db := openFakeDB()
	defer db.Close()
	ctx := context.Background()

	t.Run("zero rows returns the zero value", func(t *testing.T) {
		resetFake()
		queueRows([]string{"id", "title"})
		row, err := FetchOneInto[mappedRow](ctx, db, fakeStatement{})
		if err != nil {
			t.Fatalf("FetchOneInto error: %v", err)
		}
		if row.ID != 0 || row.Title != "" {
			t.Errorf("row = %#v, want zero value", row)
		}
	})

	t.Run("one row returns it", func(t *testing.T) {
		resetFake()
		queueRows([]string{"id", "title"}, []driver.Value{int64(5), "solo"})
		row, err := FetchOneInto[mappedRow](ctx, db, fakeStatement{})
		if err != nil {
			t.Fatalf("FetchOneInto error: %v", err)
		}
		if row.ID != 5 || row.Title != "solo" {
			t.Errorf("row = %#v, want the single row", row)
		}
	})

	t.Run("many rows reports ErrTooManyRows", func(t *testing.T) {
		resetFake()
		queueRows([]string{"id", "title"}, []driver.Value{int64(1), "a"}, []driver.Value{int64(2), "b"})
		_, err := FetchOneInto[mappedRow](ctx, db, fakeStatement{})
		if !errors.Is(err, ErrTooManyRows) {
			t.Errorf("error = %v, want ErrTooManyRows", err)
		}
	})
}

func TestFetchMapIndexesByKeyColumn(t *testing.T) {
	resetFake()
	queueRows(
		[]string{"id", "title"},
		[]driver.Value{int64(1), "first"},
		[]driver.Value{int64(2), "second"},
	)
	db := openFakeDB()
	defer db.Close()

	got, err := FetchMap[int64, mappedRow](context.Background(), db, fakeStatement{}, "id")
	if err != nil {
		t.Fatalf("FetchMap error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("entries = %d, want 2", len(got))
	}
	if got[1].Title != "first" || got[2].Title != "second" {
		t.Errorf("map = %#v, want keyed by id", got)
	}
}

func TestFetchGroupsGroupsByKeyColumn(t *testing.T) {
	resetFake()
	queueRows(
		[]string{"id", "title"},
		[]driver.Value{int64(1), "alpha"},
		[]driver.Value{int64(1), "beta"},
		[]driver.Value{int64(2), "gamma"},
	)
	db := openFakeDB()
	defer db.Close()

	got, err := FetchGroups[int64, mappedRow](context.Background(), db, fakeStatement{}, "id")
	if err != nil {
		t.Fatalf("FetchGroups error: %v", err)
	}
	if len(got[1]) != 2 {
		t.Fatalf("group 1 size = %d, want 2", len(got[1]))
	}
	titles := []string{got[1][0].Title, got[1][1].Title}
	sort.Strings(titles)
	if !reflect.DeepEqual(titles, []string{"alpha", "beta"}) {
		t.Errorf("group 1 titles = %v, want alpha and beta", titles)
	}
	if len(got[2]) != 1 || got[2][0].Title != "gamma" {
		t.Errorf("group 2 = %#v, want a single gamma", got[2])
	}
}

func TestFetchMapRejectsUnknownKeyColumn(t *testing.T) {
	resetFake()
	queueRows([]string{"id", "title"}, []driver.Value{int64(1), "x"})
	db := openFakeDB()
	defer db.Close()

	_, err := FetchMap[int64, mappedRow](context.Background(), db, fakeStatement{}, "missing")
	if err == nil {
		t.Fatal("FetchMap with an unknown key column should error")
	}
}

func TestReturningIntoMapsRows(t *testing.T) {
	resetFake()
	queueRows([]string{"id"}, []driver.Value{int64(42)})
	db := openFakeDB()
	defer db.Close()

	type idOnly struct {
		ID int64 `db:"id"`
	}
	step := InsertInto(Book).Columns(Book.Title).Values("Go").Returning(Book.ID)
	got, err := ReturningInto[idOnly](context.Background(), db, step)
	if err != nil {
		t.Fatalf("ReturningInto error: %v", err)
	}
	if len(got) != 1 || got[0].ID != 42 {
		t.Errorf("returned = %#v, want a single id of 42", got)
	}
}
