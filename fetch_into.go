package gooq

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
)

// statement is the minimal rendering surface the free generic result-mapping
// helpers require. Every typed and untyped statement builder satisfies it
// through its SQL method, so the helpers accept SELECT queries as well as DML
// statements that render a RETURNING clause.
type statement interface {
	SQL() (string, []any, error)
}

// FetchInto runs the supplied statement and maps every result row into a new
// value of the struct type S, returning the resulting slice.
//
// Columns are matched to struct fields by the `db` struct tag; a field tagged
// `db:"col"` receives the value of the result column named "col". When a field
// has no db tag, its Go field name is matched case-insensitively against the
// column name. Result columns without a matching exported struct field are
// skipped. Pointer fields and types implementing the standard library
// sql.Scanner interface are supported, as are values implementing
// driver.Valuer. The error returned by rows.Err is propagated.
func FetchInto[S any](ctx context.Context, db Querier, q statement) ([]S, error) {
	query, args, err := q.SQL()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mapper, err := newRowMapper[S](rows)
	if err != nil {
		return nil, err
	}

	var out []S
	for rows.Next() {
		row, err := mapper.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// FetchOneInto runs the statement and returns a single mapped row. It returns
// the zero value of S and a nil error when no row matches, the single row when
// exactly one matches, or ErrTooManyRows when more than one row matches.
func FetchOneInto[S any](ctx context.Context, db Querier, q statement) (S, error) {
	var zero S
	rows, err := FetchInto[S](ctx, db, q)
	if err != nil {
		return zero, err
	}
	switch len(rows) {
	case 0:
		return zero, nil
	case 1:
		return rows[0], nil
	default:
		return zero, ErrTooManyRows
	}
}

// FetchMap runs the statement, maps each row into a value of S, and indexes the
// results by the value of keyColumn cast to K. The keyColumn must name one of
// the columns selected by the statement. When two rows share a key, the later
// row overwrites the earlier one.
func FetchMap[K comparable, S any](ctx context.Context, db Querier, q statement, keyColumn string) (map[K]S, error) {
	out := map[K]S{}
	err := fetchKeyed[K, S](ctx, db, q, keyColumn, func(key K, row S) {
		out[key] = row
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FetchGroups runs the statement, maps each row into a value of S, and groups
// the results by the value of keyColumn cast to K. The keyColumn must name one
// of the columns selected by the statement. Rows that share a key are appended
// to the same slice in result order.
func FetchGroups[K comparable, S any](ctx context.Context, db Querier, q statement, keyColumn string) (map[K][]S, error) {
	out := map[K][]S{}
	err := fetchKeyed[K, S](ctx, db, q, keyColumn, func(key K, row S) {
		out[key] = append(out[key], row)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// fetchKeyed runs the statement and, for every row, maps it into a value of S
// and extracts the key column as K, invoking collect with the pair. It backs
// both FetchMap and FetchGroups.
func fetchKeyed[K comparable, S any](ctx context.Context, db Querier, q statement, keyColumn string, collect func(K, S)) error {
	query, args, err := q.SQL()
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	mapper, err := newRowMapper[S](rows)
	if err != nil {
		return err
	}
	keyIndex, err := mapper.columnIndex(keyColumn)
	if err != nil {
		return err
	}

	for rows.Next() {
		row, raw, err := mapper.scanWithRaw(rows)
		if err != nil {
			return err
		}
		key, err := convertKey[K](raw[keyIndex])
		if err != nil {
			return err
		}
		collect(key, row)
	}
	return rows.Err()
}

// rowMapper holds the precomputed plan for mapping a result set into the struct
// type S. The plan is derived once from the column list and reused for every
// row of the same result set.
type rowMapper[S any] struct {
	structType reflect.Type
	columns    []string
	// targets[i] is the struct field index path for result column i, or nil when
	// no struct field matches the column and it must be discarded.
	targets [][]int
}

// newRowMapper inspects the result set columns and the struct type S to build a
// reusable mapping plan. It returns an error when S is not a struct type.
func newRowMapper[S any](rows *sql.Rows) (*rowMapper[S], error) {
	var zero S
	structType := reflect.TypeOf(zero)
	if structType == nil || structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("jooq: FetchInto target %T is not a struct type", zero)
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	fieldByColumn := buildFieldIndex(structType)
	targets := make([][]int, len(columns))
	for i, column := range columns {
		if index, ok := fieldByColumn[strings.ToLower(column)]; ok {
			targets[i] = index
		}
	}

	return &rowMapper[S]{
		structType: structType,
		columns:    columns,
		targets:    targets,
	}, nil
}

// buildFieldIndex maps a lower-cased lookup key to the field index path for
// every exported field of the struct. A field tagged `db:"col"` is registered
// under "col"; fields without a tag are registered under their lower-cased Go
// field name. The db tag takes precedence so an explicit mapping is never
// shadowed by a field-name fallback.
func buildFieldIndex(structType reflect.Type) map[string][]int {
	tagged := map[string][]int{}
	byName := map[string][]int{}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" {
			// Unexported fields cannot be addressed for scanning.
			continue
		}
		tag, hasTag := field.Tag.Lookup("db")
		if hasTag {
			tag = strings.TrimSpace(strings.Split(tag, ",")[0])
		}
		if hasTag && tag == "-" {
			continue
		}
		if hasTag && tag != "" {
			tagged[strings.ToLower(tag)] = field.Index
			continue
		}
		byName[strings.ToLower(field.Name)] = field.Index
	}
	// Tagged entries win over field-name fallbacks for the same key.
	for key, index := range tagged {
		byName[key] = index
	}
	return byName
}

// columnIndex returns the result-set position of the named column, matched
// case-insensitively, or an error when the column is not part of the result.
func (m *rowMapper[S]) columnIndex(column string) (int, error) {
	target := strings.ToLower(column)
	for i, name := range m.columns {
		if strings.ToLower(name) == target {
			return i, nil
		}
	}
	return 0, fmt.Errorf("jooq: key column %q is not among the selected columns", column)
}

// scan maps the current row into a new value of S.
func (m *rowMapper[S]) scan(rows *sql.Rows) (S, error) {
	row, _, err := m.scanWithRaw(rows)
	return row, err
}

// scanWithRaw maps the current row into a new value of S and also returns the
// raw scan targets for every column. The raw slice lets keyed fetches read a
// key column even when it is also mapped into a struct field. Columns that map
// to a struct field are scanned directly into the field's address; unmapped
// columns are scanned into a discarded placeholder.
func (m *rowMapper[S]) scanWithRaw(rows *sql.Rows) (S, []any, error) {
	var result S
	structValue := reflect.ValueOf(&result).Elem()

	targets := make([]any, len(m.columns))
	raw := make([]any, len(m.columns))
	for i, index := range m.targets {
		if index == nil {
			placeholder := new(any)
			targets[i] = placeholder
			raw[i] = placeholder
			continue
		}
		fieldAddr := structValue.FieldByIndex(index).Addr().Interface()
		targets[i] = fieldAddr
		raw[i] = fieldAddr
	}

	if err := rows.Scan(targets...); err != nil {
		return result, nil, err
	}
	return result, raw, nil
}

// convertKey turns a scanned key value into the requested key type K. The
// value originates either from a discarded placeholder (*any) or from a struct
// field address. Direct assignability is attempted first, then a reflect-based
// conversion for compatible numeric and string kinds, which keeps integer key
// columns usable as int64, int, and the like.
func convertKey[K comparable](scanned any) (K, error) {
	var zero K
	value := dereference(scanned)

	if value == nil {
		return zero, nil
	}
	if typed, ok := value.(K); ok {
		return typed, nil
	}

	keyType := reflect.TypeOf(zero)
	source := reflect.ValueOf(value)
	if keyType != nil && source.Type().ConvertibleTo(keyType) {
		return source.Convert(keyType).Interface().(K), nil
	}
	return zero, fmt.Errorf("jooq: cannot use key value of type %T as key type %T", value, zero)
}

// dereference unwraps a scan target back to the underlying scanned value. A
// *any placeholder yields its element, any other pointer is followed once, and
// a driver.Valuer is resolved to its driver value so it can be compared.
func dereference(scanned any) any {
	switch v := scanned.(type) {
	case *any:
		return *v
	case nil:
		return nil
	}

	rv := reflect.ValueOf(scanned)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		elem := rv.Elem().Interface()
		if valuer, ok := elem.(driver.Valuer); ok {
			if dv, err := valuer.Value(); err == nil {
				return dv
			}
		}
		return elem
	}
	return scanned
}
