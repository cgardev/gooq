---
title: Code Generation
description: Use cmd/gooq-gen to introspect information_schema and emit typed table accessors, including nullable and JSON column handling.
---

The `cmd/gooq-gen` command generates typed table accessors from a live
database. Rather than referencing tables and columns by string, you work with
generated values whose Go types match the database schema, which is what makes
queries type-safe end to end.

## Running the generator

The generator connects to a database, reads `information_schema`, and writes a
Go package. A driver must be blank-imported when the command is built, so it is
passed by name and selected at run time:

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
| `-schema` | The schema to introspect (for example `public`). |
| `-o` | The output directory for the generated package. |
| `-package` | The name of the generated Go package. |

## What is generated

For each table the generator emits a struct that embeds `gooq.TableImpl` and
exposes one field per column, plus a package-level variable you use as the
accessor. Given a `book` table, the result looks like:

```go
package db

var Book = newBookTable("book")

type bookTable struct {
	gooq.TableImpl
	Id    gooq.NumericField[int64]
	Title gooq.StringField
	Price gooq.NumericField[float64]
	// ... one field per column
}
```

You then refer to columns as `db.Book.Id`, `db.Book.Title`, and
`db.Book.Price`, and to the table itself as `db.Book`.

## Field types

Columns are mapped to the most specific gooq field type so that the appropriate
operators are available:

- Plain scalar columns become `gooq.Field[T]`.
- Text columns become `gooq.StringField`, adding `Like`, `NotLike`, and
  `ILike`.
- Numeric columns become `gooq.NumericField[T]`, adding `Add`, `Sub`, `Mul`,
  and `Div`.

### Nullable columns

A nullable scalar column is typed as `gooq.Field[sql.Null[T]]`, mirroring the
nullable wrapper from `database/sql`. For example, a nullable `integer` column
becomes `gooq.Field[sql.Null[int64]]`, and reading it back yields a value whose
`.Valid` field indicates presence.

### JSON columns

JSON and JSONB columns map according to nullability:

- A non-nullable `json` or `jsonb` column becomes
  `gooq.Field[json.RawMessage]`.
- A nullable `json` or `jsonb` column becomes `gooq.Field[[]byte]`.

## Aliasing generated tables

Each generated table provides an `As(alias)` method, which is essential for
self-joins and disambiguation:

```go
b := db.Book.As("b")

rows, err := gooq.
	Select1(b.Title).
	From(b).
	Fetch(ctx, conn)
```

## The constructors behind the output

The generated code is ordinary Go that you could also write by hand. It is
built from a small set of constructors:

```go
base := gooq.NewTable("book").WithAlias("b")

id := gooq.NewNumericField[int64](base, "id")
title := gooq.NewStringField(base, "title")
price := gooq.NewNumericField[float64](base, "price")
data := gooq.NewField[json.RawMessage](base, "data")
```

- `gooq.NewTable(name)` creates a table, and `WithAlias(alias)` aliases it.
- `gooq.NewField[T](base, "col")` creates a plain field.
- `gooq.NewStringField(base, "col")` creates a string field.
- `gooq.NewNumericField[T](base, "col")` creates a numeric field.

## Keeping accessors in sync

Re-run the generator whenever the schema changes. Because the output is plain
Go, a schema change that removes or retypes a column surfaces as a compile error
in the code that uses the old accessor, which keeps queries honest against the
real database.

## Blank-imported driver

The generator does not bundle any database driver, and neither does the runtime
library. Build the command with the driver you need blank-imported so it
registers with `database/sql`, and pass its registered name to `-driver`. The
same applies to the application that runs the generated queries.
