package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	// The PostgreSQL driver is blank-imported so that database/sql can open a
	// "pgx" connection for the whole test binary.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/integration/internal/db"
)

// Deterministic identifier constants for the fixtures. Using fixed UUID strings
// rather than generated values keeps every assertion stable and lets the tests
// read like prose ("the Go book", "the second author").
const (
	authorDonovan   = "a0000000-0000-0000-0000-000000000001"
	authorKernighan = "a0000000-0000-0000-0000-000000000002"

	bookGo       = "b0000000-0000-0000-0000-000000000001"
	bookPractice = "b0000000-0000-0000-0000-000000000002"
	bookC        = "b0000000-0000-0000-0000-000000000003"

	reviewGoFirst    = "c0000000-0000-0000-0000-000000000001"
	reviewGoSecond   = "c0000000-0000-0000-0000-000000000002"
	reviewPractice   = "c0000000-0000-0000-0000-000000000003"
	reviewCAnonymous = "c0000000-0000-0000-0000-000000000004"
)

// publishedGo is the exact publication instant of the Go book. It is kept as a
// package-level value so the seeding and the timestamp assertions compare
// against the very same instant.
var publishedGo = time.Date(2015, time.October, 26, 12, 0, 0, 0, time.UTC)

// sharedDB is the single PostgreSQL instance shared by the whole package. It is
// created once in TestMain so the suite starts only one container; each test
// then runs inside its own transaction and rolls it back, which keeps the tests
// isolated while remaining fast.
var sharedDB *sql.DB

// defaultPostgresImage is the container image used when GOOQ_PG_IMAGE is not set.
// The library supports the latest two PostgreSQL majors (18 and 17); the suite is
// run once per supported image.
const defaultPostgresImage = "postgres:18-alpine"

// postgresImage returns the PostgreSQL container image for this run. Continuous
// integration drives the version matrix by re-running the whole suite once per
// supported image, setting GOOQ_PG_IMAGE to "postgres:18-alpine" and then to
// "postgres:17-alpine". Only a single container is ever started per process; the
// matrix is external, so the shared-container plus per-test-transaction design is
// preserved.
func postgresImage() string {
	if image := os.Getenv("GOOQ_PG_IMAGE"); image != "" {
		return image
	}
	return defaultPostgresImage
}

// TestMain starts one PostgreSQL container for the package, applies the schema,
// runs the tests, and tears the container down afterwards. When Docker is
// unavailable the suite still runs so that every test skips with a clear reason.
func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	flag.Parse()
	if testing.Short() {
		return m.Run()
	}

	ctx := context.Background()

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
		log.Printf("integration: PostgreSQL unavailable, tests will skip: %v", err)
		return m.Run()
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			log.Printf("integration: terminating container: %v", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("integration: connection string: %v", err)
		return 1
	}

	sharedDB, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Printf("integration: open database: %v", err)
		return 1
	}
	defer sharedDB.Close()

	if err := applySchema(ctx, sharedDB); err != nil {
		log.Printf("integration: apply schema: %v", err)
		return 1
	}

	return m.Run()
}

// applySchema executes the data definition language held in the schema file.
func applySchema(ctx context.Context, conn *sql.DB) error {
	schema, err := os.ReadFile(schemaFilePath())
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, string(schema))
	return err
}

// schemaFilePath resolves the schema definition file relative to this source.
func schemaFilePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "schema.sql")
}

// requireDatabase skips the calling test when the shared database is not
// available, either because of -short or because Docker could not be reached.
func requireDatabase(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if sharedDB == nil {
		t.Skip("PostgreSQL is not available; is Docker running?")
	}
}

