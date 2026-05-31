---
title: Code Generation and Setup
description: Recipes for running gooq-gen, type overrides, enums, views, metadata, and dialect selection.
---

## Generate accessors with a blank-imported driver

You want to generate the `db` package from a live database.

```sh
go run github.com/cgardev/gooq/cmd/gooq-gen \
    -driver pgx \
    -dsn "postgres://user:pass@localhost:5432/db?sslmode=disable" \
    -schema public \
    -o internal/db \
    -package db
```

The generator needs a registered driver, so blank-import it from the program that
runs it (typically a small `internal/gendb` command wired to `go generate`):

```go
import (
    _ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL
    _ "modernc.org/sqlite"             // SQLite
)
```

Gotcha: without the blank import the driver name passed to `-driver` is not
registered with `database/sql` and introspection fails immediately.

## Override a SQL type (uuid to a custom type)

You want UUID columns generated as `uuid.UUID` rather than the default string
fields.

```sh
go run github.com/cgardev/gooq/cmd/gooq-gen \
    -driver pgx \
    -dsn "postgres://user:pass@localhost:5432/db?sslmode=disable" \
    -o internal/db \
    -type "uuid=github.com/google/uuid.UUID"
```

Gotcha: the override value is the fully qualified Go type path; the generator adds
the matching import to the output package.

## Use a generated enum

You want to filter on an enum column with compile-time safety.

```go
rows, err := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusInPrint)).
    Fetch(ctx, conn)
```

```sql
SELECT "book"."title" FROM "book" WHERE "book"."status" = $1
```

Gotcha: the `book_status` enum becomes a `BookStatus` type with one constant per
label, so use the generated constants (`db.BookStatusInPrint`) rather than raw
strings.

## Query a generated view

You want to read from the `book_overview` view.

```go
type Overview struct {
    Title      string `db:"title"`
    AuthorName string `db:"author_name"`
}

rows, err := gooq.FetchInto[Overview](ctx, conn,
    gooq.Select2(
        db.BookOverview.Title,
        db.BookOverview.AuthorName,
    ).
        From(db.BookOverview),
)
```

Gotcha: views are read-only accessors with no key metadata; build `SELECT`
queries against them, not `INSERT`, `UPDATE`, or `DELETE`.

## Inspect foreign keys through metadata

You want to read a table's foreign key relationships at runtime.

```go
for _, fk := range db.Book.ForeignKeys() {
    fmt.Printf("%s: %v -> %s.%v\n",
        fk.Name, fk.Columns, fk.RefTable, fk.RefColumns)
}
```

Gotcha: foreign keys are reported as `gooq.ForeignKeyMeta` values
(`Name`, `Columns`, `RefTable`, `RefColumns`); unique keys use
`gooq.UniqueKeyMeta`.

## Switch dialects with SQLFor and Using

You want the same query rendered or run for either engine.

```go
query := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(db.Book.PageCount.GT(int64(100)))

// Render for inspection.
pgSQL, pgArgs, _ := query.SQLFor(gooq.Postgres())
liteSQL, liteArgs, _ := query.SQLFor(gooq.SQLite())

// Or bind a dialect before fetching.
rows, err := query.Using(gooq.SQLite()).Fetch(ctx, conn)

_, _, _, _ = pgSQL, pgArgs, liteSQL, liteArgs
```

```sql
-- PostgreSQL: SELECT "book"."id" FROM "book" WHERE "book"."page_count" > $1
-- SQLite:     SELECT "book"."id" FROM "book" WHERE "book"."page_count" > ?
```

Gotcha: match the bound dialect to the connection you fetch with, since
placeholder styles differ; builders default to PostgreSQL when no dialect is
bound.
