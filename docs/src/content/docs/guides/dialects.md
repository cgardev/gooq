---
title: Dialects
description: Render a single query definition for PostgreSQL, MySQL, and SQLite, with per-dialect placeholders, quoting, RETURNING, and upserts.
---

A gooq statement is an abstract query definition, not a string. The same
definition can be rendered for any supported dialect, and the renderer adapts
placeholders, identifier quoting, `RETURNING` support, and upsert syntax to the
target database.

## The dialect values

Three dialect constructors are available, each returning a `gooq.Dialect`:

```go
gooq.Postgres() // PostgreSQL
gooq.MySQL()    // MySQL
gooq.SQLite()   // SQLite
```

## Choosing a dialect

There are two ways to apply a dialect:

- `SQLFor(d)` renders the SQL and arguments for a specific dialect without
  executing.
- `Using(d)` binds the dialect to the statement, after which the `Fetch` or
  `Execute` terminals run against that dialect.

```go
// Render only.
sql, args, err := query.SQLFor(gooq.MySQL())

// Bind a dialect, then execute.
rows, err := query.Using(gooq.Postgres()).Fetch(ctx, conn)
```

## One query, three renderings

The single definition below renders differently for each database. Note the
placeholder style and the identifier quoting:

```go
query := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Price.GT(10)).
	OrderBy(db.Book.Title.Asc()).
	Limit(5)

pgSQL, pgArgs, _ := query.SQLFor(gooq.Postgres())
mySQL, myArgs, _ := query.SQLFor(gooq.MySQL())
liteSQL, liteArgs, _ := query.SQLFor(gooq.SQLite())
```

The rendered statements:

```sql
-- PostgreSQL
SELECT "book"."title", "book"."price" FROM "book"
WHERE "book"."price" > $1 ORDER BY "book"."title" ASC LIMIT 5

-- MySQL
SELECT `book`.`title`, `book`.`price` FROM `book`
WHERE `book`.`price` > ? ORDER BY `book`.`title` ASC LIMIT 5

-- SQLite
SELECT "book"."title", "book"."price" FROM "book"
WHERE "book"."price" > ? ORDER BY "book"."title" ASC LIMIT 5
```

In every case the argument list is the same — `[]any{10.0}` — because only the
rendering changes, not the values.

## What varies between dialects

### Placeholders

PostgreSQL uses ordinal placeholders (`$1`, `$2`, …). MySQL and SQLite use
positional question marks (`?`).

### Identifier quoting

PostgreSQL and SQLite quote identifiers with double quotes (`"book"`). MySQL
quotes with backticks.

### RETURNING support

PostgreSQL and SQLite support `RETURNING` on INSERT, UPDATE, and DELETE.
Requesting `Returning` on a dialect that does not support it produces
`gooq.ErrReturningUnsupported`:

```go
_, _, err := gooq.
	InsertInto(db.Book).
	Columns(db.Book.Title).
	Values("x").
	Returning(db.Book.Id).
	SQLFor(gooq.MySQL())
// err is gooq.ErrReturningUnsupported
```

### Upsert syntax

PostgreSQL and SQLite express upserts with `ON CONFLICT ... DO UPDATE SET` or
`DO NOTHING`. MySQL uses `ON DUPLICATE KEY UPDATE`. gooq exposes the matching
builder methods (`OnConflict` / `DoUpdateSet` / `DoNothing` versus
`OnDuplicateKeyUpdate`); see
[Inserts, Updates & Deletes](/gooq/guides/inserts-updates-deletes/).

## Drivers are separate from dialects

The dialect controls how SQL is rendered. It does not open a connection. You
still blank-import a driver and pass a `*sql.DB` or `*sql.Tx` to the execution
terminals:

```go
import (
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL
	_ "github.com/go-sql-driver/mysql" // MySQL
	_ "modernc.org/sqlite"             // SQLite
)
```

Pairing the right driver with the right dialect is your responsibility; gooq
keeps the two concerns independent so it can stay dependency-free.