// transaction opens a transaction on the shared database and rolls it back when
// the test ends, so each test observes a clean, isolated database.
func transaction(t *testing.T) (context.Context, *sql.Tx) {
	t.Helper()
	requireDatabase(t)

	ctx := context.Background()
	tx, err := sharedDB.BeginTx(ctx, nil)
	noError(t, "begin transaction", err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return ctx, tx
}

// library returns a transaction already populated with the standard fixtures,
// which most tests build upon. The name reads naturally at the call site:
// "given a library, when I query it, then ...".
func library(t *testing.T) (context.Context, *sql.Tx) {
	t.Helper()
	ctx, tx := transaction(t)
	seed(ctx, t, tx)
	return ctx, tx
}

// seed inserts the canonical fixtures through the jooq insert builder. The data
// is deliberately varied so the edge-case tests have something meaningful to
// query: two authors (one with a JSONB metadata document, one without), three
// books with different prices, page counts, print status, JSONB attribute
// documents, and an optional editor, and four reviews with assorted ratings and
// an occasional NULL body.
//
// Several inserts deliberately omit the created_at column so the database's
// DEFAULT now() also gets exercised.
func seed(ctx context.Context, t *testing.T, conn gooq.Querier) {
	t.Helper()

	_, err := gooq.InsertInto(db.Author).
		Columns(db.Author.Id, db.Author.Name, db.Author.Email, db.Author.Metadata).
		Values(authorDonovan, "Alan Donovan", "donovan@example.com",
			doc(t, map[string]any{"country": "US", "awards": []any{"Hugo"}})).
		Values(authorKernighan, "Brian Kernighan", "kernighan@example.com", noDoc()).
		Execute(ctx, conn)
	noError(t, "seed authors", err)

	_, err = gooq.InsertInto(db.Book).
		Columns(
			db.Book.Id, db.Book.AuthorId, db.Book.EditorId, db.Book.Title,
			db.Book.Subtitle, db.Book.Price, db.Book.PageCount, db.Book.InPrint,
			db.Book.Attributes, db.Book.PublishedAt,
		).
		Values(bookGo, authorDonovan, text(authorKernighan), "The Go Programming Language",
			text("An Idiomatic Guide"), 39.99, int64(380), true,
			doc(t, map[string]any{"format": "paperback", "languages": []any{"en", "es"}}),
			moment(publishedGo)).
		Values(bookPractice, authorKernighan, noText(), "The Practice of Programming",
			noText(), 29.50, int64(267), false,
			doc(t, map[string]any{"format": "hardcover", "languages": []any{"en"}}),
			noMoment()).
		Values(bookC, authorKernighan, noText(), "The C Programming Language",
			noText(), 45.00, int64(272), true,
			doc(t, map[string]any{"format": "paperback", "languages": []any{"en", "de"}}),
			noMoment()).
		Execute(ctx, conn)
	noError(t, "seed books", err)

	_, err = gooq.InsertInto(db.Review).
		Columns(db.Review.Id, db.Review.BookId, db.Review.Reviewer, db.Review.Rating, db.Review.Body).
		Values(reviewGoFirst, bookGo, "Ada", int64(5), text("A modern classic.")).
		Values(reviewGoSecond, bookGo, "Linus", int64(4), text("Solid and pragmatic.")).
		Values(reviewPractice, bookPractice, "Grace", int64(5), text("Timeless advice.")).
		Values(reviewCAnonymous, bookC, "Anonymous", int64(3), noText()).
		Execute(ctx, conn)
	noError(t, "seed reviews", err)
}

// The following constructors make nullable and document fixture values read as
// plain prose at the call site, for example text("..."), noText(), and
// doc(t, {...}).

func text(s string) sql.Null[string]          { return sql.Null[string]{V: s, Valid: true} }
func noText() sql.Null[string]                { return sql.Null[string]{} }
func moment(ts time.Time) sql.Null[time.Time] { return sql.Null[time.Time]{V: ts, Valid: true} }
func noMoment() sql.Null[time.Time]           { return sql.Null[time.Time]{} }

// doc marshals a Go map into a JSON document for a JSONB column, failing the
// test if the value cannot be marshalled.
func doc(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	noError(t, "marshal json document", err)
	return raw
}

// noDoc returns a NULL JSONB value.
func noDoc() json.RawMessage { return nil }

// noError fails the test immediately when err is non-nil, labelling the action
// that produced it.
func noError(t *testing.T, action string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
}

// equal fails the test when got differs from want, labelling the compared value.
func equal[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// isError fails the test when err does not wrap target, labelling the action.
func isError(t *testing.T, action string, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s error = %v, want %v", action, err, target)
	}
}
