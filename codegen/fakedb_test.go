package codegen

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
)

// This file implements a minimal in-process database/sql driver so the
// introspection path can be tested against a real *sql.DB without any external
// dependency or live database. The driver dispatches on the text of the query to
// replay the canned rows that the corresponding catalog query would return.

func init() {
	sql.Register("gooqgenfake", fakeDriver{})
}

// fakeCatalog holds the canned catalog rows the fake driver replays for an
// introspection run. Each field corresponds to one of the introspection queries
// and is matched on a distinctive fragment of that query's text. Tests assign a
// fakeCatalog before opening the database.
type fakeCatalog struct {
	// relations are rows of (table_name, table_type).
	relations [][]driver.Value
	// columns are rows of (table_name, column_name, data_type, is_nullable,
	// udt_name).
	columns [][]driver.Value
	// enums are rows of (typname, enumlabel).
	enums [][]driver.Value
	// primaryKeys are rows of (table_name, column_name).
	primaryKeys [][]driver.Value
	// uniques are rows of (table_name, constraint_name, column_name).
	uniques [][]driver.Value
	// foreignKeys are rows of (table_name, constraint_name, column_name,
	// ref_table, ref_column).
	foreignKeys [][]driver.Value
}

// activeCatalog is the catalog the fake driver replays. Tests set it directly.
var activeCatalog fakeCatalog

// catalogColumns maps each catalog query to the column names the fake driver
// reports, matching the projection of the real introspection queries.
var (
	relationsColumns  = []string{"table_name", "table_type"}
	columnsColumns    = []string{"table_name", "column_name", "data_type", "is_nullable", "udt_name"}
	enumsColumns      = []string{"typname", "enumlabel"}
	primaryKeyColumns = []string{"table_name", "column_name"}
	uniqueColumns     = []string{"table_name", "constraint_name", "column_name"}
	foreignKeyColumns = []string{"table_name", "constraint_name", "column_name", "ref_table", "ref_column"}
)

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

// QueryContext dispatches on the query text to return the matching canned rows.
func (*fakeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	columns, data := selectCatalogRows(query)
	rows := make([][]driver.Value, len(data))
	copy(rows, data)
	return &fakeRows{columns: columns, data: rows}, nil
}

// selectCatalogRows returns the column names and canned rows for the catalog
// query identified by a distinctive fragment of its text.
func selectCatalogRows(query string) ([]string, [][]driver.Value) {
	switch {
	case strings.Contains(query, "information_schema.tables"):
		return relationsColumns, activeCatalog.relations
	case strings.Contains(query, "information_schema.columns"):
		return columnsColumns, activeCatalog.columns
	case strings.Contains(query, "pg_enum"):
		return enumsColumns, activeCatalog.enums
	case strings.Contains(query, "PRIMARY KEY"):
		return primaryKeyColumns, activeCatalog.primaryKeys
	case strings.Contains(query, "'UNIQUE'"):
		return uniqueColumns, activeCatalog.uniques
	case strings.Contains(query, "pg_constraint"):
		return foreignKeyColumns, activeCatalog.foreignKeys
	default:
		return nil, nil
	}
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
