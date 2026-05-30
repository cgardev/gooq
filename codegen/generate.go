package codegen

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Options configures a code generation run.
type Options struct {
	// Schema is the database schema to introspect. It defaults to "public"
	// when empty.
	Schema string
	// OutDir is the directory into which the generated files are written. It
	// is created if it does not already exist.
	OutDir string
	// Package is the package name declared in the generated files. It defaults
	// to "db" when empty.
	Package string
}

// Generate introspects the provided database, emits typed accessors for every
// discovered table, and writes them as "<table>.gen.go" files into the
// configured output directory. It returns the paths of the written files in
// table order.
//
// Generate returns an error when introspection discovers no tables, ensuring
// that an empty or unreachable schema is reported rather than silently
// producing no output.
func Generate(ctx context.Context, db *sql.DB, opts Options) ([]string, error) {
	schema := opts.Schema
	if schema == "" {
		schema = "public"
	}
	pkg := opts.Package
	if pkg == "" {
		pkg = "db"
	}

	tables, err := Introspect(ctx, db, schema)
	if err != nil {
		return nil, fmt.Errorf("introspecting schema %q: %w", schema, err)
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("no tables found in schema %q", schema)
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	written := make([]string, 0, len(tables))
	for _, table := range tables {
		source, err := EmitTable(pkg, table)
		if err != nil {
			return nil, err
		}

		path := filepath.Join(opts.OutDir, table.Name+".gen.go")
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", path, err)
		}
		written = append(written, path)
	}

	return written, nil
}
