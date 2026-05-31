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
	"sort"
)

// Column describes a single table column discovered during introspection.
type Column struct {
	// Name is the unqualified column name as stored in the database.
	Name string
	// DataType is the raw SQL data type reported by the catalog.
	DataType string
	// Nullable reports whether the column accepts NULL values.
	Nullable bool
	// EnumType is the name of the PostgreSQL enumerated type backing the
	// column, or the empty string when the column is not an enum. When set, the
	// column is emitted with a Go named string type whose labels are listed in
	// EnumLabels.
	EnumType string
	// EnumLabels lists the enum's labels in their declared sort order. It is
	// non-empty only when EnumType is set.
	EnumLabels []string
}

// ForeignKey describes a single foreign key constraint discovered during
// introspection. The columns and referenced columns are aligned positionally.
type ForeignKey struct {
	// Name is the constraint name as stored in the catalog.
	Name string
	// Columns are the local columns that participate in the constraint, in
	// constraint definition order.
	Columns []string
	// RefTable is the unqualified name of the referenced table.
	RefTable string
	// RefColumns are the referenced columns, aligned with Columns.
	RefColumns []string
}

// TableSchema describes a single table together with its columns in their
// declared ordinal order and the key metadata discovered for it.
type TableSchema struct {
	// Name is the unqualified table or view name.
	Name string
	// IsView reports whether the relation is a view rather than a base table.
	// Views carry no primary key, unique, or foreign key metadata.
	IsView bool
	// Columns are the columns in ordinal position order.
	Columns []Column
	// PrimaryKey lists the primary key column names in key order. It is empty
	// for views and for tables without a primary key.
	PrimaryKey []string
	// Uniques lists each unique constraint as the ordered set of its column
	// names. It is empty for views.
	Uniques [][]string
	// ForeignKeys lists the foreign key constraints discovered for the table,
	// ordered by constraint name. It is empty for views.
	ForeignKeys []ForeignKey
}

// relationsQuery selects every base table and view in a schema together with a
// flag distinguishing the two. Ordering by name keeps the output deterministic.
const relationsQuery = `SELECT table_name, table_type
FROM information_schema.tables
WHERE table_schema = $1 AND table_type IN ('BASE TABLE', 'VIEW')
ORDER BY table_name`

// columnsQuery selects the columns of every relation in a schema. The udt_name
// is projected alongside data_type so that enum columns (reported as the generic
// "USER-DEFINED" data type) can be resolved to their underlying type name.
const columnsQuery = `SELECT table_name, column_name, data_type, is_nullable, udt_name
FROM information_schema.columns
WHERE table_schema = $1
ORDER BY table_name, ordinal_position`

// enumLabelsQuery selects every enum label in a schema joined to its type name.
// Labels are ordered by their catalog sort order so the emitted constants follow
// the declared order of the enum.
const enumLabelsQuery = `SELECT t.typname, e.enumlabel
FROM pg_type t
JOIN pg_enum e ON e.enumtypid = t.oid
JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname = $1
ORDER BY t.typname, e.enumsortorder`

// primaryKeyQuery selects the primary key columns of every base table in a
// schema, ordered by the column position within the key.
const primaryKeyQuery = `SELECT tc.table_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_name = tc.constraint_name
 AND kcu.constraint_schema = tc.constraint_schema
WHERE tc.constraint_schema = $1 AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY tc.table_name, kcu.ordinal_position`

// uniqueQuery selects the unique constraint columns of every base table in a
// schema, ordered by constraint name and the column position within the
// constraint so that multi-column constraints retain their declared order.
const uniqueQuery = `SELECT tc.table_name, tc.constraint_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_name = tc.constraint_name
 AND kcu.constraint_schema = tc.constraint_schema
WHERE tc.constraint_schema = $1 AND tc.constraint_type = 'UNIQUE'
ORDER BY tc.table_name, tc.constraint_name, kcu.ordinal_position`

