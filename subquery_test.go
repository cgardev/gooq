package gooq

import (
	"reflect"
	"testing"
)

// subquery_test.go contains golden SQL tests for EXISTS / NOT EXISTS,
// IN (SELECT ...), and scalar subqueries, asserting both the rendered SQL and
// the bind argument ordering for the PostgreSQL and SQLite dialects.

func TestExistsSubquery(t *testing.T) {
	users := NewTable("users")
	orders := NewTable("orders")
	userID := NewField[int64](users, "id")
	orderUser := NewField[int64](orders, "user_id")
	orderTotal := NewField[int64](orders, "total")

	// EXISTS over a correlated subquery whose own WHERE binds an argument.
	sub := Select1(orderUser).From(orders).Where(orderTotal.GT(100))
	cond := Exists(sub)

	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{
			name:    "postgres",
			dialect: Postgres(),
			want:    `EXISTS (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."total" > $1)`,
		},
		{
			name:    "sqlite",
			dialect: SQLite(),
			want:    `EXISTS (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."total" > ?)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBuilder(tt.dialect)
			cond.render(b)
			sql, args, err := b.result()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tt.want {
				t.Errorf("sql = %q, want %q", sql, tt.want)
			}
			if !reflect.DeepEqual(args, []any{int64(100)}) {
				t.Errorf("args = %v, want [100]", args)
			}
		})
	}

	// NOT EXISTS rendering.
	b := newBuilder(Postgres())
	NotExists(Select1(orderUser).From(orders).Where(orderUser.EQField(userID))).render(b)
	sql, _, err := b.result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `NOT EXISTS (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."user_id" = "users"."id")`
	if sql != want {
		t.Errorf("not exists sql = %q, want %q", sql, want)
	}
}

func TestInSubquery(t *testing.T) {
	users := NewTable("users")
	orders := NewTable("orders")
	userID := NewField[int64](users, "id")
	orderUser := NewField[int64](orders, "user_id")
	orderTotal := NewField[int64](orders, "total")

	sub := Select1(orderUser).From(orders).Where(orderTotal.GT(50))
	cond := userID.InSubquery(sub)

	tests := []struct {
		name    string
		dialect Dialect
		want    string
	}{
		{
			name:    "postgres",
			dialect: Postgres(),
			want:    `"users"."id" IN (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."total" > $1)`,
		},
		{
			name:    "sqlite",
			dialect: SQLite(),
			want:    `"users"."id" IN (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."total" > ?)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBuilder(tt.dialect)
			cond.render(b)
			sql, args, err := b.result()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tt.want {
				t.Errorf("sql = %q, want %q", sql, tt.want)
			}
			if !reflect.DeepEqual(args, []any{int64(50)}) {
				t.Errorf("args = %v, want [50]", args)
			}
		})
	}

	// NOT IN (subquery).
	b := newBuilder(Postgres())
	userID.NotInSubquery(Select1(orderUser).From(orders)).render(b)
	sql, _, err := b.result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"users"."id" NOT IN (SELECT "orders"."user_id" FROM "orders")`
	if sql != want {
		t.Errorf("not in sql = %q, want %q", sql, want)
	}
}

// TestSubqueryBindOrdering asserts that an outer predicate and the predicate of
// an IN-subquery bind their arguments in render order, so PostgreSQL numbers the
// placeholders $1, $2 in the correct positions and the argument slice is
// [outer, inner].
func TestSubqueryBindOrdering(t *testing.T) {
	users := NewTable("users")
	orders := NewTable("orders")
	userID := NewField[int64](users, "id")
	userName := NewField[string](users, "name")
	orderUser := NewField[int64](orders, "user_id")
	orderTotal := NewField[int64](orders, "total")

	// WHERE name = ? AND id IN (SELECT user_id FROM orders WHERE total = ?)
	query := Select1(userID).
		From(users).
		Where(userName.EQ("alice")).
		And(userID.InSubquery(
			Select1(orderUser).From(orders).Where(orderTotal.EQ(999)),
		))

	t.Run("postgres", func(t *testing.T) {
		sql, args, err := query.SQLFor(Postgres())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `SELECT "users"."id" FROM "users" WHERE ("users"."name" = $1 AND "users"."id" IN (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."total" = $2))`
		if sql != want {
			t.Errorf("sql = %q, want %q", sql, want)
		}
		if !reflect.DeepEqual(args, []any{"alice", int64(999)}) {
			t.Errorf("args = %v, want [alice 999]", args)
		}
	})

	t.Run("sqlite", func(t *testing.T) {
		sql, args, err := query.SQLFor(SQLite())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `SELECT "users"."id" FROM "users" WHERE ("users"."name" = ? AND "users"."id" IN (SELECT "orders"."user_id" FROM "orders" WHERE "orders"."total" = ?))`
		if sql != want {
			t.Errorf("sql = %q, want %q", sql, want)
		}
		if !reflect.DeepEqual(args, []any{"alice", int64(999)}) {
			t.Errorf("args = %v, want [alice 999]", args)
		}
	})
}

func TestScalarSubquery(t *testing.T) {
	users := NewTable("users")
	orders := NewTable("orders")
	userID := NewField[int64](users, "id")
	orderUser := NewField[int64](orders, "user_id")
	orderTotal := NewField[int64](orders, "total")

	// A scalar subquery projected alongside a column.
	maxTotal := ScalarSubquery[int64](
		Select1(Max(orderTotal)).From(orders).Where(orderUser.EQField(userID)),
	)
	query := Select2(userID, maxTotal).From(users)

	sql, args, err := query.SQLFor(Postgres())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "users"."id", (SELECT MAX("orders"."total") FROM "orders" WHERE "orders"."user_id" = "users"."id") FROM "users"`
	if sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if !reflect.DeepEqual(args, []any(nil)) {
		t.Errorf("args = %v, want nil", args)
	}
}
