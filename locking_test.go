package gooq

import (
	"strings"
	"testing"
)

// locking_test.go contains golden SQL tests for the row-locking clause. The
// clause renders for PostgreSQL after LIMIT and OFFSET, and is absent for SQLite
// because that dialect has no row-locking clause.

func TestRowLocking(t *testing.T) {
	users := NewTable("users")
	userID := NewField[int64](users, "id")

	tests := []struct {
		name   string
		query  SelectFinalStep[Record1[int64]]
		wantPG string
	}{
		{
			name:   "for update",
			query:  Select1(userID).From(users).Where(userID.GT(0)).ForUpdate(),
			wantPG: `SELECT "users"."id" FROM "users" WHERE "users"."id" > $1 FOR UPDATE`,
		},
		{
			name:   "for share",
			query:  Select1(userID).From(users).ForShare(),
			wantPG: `SELECT "users"."id" FROM "users" FOR SHARE`,
		},
		{
			name:   "for update skip locked",
			query:  Select1(userID).From(users).ForUpdate().SkipLocked(),
			wantPG: `SELECT "users"."id" FROM "users" FOR UPDATE SKIP LOCKED`,
		},
		{
			name:   "after limit and offset",
			query:  Select1(userID).From(users).OrderBy(userID.Asc()).Limit(10).Offset(5).ForUpdate(),
			wantPG: `SELECT "users"."id" FROM "users" ORDER BY "users"."id" ASC LIMIT 10 OFFSET 5 FOR UPDATE`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/postgres", func(t *testing.T) {
			sql, _, err := tt.query.SQLFor(Postgres())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tt.wantPG {
				t.Errorf("sql = %q, want %q", sql, tt.wantPG)
			}
		})

		t.Run(tt.name+"/sqlite", func(t *testing.T) {
			sql, _, err := tt.query.SQLFor(SQLite())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// SQLite has no locking clause, so it must never appear in the
			// rendered SQL regardless of which locking method was called.
			if strings.Contains(sql, "FOR UPDATE") || strings.Contains(sql, "FOR SHARE") || strings.Contains(sql, "SKIP LOCKED") {
				t.Errorf("sqlite sql unexpectedly contains a locking clause: %q", sql)
			}
		})
	}
}
