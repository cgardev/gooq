// Package codegen introspects a PostgreSQL database and generates typed table
// accessors compatible with the jooq query builder package.
//
// The package depends only on the Go standard library. Database drivers are
// intentionally not imported; callers must blank-import the driver appropriate
// for their database and pass an opened *sql.DB to the introspection and
// generation functions.
package codegen

import (
	"context"
	"database/sql"
)

// Column describes a single table column discovered during introspection.
type Column struct {
	// Name is the unqualified column name as stored in the database.
	Name string
	// DataType is the raw SQL data type reported by the catalog.
	DataType string
	// Nullable reports whether the column accepts NULL values.
	Nullable bool
}

// TableSchema describes a single table together with its columns in their
// declared ordinal order.
type TableSchema struct {
	// Name is the unqualified table name.
	Name string
	// Columns are the table's columns in ordinal position order.
	Columns []Column
}

// introspectQuery selects the columns of every table in a schema, ordered so
// that tables and the columns within them appear in a stable order. The schema
// is supplied as a bound parameter rather than concatenated into the text, which
// removes any risk of SQL injection from the schema name.
const introspectQuery = `SELECT table_name, column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_schema = $1
ORDER BY table_name, ordinal_position`

// Introspect reads the column metadata for the given schema from the standard
// information_schema catalog and groups it into one TableSchema per table,
// preserving the catalog's ordinal ordering. The returned slice is ordered by
// table name.
func Introspect(ctx context.Context, db *sql.DB, schema string) ([]TableSchema, error) {
	rows, err := db.QueryContext(ctx, introspectQuery, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableSchema
	index := make(map[string]int)

	for rows.Next() {
		var tableName, columnName, dataType, isNullable string
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable); err != nil {
			return nil, err
		}

		column := Column{
			Name:     columnName,
			DataType: dataType,
			Nullable: isNullable == "YES",
		}

		pos, ok := index[tableName]
		if !ok {
			pos = len(tables)
			index[tableName] = pos
			tables = append(tables, TableSchema{Name: tableName})
		}
		tables[pos].Columns = append(tables[pos].Columns, column)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}
