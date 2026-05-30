// Command gooq-gen generates typed table accessors for the jooq query builder by
// introspecting a live database schema through the standard information_schema
// catalog. For each table it writes a "<table>.gen.go" file containing an
// embedded gooq.TableImpl, one typed Field per column, an As method for
// aliasing, and a package-level accessor variable.
//
// The command is a thin wrapper around the github.com/cgardev/gooq/codegen
// package. The jooq library itself imports no database driver. To run this
// command, the caller builds it with their driver blank-imported, for example:
//
//	import _ "github.com/lib/pq"              // for the "postgres" driver
//	import _ "github.com/go-sql-driver/mysql" // for the "mysql" driver
//
// and then invokes the resulting binary with the appropriate flags.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"time"

	"github.com/cgardev/gooq/codegen"
)

func main() {
	driver := flag.String("driver", "postgres", "database/sql driver name (must be blank-imported when building this command)")
	dsn := flag.String("dsn", "", "data source name (connection string); required")
	schema := flag.String("schema", "public", "database schema to introspect")
	outDir := flag.String("o", "internal/db", "output directory for the generated files")
	pkgName := flag.String("package", "db", "package name for the generated files")
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

	written, err := codegen.Generate(ctx, db, codegen.Options{
		Schema:  *schema,
		OutDir:  *outDir,
		Package: *pkgName,
	})
	if err != nil {
		log.Fatalf("gooq-gen: %v", err)
	}

	for _, path := range written {
		log.Printf("generated %s", path)
	}
}
