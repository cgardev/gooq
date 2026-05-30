# gooq

A type-safe, fluent, zero-dependency SQL query builder for Go, inspired by [jOOQ](https://www.jooq.org/). `gooq` gives you parametric `Field[T]` columns whose comparison methods reject mismatched types at compile time, positional `RecordN` row types that preserve each projected column's Go type by position, step interfaces that turn clause order into a compile-time concern (you cannot place `WHERE` after `GROUP BY`, but you may omit it entirely), and runtime dialect translation: a query is built once as a detached abstract syntax tree and rendered to dialect-specific SQL for PostgreSQL and SQLite at execution time.

[![Go Reference](https://pkg.go.dev/badge/github.com/cgardev/gooq.svg)](https://pkg.go.dev/github.com/cgardev/gooq)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![CI](https://img.shields.io/github/actions/workflow/status/cgardev/gooq/ci.yml?branch=main&label=CI)](https://github.com/cgardev/gooq/actions/workflows/ci.yml)
[![Status: Alpha](https://img.shields.io/badge/status-alpha-orange.svg)](#status-and-roadmap)

> [!WARNING]
> **Alpha.** This project is in early, active development. The public API, wire
> behaviour, and module layout may change without notice between releases, and it
> has not been hardened for production use. There are **no tagged releases yet**:
> dependencies resolve to a commit, so pin an exact commit (a pseudo-version such
> as `v0.0.0-<date>-<commit>`) and review the change history before upgrading.
> See the [disclaimer](#status-and-roadmap).

## Motivation

I have written Go for many years, and I have never found a library that lets me
build **fully typed, composable SQL queries** the way [jOOQ](https://www.jooq.org/)
does in Java. The Go ecosystem is rich, but each option I tried gave up something
essential:

- **[SQLBoiler](https://github.com/volatiletech/sqlboiler)** generates typed
  models from a live schema, which is the right idea, but it follows the Active
  Record pattern: the generated query surface is shaped around row objects and
  their relations, which constrains how you express the schema and how far you can
  go with ad-hoc projections and dynamic query composition.
- **[pgx](https://github.com/jackc/pgx)** is an excellent PostgreSQL driver, but
  it is not typed at the query level. Queries are strings, results are scanned by
  hand, and a schema change does not break the build — it breaks at runtime, often
  far from the query that is now wrong.
- **[GORM](https://gorm.io/)** does a lot of implicit work through struct tags,
  hooks, and auto-migration. The "magic" is convenient until it produces
  surprising behaviour, and much of it resolves through runtime reflection rather
  than the compiler.
- **[ent](https://entgo.io/)** is powerful, but it is a code-first entity/graph
  framework with its own DSL; it does not read like SQL and steers you toward its
  own model of the world.
- **[sqlc](https://sqlc.dev/)** generates excellent typed Go from hand-written
  SQL, but the queries are static: there is no dynamic, programmatic query
  builder for predicates assembled at runtime.
- **[squirrel](https://github.com/Masterminds/squirrel)** is a fluent builder,
  but columns are plain strings and values are `interface{}`, so there is no type
  safety and no schema awareness.
- **[go-jet](https://github.com/go-jet/jet)** is the closest in spirit — schema
  code generation plus a typed-ish builder — but its typing is by category
  (`ColumnString`, `ColumnInteger`, ...) rather than the real Go type via
  `Field[T]`, it has no step interfaces to enforce clause order at compile time,
  and the dialect is fixed at generation time rather than chosen at render time.

The gap is real: nothing in Go combines live-database code generation, parametric
`Field[T]` columns, positional `RecordN` rows, step interfaces that enforce clause
order, **and** runtime dialect translation — the exact combination that makes jOOQ
feel like *typed SQL* rather than "strings with autocomplete". Go 1.18+ generics
finally make that design viable.

So I decided to build a **port of jOOQ's ideas to Go**, adapted to Go's syntactic
constraints rather than copied from Java or Kotlin. Some of jOOQ's ergonomics do
not translate directly — Go methods cannot have their own type parameters (hence
top-level `Select1` … `Select22` instead of overloaded `select`), there is no
operator overloading (hence `EQ`/`GT`/`Like` methods), and varargs are not
generic (hence the generated `RecordN` arities). The result keeps jOOQ's core
promise — SQL the compiler checks — while staying idiomatic Go.

## Features

- **Type-safe columns.** A `Field[T]` accepts only values of its own type `T`, so `db.Book.Price.GT(10)` compiles while comparing a price against a string does not.
- **Positional typed results.** `Select1` through `Select22` return `SelectFromStep[RecordN[...]]`, and each fetched row preserves the projected column types by position (`row.V1`, `row.V2`, ...).
- **Compile-time clause order.** Each builder method returns the interface describing the clauses that may legally follow, encoding the legal SQL grammar in the type system.
- **Runtime dialect translation.** One abstract syntax tree renders to PostgreSQL and SQLite through `SQLFor` or `Using`, including dialect-specific placeholders, identifier quoting, `RETURNING`, and upsert syntax.
- **Full DML coverage.** `SELECT` (with joins, `GROUP BY`, `HAVING`, `ORDER BY`, `LIMIT`/`OFFSET`), `INSERT` (with `ON CONFLICT` upserts and `RETURNING`), `UPDATE`, and `DELETE`.
- **Composable predicates.** A `Condition` is itself a `Field[bool]`, so it can be stored in a variable and combined with `And`, `Or`, and `Not`.
- **Code generation.** `gooq-gen` introspects `information_schema` and emits typed table accessors.
- **Zero dependencies.** The core module imports nothing outside the standard library and ships no database driver; you blank-import your own.

## Install

Because there are no tagged releases yet, `go get` resolves to the latest commit
and records a pseudo-version pinned to that commit hash:

```sh
go get github.com/cgardev/gooq@latest
```

To pin a specific commit explicitly:

```sh
go get github.com/cgardev/gooq@<commit-hash>
```

The recorded version in `go.mod` will look like `v0.0.0-<date>-<commit>` until
the first tagged release.

## Quick start

The examples below use a generated `db` package (see [Code generation](#code-generation)). For a `book` table, the generator produces typed accessors such as `db.Book.Title` (a `StringField`) and `db.Book.Price` (a `NumericField[float64]`).

### SELECT

```go
package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // your driver, blank-imported

	"github.com/cgardev/gooq"
	"github.com/cgardev/gooq/example/internal/db"
)

func main() {
	conn, err := sql.Open("pgx", "postgres://localhost:5432/library?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ctx := context.Background()

	// db.Book.Price is a NumericField[float64], so GT requires a float64;
	// a mismatched type would be a compile error, not a runtime surprise.
	rows, err := gooq.Select2(db.Book.Title, db.Book.Price).
		From(db.Book).
		Where(db.Book.Price.GT(10)).
		OrderBy(db.Book.Title.Asc()).
		Limit(20).
		Fetch(ctx, conn)
	if err != nil {
		panic(err)
	}

	// Each row is a Record2[string, float64]; the column types are preserved
	// by position.
	for _, row := range rows {
		fmt.Printf("%s: %.2f\n", row.V1, row.V2)
	}
}
```

`Fetch` returns every matching row. To read a single row, use `FetchOne` (returns the zero value when nothing matches and `gooq.ErrTooManyRows` when more than one matches) or `FetchSingle` (returns `sql.ErrNoRows` when nothing matches). The `conn` argument is a `gooq.Querier`, satisfied by `*sql.DB`, `*sql.Tx`, and `*sql.Conn`.

### One AST, two dialects

The query is rendered, not re-built, so the same value produces SQL for every dialect via `SQLFor`:

```go
q := gooq.Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Price.GT(10)).
	OrderBy(db.Book.Title.Asc()).
	Limit(20)

pg, args, _ := q.SQLFor(gooq.Postgres())
sq, _, _ := q.SQLFor(gooq.SQLite())

fmt.Println(pg)   // SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" > $1 ORDER BY "book"."title" ASC LIMIT 20
fmt.Println(sq)   // SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" > ? ORDER BY "book"."title" ASC LIMIT 20
fmt.Println(args) // [10]
```

Call `Using(dialect)` to bind a dialect for the terminal `SQL`, `Fetch`, `FetchOne`, and `FetchSingle` methods; the default is `gooq.Postgres()`.

### INSERT with upsert

`SetToExcluded` sets a column to the value the conflicting `INSERT` attempted to write (`EXCLUDED.col` on PostgreSQL and `excluded.col` on SQLite):

```go
_, err := gooq.InsertInto(db.Book).
	Columns(db.Book.Id, db.Book.Title, db.Book.Price).
	Values(int64(1), "The Go Programming Language", 39.99).
	OnConflict(db.Book.Id).
	DoUpdateSet(gooq.SetToExcluded(db.Book.Title), gooq.SetToExcluded(db.Book.Price)).
	Execute(ctx, conn)
```

Use `OnConflictDoNothing()` to ignore the conflict. `Returning(cols...)` is available on both PostgreSQL and SQLite.

### UPDATE

```go
_, err := gooq.Update(db.Book).
	Set(db.Book.Price.Set(29.99)).
	Where(db.Book.Id.EQ(1)).
	Execute(ctx, conn)
```

### DELETE

```go
_, err := gooq.DeleteFrom(db.Book).
	Where(db.Book.Price.LT(5)).
	Returning(db.Book.Id).
	Execute(ctx, conn)
```

`Update`, `DeleteFrom`, and `InsertInto` all return `sql.Result` from `Execute`. To read a `RETURNING` projection, render the statement with `SQL()` and run it through `QueryRowContext`/`QueryContext`:

```go
query, args, err := gooq.DeleteFrom(db.Book).
	Where(db.Book.Id.EQ(1)).
	Returning(db.Book.Id).
	SQL()
if err != nil {
	panic(err)
}

var id int64
err = conn.QueryRowContext(ctx, query, args...).Scan(&id)
```

### Composable conditions

A `Condition` is a `Field[bool]`, so predicates can be stored and combined:

```go
cheap := db.Book.Price.LT(10)
goBook := db.Book.Title.Like("Go%")

rows, err := gooq.Select1(db.Book.Id).
	From(db.Book).
	Where(cheap.And(goBook)).
	Fetch(ctx, conn)
```

### Joins with aliased tables

```go
a := db.Author.As("a")

rows, err := gooq.Select2(db.Book.Title, a.Name).
	From(db.Book).
	LeftJoin(a).On(db.Book.AuthorId.EQField(a.Id)).
	Fetch(ctx, conn)
```

## Code generation

The `gooq-gen` command introspects a live database through the standard `information_schema` catalog. For each table it writes a `<table>.gen.go` file containing an embedded `gooq.TableImpl`, one typed `Field` per column, an `As` method for aliasing, and a package-level accessor variable. The example accessor for a `book` table looks like:

```go
// Code generated by gooq-gen; DO NOT EDIT.

package db

import (
	"github.com/cgardev/gooq"
)

type bookTable struct {
	gooq.TableImpl
	Id       gooq.NumericField[int64]
	Title    gooq.StringField
	Price    gooq.NumericField[float64]
	AuthorId gooq.NumericField[int64]
}

func newBookTable(alias string) *bookTable {
	base := gooq.NewTable("book").WithAlias(alias)
	return &bookTable{
		TableImpl: base,
		Id:        gooq.NewNumericField[int64](base, "id"),
		Title:     gooq.NewStringField(base, "title"),
		Price:     gooq.NewNumericField[float64](base, "price"),
		AuthorId:  gooq.NewNumericField[int64](base, "author_id"),
	}
}

var Book = newBookTable("")
```

The library itself imports no database driver, so the generator is built with your driver blank-imported. Run it directly with `go run`:

```go
go run github.com/cgardev/gooq/cmd/gooq-gen -dsn "postgres://localhost:5432/library?sslmode=disable" -o internal/db
```

Flags: `-driver` (the `database/sql` driver name, default `postgres`), `-dsn` (required), `-schema` (default `public`), `-o` (output directory, default `internal/db`), and `-package` (default `db`). Because the command opens the connection through `database/sql`, the chosen driver must be blank-imported into the build, for example `_ "github.com/lib/pq"` for PostgreSQL or `_ "modernc.org/sqlite"` for SQLite.

### Column type mapping

Non-nullable columns map to the refined field types: integers to `gooq.NumericField[int64]`, floating and fixed-point types to `gooq.NumericField[float64]`, booleans to `gooq.Field[bool]`, temporal types to `gooq.Field[time.Time]`, text types to `gooq.StringField`, binary types to `gooq.Field[[]byte]`, and `json`/`jsonb` to `gooq.Field[json.RawMessage]`.

Nullable columns whose element is a scalar are wrapped in the generic `sql.Null[T]` type, for example a nullable text column becomes `gooq.Field[sql.Null[string]]` and a nullable timestamp becomes `gooq.Field[sql.Null[time.Time]]`. Byte slices already scan `NULL` as `nil`, so they are left unwrapped; a nullable `json`/`jsonb` column maps to a plain `gooq.Field[[]byte]`, because `database/sql` cannot scan a `NULL` driver value into the named `json.RawMessage` slice type.

## Dialects

gooq targets the latest two PostgreSQL majors (18 and 17) and SQLite. The SQLite
support is tested with the pure-Go [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
driver.

| Capability | `Postgres()` | `SQLite()` |
| --- | --- | --- |
| Bind placeholders | `$1`, `$2`, ... | `?` |
| Identifier quoting | `"name"` | `"name"` |
| Boolean literals | `TRUE` / `FALSE` | `1` / `0` |
| `RETURNING` | yes | yes |
| Upsert syntax | `ON CONFLICT (...) DO UPDATE` / `DO NOTHING` | `ON CONFLICT (...) DO UPDATE` / `DO NOTHING` |
| Excluded-row reference | `EXCLUDED.col` | `excluded.col` |

`ILike` renders as `ILIKE` on PostgreSQL and as a plain `LIKE` on SQLite, whose `LIKE` is already case-insensitive for ASCII text.

## Testing

The dependency-free core module is tested with the standard toolchain:

```go
go test ./...
```

Database integration tests live in a separate module under `./integration`, kept apart on purpose so that consuming `gooq` never pulls in their dependencies. They start a real PostgreSQL instance through [testcontainers](https://golang.testcontainers.org/), so Docker must be available. Run them from the integration module:

```go
go test ./...        # run inside ./integration; requires Docker
```

Pass `-short` to skip the container-backed tests:

```go
go test ./... -short # inside ./integration, skips tests that require Docker
```

## Status and roadmap

> [!WARNING]
> **Alpha — read before depending on this.** The library is under early, active
> development and is **not hardened for production use**. The public API, the
> generated-code layout, and the module structure may change without notice. No
> versions are tagged yet, so the module is consumed by commit: every `go get`
> resolves to a pseudo-version of the form `v0.0.0-<date>-<commit>`. Pin an exact
> commit and review the change history before upgrading.

Implemented and covered by tests:

- `SELECT` with joins, `WHERE`/`AND`/`OR`, `GROUP BY`, `HAVING`, `ORDER BY`, and `LIMIT`/`OFFSET`
- `INSERT`, `UPDATE`, and `DELETE`
- `RETURNING` on PostgreSQL and SQLite
- Upserts (`ON CONFLICT ... DO UPDATE` / `DO NOTHING`)
- `Field[T]` arities `Select1` through `Select22` with positional `RecordN` results
- PostgreSQL and SQLite dialect rendering
- Schema-driven code generation via `gooq-gen`

Not yet implemented:

- Common table expressions (CTEs)
- Window functions

## Acknowledgements and trademarks

`gooq` is an independent, clean-room reimplementation of jOOQ's design ideas in
Go. It contains no jOOQ source code; the Go code here is original. jOOQ's
**Open Source Edition** is published by Data Geekery GmbH under the
[Apache License 2.0](https://github.com/jOOQ/jOOQ/blob/main/LICENSE), a permissive
licence, and reimplementing an API or design in a new language is well-established
practice. This project draws inspiration from that design with gratitude.

**jOOQ** is a registered trademark of Data Geekery GmbH. `gooq` is **not affiliated
with, endorsed by, or sponsored by** Data Geekery GmbH or the jOOQ project. The
name "jOOQ" is used here only nominatively, to describe the design `gooq` is
inspired by.

## License

`gooq` is released under the [MIT License](LICENSE), Copyright (c) 2026 Cristian
Garcia. This is your own original work; you are free to license it under MIT.
