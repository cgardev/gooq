package codegen

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
)

func TestIntrospectGroupsColumnsByTable(t *testing.T) {
	activeCatalog = fakeCatalog{
		relations: [][]driver.Value{
			{"author", "BASE TABLE"},
			{"book", "BASE TABLE"},
		},
		columns: [][]driver.Value{
			{"author", "id", "integer", "NO", "int4"},
			{"author", "name", "text", "YES", "text"},
			{"book", "id", "integer", "NO", "int4"},
			{"book", "title", "text", "NO", "text"},
			{"book", "price", "numeric", "YES", "numeric"},
		},
	}

	db, err := sql.Open("gooqgenfake", "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	tables, err := Introspect(context.Background(), db, "public")
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(tables))
	}

	// Grouping and table order are preserved from the relation order.
	if tables[0].Name != "author" || tables[1].Name != "book" {
		t.Fatalf("table names = %q, %q", tables[0].Name, tables[1].Name)
	}

	// Author columns: order and nullability.
	author := tables[0]
	if len(author.Columns) != 2 {
		t.Fatalf("author columns = %d, want 2", len(author.Columns))
	}
	if author.Columns[0].Name != "id" || author.Columns[0].Nullable {
		t.Errorf("author.id = %+v, want non-nullable", author.Columns[0])
	}
	if author.Columns[1].Name != "name" || !author.Columns[1].Nullable {
		t.Errorf("author.name = %+v, want nullable", author.Columns[1])
	}

	// Book columns: order and the raw data type are carried through verbatim.
	book := tables[1]
	wantBook := []struct {
		name     string
		dataType string
		nullable bool
	}{
		{"id", "integer", false},
		{"title", "text", false},
		{"price", "numeric", true},
	}
	if len(book.Columns) != len(wantBook) {
		t.Fatalf("book columns = %d, want %d", len(book.Columns), len(wantBook))
	}
	for i, w := range wantBook {
		got := book.Columns[i]
		if got.Name != w.name || got.DataType != w.dataType || got.Nullable != w.nullable {
			t.Errorf("book column %d = %+v, want %v", i, got, w)
		}
	}
}

