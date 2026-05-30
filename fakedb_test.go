package gooq

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
)

// This file implements a minimal in-process database/sql driver so execution
// paths (Fetch, FetchOne, Execute) can be tested against real *sql.Rows and
// *sql.Result values without any external dependency or live database.

func init() {
	sql.Register("jooqfake", fakeDriver{})
}

// openFakeDB returns a *sql.DB backed by the fake driver.
func openFakeDB() *sql.DB {
	db, err := sql.Open("jooqfake", "")
	if err != nil {
		panic(err)
	}
	return db
}

type fakeResponse struct {
	columns []string
	rows    [][]driver.Value
}

var (
	fakeMu        sync.Mutex
	fakeResponses []fakeResponse // FIFO queue consumed by successive queries
	fakeLastQuery string         // last SQL seen by the driver
	fakeLastArgs  []driver.Value // last argument list seen by the driver
)

// queueRows enqueues a canned result set for the next query.
func queueRows(columns []string, rows ...[]driver.Value) {
	fakeMu.Lock()
	defer fakeMu.Unlock()
	fakeResponses = append(fakeResponses, fakeResponse{columns: columns, rows: rows})
}

// resetFake clears all queued responses and captured state.
func resetFake() {
	fakeMu.Lock()
	defer fakeMu.Unlock()
	fakeResponses = nil
	fakeLastQuery = ""
	fakeLastArgs = nil
}

func lastQuery() (string, []driver.Value) {
	fakeMu.Lock()
	defer fakeMu.Unlock()
	return fakeLastQuery, fakeLastArgs
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return &fakeConn{}, nil }

type fakeConn struct{}

func (*fakeConn) Prepare(query string) (driver.Stmt, error) { return &fakeStmt{}, nil }
func (*fakeConn) Close() error                              { return nil }
func (*fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("jooqfake: transactions unsupported")
}

func namedToValues(named []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(named))
	for i, n := range named {
		out[i] = n.Value
	}
	return out
}

func (*fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	fakeMu.Lock()
	defer fakeMu.Unlock()
	fakeLastQuery = query
	fakeLastArgs = namedToValues(args)
	if len(fakeResponses) == 0 {
		return &fakeRows{}, nil
	}
	resp := fakeResponses[0]
	fakeResponses = fakeResponses[1:]
	return &fakeRows{columns: resp.columns, data: resp.rows}, nil
}

func (*fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	fakeMu.Lock()
	defer fakeMu.Unlock()
	fakeLastQuery = query
	fakeLastArgs = namedToValues(args)
	return driver.RowsAffected(int64(len(args))), nil
}

type fakeStmt struct{}

func (*fakeStmt) Close() error                               { return nil }
func (*fakeStmt) NumInput() int                              { return -1 }
func (*fakeStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (*fakeStmt) Query([]driver.Value) (driver.Rows, error)  { return &fakeRows{}, nil }

type fakeRows struct {
	columns []string
	data    [][]driver.Value
	pos     int
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}
