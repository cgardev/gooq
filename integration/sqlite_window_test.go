package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cgardev/gooq"

	_ "modernc.org/sqlite"
)

// openWindowDB opens a fresh in-memory SQLite database for the window and
// advanced-DML execution tests and registers its cleanup. It is named
// distinctly so it does not collide with the shared harness helpers.
func openWindowDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// mustExecRaw runs a raw statement, failing the test on error.
func mustExecRaw(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// seedScores creates and populates a small scores table used by the window
// execution tests.
func seedScores(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExecRaw(t, db, `CREATE TABLE scores (team TEXT NOT NULL, player TEXT NOT NULL, points INTEGER NOT NULL)`)
	rows := []struct {
		team   string
		player string
		points int
	}{
		{"red", "alice", 30},
		{"red", "bob", 20},
		{"red", "carol", 20},
		{"blue", "dave", 50},
		{"blue", "erin", 10},
	}
	for _, r := range rows {
		mustExecRaw(t, db, `INSERT INTO scores (team, player, points) VALUES (?, ?, ?)`, r.team, r.player, r.points)
	}
}

// rawStep adapts a literal SQL string to the SQL() method that BatchExec
// consumes, so the BatchExec execution loop can be exercised against real
// SQLite without depending on a particular table fixture.
type rawStep struct {
	sql  string
	args []any
}

func (s rawStep) SQL() (string, []any, error) { return s.sql, s.args, nil }

// TestSQLiteRowNumberRanks verifies that modernc SQLite evaluates ROW_NUMBER
// OVER (PARTITION BY ... ORDER BY ...) and returns the expected sequential
// numbers per partition. This is the SQL shape produced by gooq's RowNumber
// window constructor.
func TestSQLiteRowNumberRanks(t *testing.T) {
	db := openWindowDB(t)
	seedScores(t, db)

	query := `SELECT player, ROW_NUMBER() OVER (PARTITION BY "team" ORDER BY "points" DESC, "player" ASC) FROM scores ORDER BY team, points DESC, player ASC`
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	type result struct {
		player string
		rn     int64
	}
	var got []result
	for rows.Next() {
		var r result
		if err := rows.Scan(&r.player, &r.rn); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// blue: dave(50)=1, erin(10)=2; red: alice(30)=1, bob(20)=2, carol(20)=3.
	want := []result{
		{"dave", 1}, {"erin", 2},
		{"alice", 1}, {"bob", 2}, {"carol", 3},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestSQLiteRankWithTies verifies that RANK OVER (...) produces gap-after-ties
// ranking, distinguishing it from ROW_NUMBER for tied values, matching the SQL
// shape produced by gooq's Rank window constructor.
func TestSQLiteRankWithTies(t *testing.T) {
	db := openWindowDB(t)
	seedScores(t, db)

	query := `SELECT player, RANK() OVER (PARTITION BY "team" ORDER BY "points" DESC) FROM scores WHERE team = 'red' ORDER BY points DESC, player ASC`
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	ranks := map[string]int64{}
	for rows.Next() {
		var player string
		var rank int64
		if err := rows.Scan(&player, &rank); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ranks[player] = rank
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// alice(30)=1, then bob(20) and carol(20) tie at rank 2.
	if ranks["alice"] != 1 {
		t.Errorf("alice rank = %d, want 1", ranks["alice"])
	}
	if ranks["bob"] != 2 || ranks["carol"] != 2 {
		t.Errorf("tied ranks bob=%d carol=%d, want both 2", ranks["bob"], ranks["carol"])
	}
}

// TestSQLiteInsertSelectCopiesRows verifies that an INSERT ... SELECT of the
// exact shape gooq renders copies matching rows into a destination table under
// real SQLite execution.
func TestSQLiteInsertSelectCopiesRows(t *testing.T) {
	db := openWindowDB(t)
	mustExecRaw(t, db, `CREATE TABLE src (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	mustExecRaw(t, db, `CREATE TABLE dst (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	mustExecRaw(t, db, `INSERT INTO src (id, name) VALUES (1, 'a'), (2, 'b'), (3, 'c')`)

	// This is the rendered form of:
	//   InsertInto(dst).Columns(id, name).
	//     Select(Select2(src.id, src.name).From(src).Where(src.id.GT(1)))
	insertSelect := `INSERT INTO "dst" ("id", "name") SELECT "src"."id", "src"."name" FROM "src" WHERE "src"."id" > ?`
	if _, err := db.ExecContext(context.Background(), insertSelect, int64(1)); err != nil {
		t.Fatalf("insert ... select: %v", err)
	}

	var count, minID int64
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*), COALESCE(MIN(id), 0) FROM dst`).Scan(&count, &minID); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 2 {
		t.Fatalf("dst row count = %d, want 2 (ids 2 and 3)", count)
	}
	if minID != 2 {
		t.Errorf("min copied id = %d, want 2", minID)
	}
}

// TestSQLiteBatchExecRunsStatements verifies that gooq.BatchExec runs several
// statements in order against a single SQLite connection and returns one result
// per statement.
func TestSQLiteBatchExecRunsStatements(t *testing.T) {
	db := openWindowDB(t)
	mustExecRaw(t, db, `CREATE TABLE items (id INTEGER PRIMARY KEY, label TEXT NOT NULL, qty INTEGER NOT NULL)`)

	results, err := gooq.BatchExec(
		context.Background(),
		db,
		rawStep{sql: `INSERT INTO "items" ("id", "label", "qty") VALUES (?, ?, ?)`, args: []any{int64(1), "a", int64(5)}},
		rawStep{sql: `INSERT INTO "items" ("id", "label", "qty") VALUES (?, ?, ?)`, args: []any{int64(2), "b", int64(7)}},
		rawStep{sql: `UPDATE "items" SET "qty" = ? WHERE "id" = ?`, args: []any{int64(10), int64(1)}},
		rawStep{sql: `DELETE FROM "items" WHERE "id" = ?`, args: []any{int64(2)}},
	)
	if err != nil {
		t.Fatalf("batch exec: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("results = %d, want 4", len(results))
	}

	var count, remainingQty int64
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*), COALESCE(SUM(qty), 0) FROM items`).Scan(&count, &remainingQty); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 1 {
		t.Fatalf("remaining rows = %d, want 1", count)
	}
	if remainingQty != 10 {
		t.Errorf("remaining qty = %d, want 10 (updated)", remainingQty)
	}
}