// foreignKeyQuery selects every foreign key constraint in a schema together with
// its local columns, referenced table, and referenced columns. It reads the
// PostgreSQL catalog directly because information_schema does not reliably expose
// the position alignment between a key's local and referenced columns. The
// constraint's conkey and confkey arrays are unnested in parallel by ordinal
// position, which keeps each local column aligned with its referenced column and
// preserves the declared column order.
const foreignKeyQuery = `SELECT
    src.relname  AS table_name,
    con.conname  AS constraint_name,
    src_att.attname AS column_name,
    ref.relname  AS ref_table,
    ref_att.attname AS ref_column
FROM pg_constraint con
JOIN pg_class src ON src.oid = con.conrelid
JOIN pg_class ref ON ref.oid = con.confrelid
JOIN pg_namespace ns ON ns.oid = con.connamespace
JOIN LATERAL unnest(con.conkey, con.confkey)
        WITH ORDINALITY AS cols(src_attnum, ref_attnum, ord) ON TRUE
JOIN pg_attribute src_att
  ON src_att.attrelid = con.conrelid AND src_att.attnum = cols.src_attnum
JOIN pg_attribute ref_att
  ON ref_att.attrelid = con.confrelid AND ref_att.attnum = cols.ref_attnum
WHERE ns.nspname = $1 AND con.contype = 'f'
ORDER BY src.relname, con.conname, cols.ord`

// Introspect reads the relation, column, key, and enum metadata for the given
// schema from the standard catalogs and groups it into one TableSchema per
// relation. Base tables carry their primary key, unique, and foreign key
// metadata; views carry only their columns. The returned slice is ordered by
// relation name and every nested collection is in a deterministic order.
func Introspect(ctx context.Context, db *sql.DB, schema string) ([]TableSchema, error) {
	relations, order, err := introspectRelations(ctx, db, schema)
	if err != nil {
		return nil, err
	}

	enums, err := introspectEnums(ctx, db, schema)
	if err != nil {
		return nil, err
	}

	if err := introspectColumns(ctx, db, schema, relations, &order, enums); err != nil {
		return nil, err
	}
	if err := introspectPrimaryKeys(ctx, db, schema, relations); err != nil {
		return nil, err
	}
	if err := introspectUniques(ctx, db, schema, relations); err != nil {
		return nil, err
	}
	if err := introspectForeignKeys(ctx, db, schema, relations); err != nil {
		return nil, err
	}

	tables := make([]TableSchema, 0, len(order))
	for _, name := range order {
		tables = append(tables, *relations[name])
	}
	return tables, nil
}

// introspectRelations reads the base tables and views in the schema, returning a
// lookup keyed by relation name and the deterministic order of names.
func introspectRelations(ctx context.Context, db *sql.DB, schema string) (map[string]*TableSchema, []string, error) {
	rows, err := db.QueryContext(ctx, relationsQuery, schema)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	relations := make(map[string]*TableSchema)
	var order []string
	for rows.Next() {
		var name, tableType string
		if err := rows.Scan(&name, &tableType); err != nil {
			return nil, nil, err
		}
		relations[name] = &TableSchema{Name: name, IsView: tableType == "VIEW"}
		order = append(order, name)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return relations, order, nil
}

// introspectEnums reads the enum types and their labels in the schema, returning
// a lookup from enum type name to its labels in declared order.
func introspectEnums(ctx context.Context, db *sql.DB, schema string) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, enumLabelsQuery, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	enums := make(map[string][]string)
	for rows.Next() {
		var typeName, label string
		if err := rows.Scan(&typeName, &label); err != nil {
			return nil, err
		}
		enums[typeName] = append(enums[typeName], label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return enums, nil
}

// introspectColumns reads the columns of every relation, attaching enum metadata
// when a column's data type is the generic USER-DEFINED placeholder that resolves
// to a known enum. Columns belonging to relations not present in the relations
// lookup (for example, columns of relations excluded by the relations query) are
// ignored.
func introspectColumns(ctx context.Context, db *sql.DB, schema string, relations map[string]*TableSchema, order *[]string, enums map[string][]string) error {
	rows, err := db.QueryContext(ctx, columnsQuery, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, columnName, dataType, isNullable, udtName string
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &udtName); err != nil {
			return err
		}

		relation, ok := relations[tableName]
		if !ok {
			// The relation was not reported by the relations query (for example
			// a relation type the generator does not handle). Record it as a
			// base table so its columns are still emitted, preserving the prior
			// behavior of emitting accessors for every table that has columns.
			relation = &TableSchema{Name: tableName}
			relations[tableName] = relation
			*order = append(*order, tableName)
			sort.Strings(*order)
		}

		column := Column{
			Name:     columnName,
			DataType: dataType,
			Nullable: isNullable == "YES",
		}
		// A USER-DEFINED data type whose udt_name names a known enum is emitted
		// as a Go named string type with its labels as constants.
		if dataType == "USER-DEFINED" {
			if labels, isEnum := enums[udtName]; isEnum {
				column.EnumType = udtName
				column.EnumLabels = labels
			}
		}
		relation.Columns = append(relation.Columns, column)
	}
	return rows.Err()
}

// introspectPrimaryKeys reads the primary key columns of every base table and
// records them on the corresponding relation in key order.
func introspectPrimaryKeys(ctx context.Context, db *sql.DB, schema string, relations map[string]*TableSchema) error {
	rows, err := db.QueryContext(ctx, primaryKeyQuery, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			return err
		}
		if relation, ok := relations[tableName]; ok {
			relation.PrimaryKey = append(relation.PrimaryKey, columnName)
		}
	}
	return rows.Err()
}

