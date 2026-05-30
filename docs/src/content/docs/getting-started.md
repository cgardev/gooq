---
title: Getting Started
description: Install gooq, generate typed table accessors, and run your first type-safe query.
---

This guide walks through installing gooq, generating typed accessors, and
running a first query end to end.

## Requirements

- Go 1.22 or newer.
- A database driver of your choice. gooq imports no driver itself, so you
  blank-import the one that matches your database.

## Installation

Add the library to your module:

```sh
go get github.com/cgardev/gooq
```

gooq has zero runtime dependencies. The only third-party package you need is a
database driver, which you import for its side effects so that it registers
itself with `database/sql`:

```go
import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver, blank-imported
)
```

## Opening a connection

gooq executes against the standard `database/sql` types. Any `*sql.DB` or
`*sql.Tx` satisfies the `gooq.Querier` interface used by the terminal `Fetch`
and `Execute` methods.

```go
conn, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/app?sslmode=disable")
if err != nil {
	return err
}
defer conn.Close()
```

## Generating typed accessors

Rather than referring to tables and columns by string, gooq works with typed
accessors generated from your live schema. Run the generator against a database
to emit a package of table definitions:

```sh
go run github.com/cgardev/gooq/cmd/gooq-gen \
	-driver pgx \
	-dsn "postgres://user:pass@localhost:5432/db?sslmode=disable" \
	-schema public \
	-o internal/db \
	-package db
```

This introspects `information_schema` and writes a `db` package exposing
accessors such as `db.Book`, with typed columns like `db.Book.Id`,
`db.Book.Title` (a `StringField`), and `db.Book.Price` (a
`NumericField[float64]`). See [Code Generation](/gooq/guides/code-generation/)
for the full details.

## Your first query

With the accessors generated, you can build and execute a query. The following
selects the title and price of every book priced above ten, ordered by title:

```go
package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/cgardev/gooq"
	"example.com/app/internal/db"
)

func main() {
	ctx := context.Background()

	conn, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/app?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rows, err := gooq.
		Select2(db.Book.Title, db.Book.Price).
		From(db.Book).
		Where(db.Book.Price.GT(10)).
		OrderBy(db.Book.Title.Asc()).
		Fetch(ctx, conn)
	if err != nil {
		panic(err)
	}

	for _, row := range rows {
		// row is a gooq.Record2[string, float64].
		fmt.Printf("%s costs %.2f\n", row.V1, row.V2)
	}
}
```

`Select2` returns rows typed as `gooq.Record2[string, float64]`, so `row.V1`
is a `string` and `row.V2` is a `float64` without any casting.

## Inspecting the generated SQL

You do not have to execute a query to see what it produces. The `SQL` and
`SQLFor` terminals return the rendered statement and its arguments:

```go
sql, args, err := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Price.GT(10)).
	SQLFor(gooq.Postgres())
// sql:  SELECT "book"."title", "book"."price" FROM "book" WHERE "book"."price" > $1
// args: []any{10.0}
```

## Next steps

- [Building Queries](/gooq/guides/building-queries/) covers the full SELECT
  surface and the step interfaces that enforce clause order.
- [Predicates & Conditions](/gooq/guides/predicates-and-conditions/) describes
  the operators available on every field.
- [Dialects](/gooq/guides/dialects/) explains how one query renders for three
  databases.
