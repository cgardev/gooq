// Command gooq-gen generates typed table accessors for the jooq query builder by
// introspecting a live database schema through the standard information_schema
// catalog. For each table it writes a "<table>.gen.go" file containing an
// embedded gooq.TableImpl, one typed Field per column, an As method for
// aliasing, key metadata accessors, and a package-level accessor variable.
//
// The command is a thin wrapper around the github.com/cgardev/gooq/codegen
// package. The jooq library itself imports no database driver. To run this
// command, the caller builds it with their driver blank-imported, for example:
//
//	import _ "github.com/jackc/pgx/v5/stdlib" // for the "postgres" driver
//	import _ "modernc.org/sqlite"             // for the "sqlite" driver
//
// and then invokes the resulting binary with the appropriate flags.
//
// The repeatable -type flag overrides the Go type generated for a column or SQL
// type, for example:
//
//	gooq-gen -type 'public.book.id=github.com/google/uuid.UUID' \
//	         -type 'uuid=github.com/google/uuid.UUID'
//
// The left-hand side is either a fully qualified "schema.table.column" key or a
// SQL type name; the right-hand side is a fully qualified Go type whose package
// import path and type name are split at the final dot.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cgardev/gooq/codegen"
)

// typeOverrideFlag collects repeated -type flags into a map of override keys to
// the resolved Go type and its import path.
type typeOverrideFlag map[string]codegen.TypeOverride

// String implements flag.Value. It renders the collected overrides for the flag
// package's usage output.
func (f typeOverrideFlag) String() string {
	if len(f) == 0 {
		return ""
	}
	parts := make([]string, 0, len(f))
	for key, ov := range f {
		parts = append(parts, key+"="+qualified(ov))
	}
	return strings.Join(parts, ",")
}

// Set implements flag.Value. It parses a single "key=pkgpath.TypeName" entry,
// splitting the value at the final dot into an import path and a type expression
// qualified by the package's base name.
func (f typeOverrideFlag) Set(value string) error {
	key, spec, ok := strings.Cut(value, "=")
	if !ok || key == "" || spec == "" {
		return fmt.Errorf("invalid -type %q: want key=pkgpath.TypeName", value)
	}

	override, err := parseTypeSpec(spec)
	if err != nil {
		return fmt.Errorf("invalid -type %q: %w", value, err)
	}
	f[key] = override
	return nil
}

// parseTypeSpec splits a fully qualified Go type expression into a TypeOverride.
// A specification without a slash is treated as a builtin or already-imported
// type and produces no import. Otherwise the final path segment is split at its
// last dot into the import path and the package-qualified type expression, for
// example "github.com/google/uuid.UUID" becomes import "github.com/google/uuid"
// and type "uuid.UUID".
func parseTypeSpec(spec string) (codegen.TypeOverride, error) {
	if !strings.Contains(spec, "/") {
		// A bare type such as "string" or "time.Time" needs no extra import.
		return codegen.TypeOverride{GoType: spec}, nil
	}

	slash := strings.LastIndex(spec, "/")
	dir := spec[:slash]
	last := spec[slash+1:]

	dot := strings.LastIndex(last, ".")
	if dot <= 0 || dot == len(last)-1 {
		return codegen.TypeOverride{}, fmt.Errorf("want pkgpath.TypeName, missing type name in %q", spec)
	}
	pkgBase := last[:dot]
	typeName := last[dot+1:]

	importPath := dir + "/" + pkgBase
	return codegen.TypeOverride{
		GoType: pkgBase + "." + typeName,
		Import: importPath,
	}, nil
}

// qualified reconstructs the fully qualified Go type expression for an override,
// used when rendering the flag's current value.
func qualified(o codegen.TypeOverride) string {
	if o.Import == "" {
		return o.GoType
	}
	dot := strings.LastIndex(o.GoType, ".")
	if dot < 0 {
		return o.Import + "." + o.GoType
	}
	return o.Import + o.GoType[dot:]
}

func main() {
	driver := flag.String("driver", "postgres", "database/sql driver name (must be blank-imported when building this command)")
	dsn := flag.String("dsn", "", "data source name (connection string); required")
	schema := flag.String("schema", "public", "database schema to introspect")
	outDir := flag.String("o", "internal/db", "output directory for the generated files")
	pkgName := flag.String("package", "db", "package name for the generated files")

	overrides := typeOverrideFlag{}
	flag.Var(overrides, "type", "override a column or SQL type, repeatable; for example public.book.id=github.com/google/uuid.UUID")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("gooq-gen: -dsn is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open(*driver, *dsn)
	if err != nil {
		log.Fatalf("gooq-gen: opening database: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("gooq-gen: connecting to database: %v", err)
	}

	options := codegen.Options{
		Schema:  *schema,
		OutDir:  *outDir,
		Package: *pkgName,
	}
	if len(overrides) > 0 {
		options.TypeOverrides = map[string]codegen.TypeOverride(overrides)
	}

	written, err := codegen.Generate(ctx, db, options)
	if err != nil {
		log.Fatalf("gooq-gen: %v", err)
	}

	for _, path := range written {
		log.Printf("generated %s", path)
	}
}
