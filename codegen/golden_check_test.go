package codegen

import (
	"os"
	"testing"
)

// TestGoldenFilesMatch confirms the emitter reproduces the checked-in example
// golden files byte-for-byte for the book and author tables. This guards the
// non-nullable rendering path against accidental changes.
func TestGoldenFilesMatch(t *testing.T) {
	cases := []struct {
		path  string
		table TableSchema
	}{
		{
			path: "../example/internal/db/book.gen.go",
			table: TableSchema{Name: "book", Columns: []Column{
				{Name: "id", DataType: "integer"},
				{Name: "title", DataType: "text"},
				{Name: "price", DataType: "numeric"},
				{Name: "author_id", DataType: "integer"},
			}},
		},
		{
			path: "../example/internal/db/author.gen.go",
			table: TableSchema{Name: "author", Columns: []Column{
				{Name: "id", DataType: "integer"},
				{Name: "name", DataType: "text"},
			}},
		},
	}
	for _, c := range cases {
		want, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read golden %s: %v", c.path, err)
		}
		got, err := EmitTable("db", c.table)
		if err != nil {
			t.Fatalf("EmitTable %s: %v", c.table.Name, err)
		}
		if got != string(want) {
			t.Errorf("emitted %s differs from golden:\n--- got ---\n%s\n--- want ---\n%s", c.table.Name, got, want)
		}
	}
}