// TestIntrospectKeyMetadata verifies that primary key, unique, and foreign key
// metadata are grouped onto the correct tables in a deterministic order, that
// enum columns resolve their labels, and that views carry only columns.
func TestIntrospectKeyMetadata(t *testing.T) {
	activeCatalog = fakeCatalog{
		relations: [][]driver.Value{
			{"author", "BASE TABLE"},
			{"book", "BASE TABLE"},
			{"book_summary", "VIEW"},
		},
		columns: [][]driver.Value{
			{"author", "id", "integer", "NO", "int4"},
			{"author", "email", "text", "NO", "text"},
			{"book", "id", "integer", "NO", "int4"},
			{"book", "author_id", "integer", "NO", "int4"},
			{"book", "status", "USER-DEFINED", "NO", "book_status"},
			{"book_summary", "title", "text", "YES", "text"},
		},
		enums: [][]driver.Value{
			{"book_status", "draft"},
			{"book_status", "published"},
			{"book_status", "archived"},
		},
		primaryKeys: [][]driver.Value{
			{"author", "id"},
			{"book", "id"},
		},
		uniques: [][]driver.Value{
			{"author", "author_email_key", "email"},
		},
		foreignKeys: [][]driver.Value{
			{"book", "book_author_id_fkey", "author_id", "author", "id"},
		},
	}

	db, err := sql.Open("gooqgenfake", "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	tables, err := Introspect(context.Background(), db, "public")
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	if len(tables) != 3 {
		t.Fatalf("got %d relations, want 3", len(tables))
	}

	byName := make(map[string]TableSchema, len(tables))
	for _, table := range tables {
		byName[table.Name] = table
	}

	author := byName["author"]
	if len(author.PrimaryKey) != 1 || author.PrimaryKey[0] != "id" {
		t.Errorf("author primary key = %v, want [id]", author.PrimaryKey)
	}
	if len(author.Uniques) != 1 || len(author.Uniques[0]) != 1 || author.Uniques[0][0] != "email" {
		t.Errorf("author uniques = %v, want [[email]]", author.Uniques)
	}

	book := byName["book"]
	if len(book.ForeignKeys) != 1 {
		t.Fatalf("book foreign keys = %v, want one", book.ForeignKeys)
	}
	fk := book.ForeignKeys[0]
	if fk.Name != "book_author_id_fkey" || fk.RefTable != "author" {
		t.Errorf("book foreign key = %+v", fk)
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "author_id" {
		t.Errorf("book foreign key columns = %v, want [author_id]", fk.Columns)
	}
	if len(fk.RefColumns) != 1 || fk.RefColumns[0] != "id" {
		t.Errorf("book foreign key ref columns = %v, want [id]", fk.RefColumns)
	}

	// The enum column resolves its type and labels.
	var status Column
	for _, c := range book.Columns {
		if c.Name == "status" {
			status = c
		}
	}
	if status.EnumType != "book_status" {
		t.Errorf("status enum type = %q, want book_status", status.EnumType)
	}
	wantLabels := []string{"draft", "published", "archived"}
	if len(status.EnumLabels) != len(wantLabels) {
		t.Fatalf("status enum labels = %v, want %v", status.EnumLabels, wantLabels)
	}
	for i, want := range wantLabels {
		if status.EnumLabels[i] != want {
			t.Errorf("status enum label %d = %q, want %q", i, status.EnumLabels[i], want)
		}
	}

	// The view carries columns but no key metadata.
	view := byName["book_summary"]
	if !view.IsView {
		t.Errorf("book_summary IsView = false, want true")
	}
	if len(view.PrimaryKey) != 0 || len(view.Uniques) != 0 || len(view.ForeignKeys) != 0 {
		t.Errorf("view %q unexpectedly carries key metadata", view.Name)
	}
	if len(view.Columns) != 1 || view.Columns[0].Name != "title" {
		t.Errorf("view columns = %v, want [title]", view.Columns)
	}
}

func TestMapSQLType(t *testing.T) {
	tests := []struct {
		dataType    string
		nullable    bool
		fieldType   string
		constructor string
		imports     []string
	}{
		// Non-nullable mappings preserve the refined field types.
		{"integer", false, "gooq.NumericField[int64]", "gooq.NewNumericField[int64]", nil},
		{"bigint", false, "gooq.NumericField[int64]", "gooq.NewNumericField[int64]", nil},
		{"int unsigned", false, "gooq.NumericField[int64]", "gooq.NewNumericField[int64]", nil},
		{"numeric(10,2)", false, "gooq.NumericField[float64]", "gooq.NewNumericField[float64]", nil},
		{"double precision", false, "gooq.NumericField[float64]", "gooq.NewNumericField[float64]", nil},
		{"boolean", false, "gooq.Field[bool]", "gooq.NewField[bool]", nil},
		{"VARCHAR(255)", false, "gooq.StringField", "gooq.NewStringField", nil},
		{"uuid", false, "gooq.StringField", "gooq.NewStringField", nil},
		{"timestamp with time zone", false, "gooq.Field[time.Time]", "gooq.NewField[time.Time]", []string{"time"}},
		{"date", false, "gooq.Field[time.Time]", "gooq.NewField[time.Time]", []string{"time"}},
		{"bytea", false, "gooq.Field[[]byte]", "gooq.NewField[[]byte]", nil},
		{"jsonb", false, "gooq.Field[json.RawMessage]", "gooq.NewField[json.RawMessage]", []string{"encoding/json"}},
		{"some_unknown_type", false, "gooq.StringField", "gooq.NewStringField", nil},

		// Nullable scalar mappings wrap the element type in sql.Null and import
		// "database/sql".
		{"text", true, "gooq.Field[sql.Null[string]]", "gooq.NewField[sql.Null[string]]", []string{"database/sql"}},
		{"bigint", true, "gooq.Field[sql.Null[int64]]", "gooq.NewField[sql.Null[int64]]", []string{"database/sql"}},
		{"double precision", true, "gooq.Field[sql.Null[float64]]", "gooq.NewField[sql.Null[float64]]", []string{"database/sql"}},
		{"boolean", true, "gooq.Field[sql.Null[bool]]", "gooq.NewField[sql.Null[bool]]", []string{"database/sql"}},
		{"date", true, "gooq.Field[sql.Null[time.Time]]", "gooq.NewField[sql.Null[time.Time]]", []string{"database/sql", "time"}},

		// A byte slice already scans NULL as nil, so it is never wrapped. A
		// nullable json/jsonb column maps to a plain []byte (not json.RawMessage)
		// because database/sql cannot scan a NULL into a named slice type.
		{"bytea", true, "gooq.Field[[]byte]", "gooq.NewField[[]byte]", nil},
		{"jsonb", true, "gooq.Field[[]byte]", "gooq.NewField[[]byte]", nil},
		{"json", true, "gooq.Field[[]byte]", "gooq.NewField[[]byte]", nil},
	}
	for _, tc := range tests {
		got := mapColumn(Column{DataType: tc.dataType, Nullable: tc.nullable}, nil)
		if got.fieldType != tc.fieldType {
			t.Errorf("mapColumn(%q, %t) fieldType = %q, want %q", tc.dataType, tc.nullable, got.fieldType, tc.fieldType)
		}
		if got.constructor != tc.constructor {
			t.Errorf("mapColumn(%q, %t) constructor = %q, want %q", tc.dataType, tc.nullable, got.constructor, tc.constructor)
		}
		if len(got.imports) != len(tc.imports) {
			t.Fatalf("mapColumn(%q, %t) imports = %v, want %v", tc.dataType, tc.nullable, got.imports, tc.imports)
		}
		for i := range tc.imports {
			if got.imports[i] != tc.imports[i] {
				t.Errorf("mapColumn(%q, %t) import %d = %q, want %q", tc.dataType, tc.nullable, i, got.imports[i], tc.imports[i])
			}
		}
	}
}

func TestNameConversion(t *testing.T) {
	if got := camel("author_id"); got != "AuthorId" {
		t.Errorf("camel(author_id) = %q, want AuthorId", got)
	}
	if got := camel("a__b"); got != "AB" {
		t.Errorf("camel(a__b) = %q, want AB (empty segments skipped)", got)
	}
	if got := lowerCamel("book_table"); got != "bookTable" {
		t.Errorf("lowerCamel(book_table) = %q, want bookTable", got)
	}
	if got := structName("book"); got != "bookTable" {
		t.Errorf("structName(book) = %q, want bookTable", got)
	}
	if got := exportName("book"); got != "Book" {
		t.Errorf("exportName(book) = %q, want Book", got)
	}
	if got := fieldName("author_id"); got != "AuthorId" {
		t.Errorf("fieldName(author_id) = %q, want AuthorId", got)
	}

	// A column starting with a digit must yield a valid identifier.
	if got := fieldName("2fa"); !validIdent(got) {
		t.Errorf("fieldName(2fa) = %q is not a valid Go identifier", got)
	}

	// A column matching a Go keyword must be rewritten to a non-keyword.
	got := fieldName("type")
	if goKeywords[got] {
		t.Errorf("fieldName(type) = %q is still a keyword", got)
	}
	if !validIdent(got) {
		t.Errorf("fieldName(type) = %q is not a valid Go identifier", got)
	}

	// A name that becomes empty after stripping invalid characters falls back.
	if got := fieldName("***"); !validIdent(got) {
		t.Errorf("fieldName(***) = %q is not a valid Go identifier", got)
	}
}

// validIdent reports whether s is a syntactically valid Go identifier: it is
// non-empty, starts with a letter or underscore, and contains only letters,
// digits, and underscores.
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
