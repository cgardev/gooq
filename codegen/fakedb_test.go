package codegen

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
)

// This file implements a minimal in-process database/sql driver so the
// introspection path can be tested against a real *sql.DB without any external
// dependency or live database. The driver replays the canned rows held in
// fakeInfoSchemaRows as the result of any query.

func init() {
	sql.Register("gooqgenfake", fakeDriver{})
}

// fakeInfoSchemaRows holds the canned information_schema rows returned by the
// fake driver. Each row must contain four values in the column order
// table_name, column_name, data_type, is_nullable. Tests set this before
// opening the database.
var fakeInfoSchemaRows [][]driver.Value

// fakeInfoSchemaColumns are the column names the fake driver reports, matching
// the projection of the real introspection query.
var fakeInfoSchemaColumns = []string{"table_name", "column_name", "data_type", "is_nullable"}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return &fakeConn{}, nil }

type fakeConn struct{}

func (*fakeConn) Prepare(string) (driver.Stmt, error) { return &fakeStmt{}, nil }
func (*fakeConn) Close() error                        { return nil }
func (*fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("gooqgenfake: transactions unsupported")
}

// Ping satisfies driver.Pinger so that sql.DB.PingContext succeeds.
func (*fakeConn) Ping(context.Context) error { return nil }

func (*fakeConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	rows := make([][]driver.Value, len(fakeInfoSchemaRows))
	copy(rows, fakeInfoSchemaRows)
	return &fakeRows{columns: fakeInfoSchemaColumns, data: rows}, nil
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
