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
	// TypeOverrides maps a column or SQL type onto a specific Go type. Keys are
	// matched most specific first: a fully qualified "schema.table.column" key
	// takes precedence over a normalized SQL type key (for example "uuid").
	// When no key matches a column, the default type mapping is used.
	TypeOverrides map[string]TypeOverride
}

// enumsFileName is the name of the shared file holding the generated enum type
// definitions. It is written only when at least one enum column is discovered.
const enumsFileName = "enums.gen.go"

// Generate introspects the provided database, emits typed accessors for every
// discovered table and view, and writes them as "<table>.gen.go" files into the
// configured output directory. When enum columns are present it additionally
// writes a shared "enums.gen.go" file holding the generated enum types. It
// returns the paths of the written files in deterministic order.
//
// Generate returns an error when introspection discovers no relations, ensuring
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

	var written []string

	enums, err := emitEnums(pkg, collectEnums(tables))
	if err != nil {
		return nil, err
	}
	if enums != "" {
		path := filepath.Join(opts.OutDir, enumsFileName)
		if err := os.WriteFile(path, []byte(enums), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", path, err)
		}
		written = append(written, path)
	}

	for _, table := range tables {
		source, err := emitTable(pkg, schema, table, opts.TypeOverrides)
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
