package gooq

import (
	"reflect"
	"testing"
)

// setop_test.go contains golden SQL tests for the set operations UNION,
// UNION ALL, INTERSECT, and EXCEPT, asserting the rendered SQL and bind
// ordering across both operands for the PostgreSQL and SQLite dialects.

func TestSetOperations(t *testing.T) {
	users := NewTable("users")
	admins := NewTable("admins")
	userID := NewField[int64](users, "id")
	adminID := NewField[int64](admins, "id")

	left := Select1(userID).From(users)
	right := Select1(adminID).From(admins)

	tests := []struct {
		name string
		// query is rebuilt per case because each set operation derives a new
		// final step from the same left and right operands.
		query SelectFinalStep[Record1[int64]]
		want  string
	}{
		{
			name:  "union",
			query: left.Union(right),
			want:  `SELECT "users"."id" FROM "users" UNION SELECT "admins"."id" FROM "admins"`,
		},
		{
			name:  "union all",
			query: left.UnionAll(right),
			want:  `SELECT "users"."id" FROM "users" UNION ALL SELECT "admins"."id" FROM "admins"`,
		},
		{
			name:  "intersect",
			query: left.Intersect(right),
			want:  `SELECT "users"."id" FROM "users" INTERSECT SELECT "admins"."id" FROM "admins"`,
		},
		{
			name:  "except",
			query: left.Except(right),
			want:  `SELECT "users"."id" FROM "users" EXCEPT SELECT "admins"."id" FROM "admins"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/postgres", func(t *testing.T) {
			sql, _, err := tt.query.SQLFor(Postgres())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tt.want {
				t.Errorf("sql = %q, want %q", sql, tt.want)
			}
		})
		t.Run(tt.name+"/sqlite", func(t *testing.T) {
			sql, _, err := tt.query.SQLFor(SQLite())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// The set-operation keyword renders identically in both dialects;
			// only placeholder spelling would differ when arguments are present.
			if sql != tt.want {
				t.Errorf("sql = %q, want %q", sql, tt.want)
			}
		})
	}
}

// TestSetOperationBindOrdering asserts that the bind arguments of both operands
// interleave in render order, so PostgreSQL numbers the placeholders $1, $2 from
// left to right with the argument slice [left, right].
func TestSetOperationBindOrdering(t *testing.T) {
	users := NewTable("users")
	admins := NewTable("admins")
	userID := NewField[int64](users, "id")
	userTier := NewField[int64](users, "tier")
	adminID := NewField[int64](admins, "id")
	adminLevel := NewField[int64](admins, "level")

	left := Select1(userID).From(users).Where(userTier.EQ(1))
	right := Select1(adminID).From(admins).Where(adminLevel.EQ(9))
	query := left.UnionAll(right)

	t.Run("postgres", func(t *testing.T) {
		sql, args, err := query.SQLFor(Postgres())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `SELECT "users"."id" FROM "users" WHERE "users"."tier" = $1 UNION ALL SELECT "admins"."id" FROM "admins" WHERE "admins"."level" = $2`
		if sql != want {
			t.Errorf("sql = %q, want %q", sql, want)
		}
		if !reflect.DeepEqual(args, []any{int64(1), int64(9)}) {
			t.Errorf("args = %v, want [1 9]", args)
		}
	})

	t.Run("sqlite", func(t *testing.T) {
		sql, args, err := query.SQLFor(SQLite())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `SELECT "users"."id" FROM "users" WHERE "users"."tier" = ? UNION ALL SELECT "admins"."id" FROM "admins" WHERE "admins"."level" = ?`
		if sql != want {
			t.Errorf("sql = %q, want %q", sql, want)
		}
		if !reflect.DeepEqual(args, []any{int64(1), int64(9)}) {
			t.Errorf("args = %v, want [1 9]", args)
		}
	})
}
