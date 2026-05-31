package gooq

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// batchRecordingDB is a minimal Querier that records each executed query and
// its arguments, returning a sentinel result per call. It backs the BatchExec
// unit tests without a real database.
type batchRecordingDB struct {
	queries []string
	args    [][]any
	failAt  int // one-based index of the call that should fail; 0 disables
	calls   int
}

func (r *batchRecordingDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("not used")
}

func (r *batchRecordingDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	r.calls++
	if r.failAt != 0 && r.calls == r.failAt {
		return nil, errors.New("exec failed")
	}
	r.queries = append(r.queries, query)
	r.args = append(r.args, args)
	return batchFakeResult{}, nil
}

// batchFakeResult is a no-op sql.Result.
type batchFakeResult struct{}

func (batchFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (batchFakeResult) RowsAffected() (int64, error) { return 0, nil }

func TestBatchExecRunsAllSteps(t *testing.T) {
	db := &batchRecordingDB{}
	results, err := BatchExec(
		context.Background(),
		db,
		InsertInto(Book).Columns(Book.Title).Values("Go"),
		Update(Book).Set(Book.Price.Set(10)).Where(Book.ID.EQ(1)),
		DeleteFrom(Book).Where(Book.ID.EQ(2)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if len(db.queries) != 3 {
		t.Fatalf("executed %d queries, want 3", len(db.queries))
	}
}

func TestBatchExecStopsOnError(t *testing.T) {
	db := &batchRecordingDB{failAt: 2}
	results, err := BatchExec(
		context.Background(),
		db,
		InsertInto(Book).Columns(Book.Title).Values("Go"),
		Update(Book).Set(Book.Price.Set(10)).Where(Book.ID.EQ(1)),
		DeleteFrom(Book).Where(Book.ID.EQ(2)),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (only the first step before the failure)", len(results))
	}
	if db.calls != 2 {
		t.Fatalf("calls = %d, want 2 (stopped after the failing step)", db.calls)
	}
}

func TestBatchExecRenderErrorStops(t *testing.T) {
	db := &batchRecordingDB{}
	// A DELETE ... USING rendered for SQLite produces a deferred render error,
	// which BatchExec surfaces without executing the offending statement.
	_, err := BatchExec(
		context.Background(),
		db,
		DeleteFrom(Book).UsingTable(Author).Where(Book.AuthorID.EQField(Author.ID)).Using(SQLite()),
	)
	if !errors.Is(err, ErrUsingUnsupported) {
		t.Fatalf("err = %v, want ErrUsingUnsupported", err)
	}
	if db.calls != 0 {
		t.Fatalf("calls = %d, want 0 (nothing executed)", db.calls)
	}
}
