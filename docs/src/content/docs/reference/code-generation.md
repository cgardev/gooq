---
title: Code Generation
description: Generating typed table accessors from a database schema with gooq-gen.
---

gooq ships with a code generator, `gooq-gen`, that introspects a live database and
produces typed Go accessors for every table, view, column, enum, and key. The
generated `db` package is what every example in this reference imports. The
generator is intended to be driven through `go generate`.

## Running the generator

Point `gooq-gen` at a registered driver and a data source, and name the output
directory and package.

```sh
go run github.com/cgardev/gooq/cmd/gooq-gen \
    -driver pgx \
    -dsn "postgres://user:pass@localhost:5432/db?sslmode=disable" \
    -schema public \
    -o internal/db \
    -package db
```

| Flag | Meaning |
| --- | --- |
| `-driver` | The registered `database/sql` driver name (for example `pgx`). |
| `-dsn` | The data source name used to connect for introspection. |
| `-schema` | The schema to introspect (defaults to `public`). |
| `-o` | The output directory for the generated package. |
| `-package` | The name of the generated package (defaults to `db`). |

The `-driver`, `-dsn`, and `-o` flags are required. The driver that backs the
data source must be registered before introspection, so blank-import it from the
program that runs the generator, for example `_ "github.com/jackc/pgx/v5/stdlib"`
for PostgreSQL or `_ "modernc.org/sqlite"` for SQLite.

## What is generated

For each table the generator emits a struct that embeds `gooq.TableImpl` and
exposes one typed field per column, plus a package-level accessor variable. Column
types map to the matching gooq field type:

- Text columns become `StringField`, adding `Like`, `NotLike`, `ILike`, and
  `Concat`.
- Numeric columns become `NumericField[T]`, adding arithmetic methods. An
  `integer` column maps to `NumericField[int64]` and a `numeric` column to
  `NumericField[float64]`.
- A nullable scalar column maps to `Field[sql.Null[T]]`, for example
  `Field[sql.Null[string]]`.
- A non-nullable `jsonb` column maps to `Field[json.RawMessage]`; a nullable one
  maps to `Field[[]byte]`.
- Other columns become `Field[T]` for the appropriate `T`, such as
  `Field[time.Time]` or `Field[bool]`.

Views are generated as read-only table accessors; the `book_overview` view, for
instance, becomes `db.BookOverview`.

## Enums

A SQL enum type is generated as a Go string type with one constant per value, in
the declared order. The `book_status` enum becomes a `BookStatus` type with
constants `db.BookStatusDraft`, `db.BookStatusInPrint`, and
`db.BookStatusOutOfPrint`. The `status` column is typed `Field[BookStatus]`, so
comparisons are checked at compile time.

```go
query := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusInPrint))
```

## Foreign keys and unique keys

Each generated table accessor records its constraints as metadata, returned by the
`ForeignKeys` and `UniqueKeys` methods. Foreign keys are reported as
`gooq.ForeignKeyMeta` values and unique keys as `gooq.UniqueKeyMeta` values, so
program code can inspect relationships at runtime. Views carry no key metadata.

```go
// ForeignKeyMeta describes a foreign key relationship.
type ForeignKeyMeta struct {
    Name       string
    Columns    []string
    RefTable   string
    RefColumns []string
}

for _, fk := range db.Book.ForeignKeys() {
    fmt.Println(fk.Name, fk.Columns, fk.RefTable, fk.RefColumns)
}
```

For example, the `book.author_id` foreign key is reported with `RefTable` set to
`author` and `RefColumns` set to `["id"]`.

## The generator-only constructors

The functions `NewTable`, `NewField`, `NewStringField`, and `NewNumericField`,
together with the `Column` method on a table, exist to construct the accessors
that `gooq-gen` writes. They are an implementation detail of the generated package
and are not intended to be called from application code; write queries through the
generated `db` accessors instead.

## Keeping accessors in sync

Re-run the generator whenever the schema changes. Because the output is ordinary
Go, a schema change that removes or retypes a column surfaces as a compile error
in the code that uses the old accessor, which keeps queries honest against the
real database.