// introspectUniques reads the unique constraints of every base table and records
// each as an ordered list of column names. Constraints are appended in catalog
// name order, which the unique query enforces.
func introspectUniques(ctx context.Context, db *sql.DB, schema string, relations map[string]*TableSchema) error {
	rows, err := db.QueryContext(ctx, uniqueQuery, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Track the position of each constraint within its table so columns of the
	// same constraint are grouped while constraint order follows the query.
	type uniqueKey struct {
		table      string
		constraint string
	}
	positions := make(map[uniqueKey]int)

	for rows.Next() {
		var tableName, constraintName, columnName string
		if err := rows.Scan(&tableName, &constraintName, &columnName); err != nil {
			return err
		}
		relation, ok := relations[tableName]
		if !ok {
			continue
		}
		key := uniqueKey{table: tableName, constraint: constraintName}
		pos, seen := positions[key]
		if !seen {
			pos = len(relation.Uniques)
			positions[key] = pos
			relation.Uniques = append(relation.Uniques, nil)
		}
		relation.Uniques[pos] = append(relation.Uniques[pos], columnName)
	}
	return rows.Err()
}

// introspectForeignKeys reads the foreign key constraints of every base table
// and records each as a ForeignKey with aligned local and referenced columns.
// Constraints are appended in catalog name order, which the foreign key query
// enforces.
func introspectForeignKeys(ctx context.Context, db *sql.DB, schema string, relations map[string]*TableSchema) error {
	rows, err := db.QueryContext(ctx, foreignKeyQuery, schema)
	if err != nil {
		return err
	}
	defer rows.Close()

	type fkKey struct {
		table      string
		constraint string
	}
	positions := make(map[fkKey]int)

	for rows.Next() {
		var tableName, constraintName, columnName, refTable, refColumn string
		if err := rows.Scan(&tableName, &constraintName, &columnName, &refTable, &refColumn); err != nil {
			return err
		}
		relation, ok := relations[tableName]
		if !ok {
			continue
		}
		key := fkKey{table: tableName, constraint: constraintName}
		pos, seen := positions[key]
		if !seen {
			pos = len(relation.ForeignKeys)
			positions[key] = pos
			relation.ForeignKeys = append(relation.ForeignKeys, ForeignKey{
				Name:     constraintName,
				RefTable: refTable,
			})
		}
		fk := &relation.ForeignKeys[pos]
		fk.Columns = append(fk.Columns, columnName)
		fk.RefColumns = append(fk.RefColumns, refColumn)
	}
	return rows.Err()
}
