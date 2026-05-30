// Command gendb regenerates the typed table accessors used by the integration
// tests. It starts a disposable PostgreSQL container, applies the authoritative
// schema from testdata/schema.sql, and runs the jooq code generator against the
// live database. The generated files are written to internal/db.
//
// Run it from the integration module root with:
//
//	go run ./internal/gendb
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	// The PostgreSQL driver is blank-imported so that database/sql can open a
	// "pgx" connection. The core codegen package never imports a driver.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cgardev/gooq/codegen"
)

func main() {
	schemaPath := flag.String("schema", defaultSchemaPath(), "path to the schema definition file")
	outDir := flag.String("o", defaultOutDir(), "output directory for generated files")
	flag.Parse()

	if err := run(*schemaPath, *outDir); err != nil {
		log.Fatalf("gendb: %v", err)
	}
}

// run starts a PostgreSQL container, applies the schema, and generates the
// typed accessors into the output directory.
func run(schemaPath, outDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx,
		postgresImage(),
		postgres.WithDatabase("integration"),
		postgres.WithUsername("integration"),
		postgres.WithPassword("integration"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = testcontainers.TerminateContainer(container)
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		return err
	}

	written, err := codegen.Generate(ctx, db, codegen.Options{
		Schema:  "public",
		OutDir:  outDir,
		Package: "db",
	})
	if err != nil {
		return err
	}

	for _, path := range written {
		log.Printf("generated %s", path)
	}
	return nil
}

// moduleRoot returns the integration module root resolved relative to this
// source file, allowing the command to run from any working directory.
func moduleRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// file is <root>/internal/gendb/main.go; the module root is three levels up.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// defaultSchemaPath returns the default location of the schema definition file.
func defaultSchemaPath() string {
	return filepath.Join(moduleRoot(), "testdata", "schema.sql")
}

// defaultOutDir returns the default output directory for generated files.
func defaultOutDir() string {
	return filepath.Join(moduleRoot(), "internal", "db")
}

// postgresImage returns the PostgreSQL container image used to regenerate the
// accessors. It honors GOOQ_PG_IMAGE and otherwise defaults to the latest
// supported major, keeping the generator aligned with the supported versions
// (PostgreSQL 18 and 17).
func postgresImage() string {
	if image := os.Getenv("GOOQ_PG_IMAGE"); image != "" {
		return image
	}
	return "postgres:18-alpine"
}
