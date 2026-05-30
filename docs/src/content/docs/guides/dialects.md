---
title: Dialects
description: Render a single query definition for PostgreSQL and SQLite, with per-dialect placeholders, quoting, boolean literals, and upserts.
---

A gooq statement is an abstract query definition, not a string. The same
definition can be rendered for either supported dialect, and the renderer adapts
placeholders, identifier quoting, boolean literals, and upsert syntax to the
target database.

PostgreSQL support targets the latest two major versions (18 and 17). SQLite is
tested with the pure-Go `modernc.org/sqlite` driver.

## The dialect values

Two dialect constructors are available, each returning a `gooq.Dialect`:

```go
gooq.Postgres() // PostgreSQL
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
sql, args, err := query.SQLFor(gooq.SQLite())

// Bind a dialect, then execute.
rows, err := query.Using(gooq.Postgres()).Fetch(ctx, conn)
```

## One query, two renderings

The single definition below renders differently for each database. Note the
placeholder style:

```go
query := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Price.GT(10)).
	OrderBy(db.Book.Title.Asc()).
	Limit(5)

pgSQL, pgArgs, _ := query.SQLFor(gooq.Postgres())
liteSQL, liteArgs, _ := query.SQLFor(gooq.SQLite())
```

The rendered statements:

```sql
-- PostgreSQL
SELECT "book"."title", "book"."price" FROM "book"
WHERE "book"."price" > $1 ORDER BY "book"."title" ASC LIMIT 5

-- SQLite
SELECT "book"."title", "book"."price" FROM "book"
WHERE "book"."price" > ? ORDER BY "book"."title" ASC LIMIT 5
```

In both cases the argument list is the same — `[]any{10.0}` — because only the
rendering changes, not the values.

## What varies between dialects

### Placeholders

PostgreSQL uses ordinal placeholders (`$1`, `$2`, …). SQLite uses positional
question marks (`?`).

### Identifier quoting

PostgreSQL and SQLite both quote identifiers with double quotes (`"book"`).

### Boolean literals

PostgreSQL renders boolean values as the native `TRUE` / `FALSE` literals. SQLite
has no boolean type, so the same values render as `1` / `0`.

### Upsert syntax

Both PostgreSQL and SQLite express upserts with `ON CONFLICT ... DO UPDATE SET`
or `DO NOTHING`. The helper `gooq.SetToExcluded(field)` assigns a column to the
value that would have been inserted; the pseudo-row is named `EXCLUDED.` on
PostgreSQL and `excluded.` on SQLite:

```sql
-- PostgreSQL
ON CONFLICT ("id") DO UPDATE SET "title" = EXCLUDED."title"

-- SQLite
ON CONFLICT ("id") DO UPDATE SET "title" = excluded."title"
```

See [Inserts, Updates & Deletes](/gooq/guides/inserts-updates-deletes/) for the
builder methods that produce these clauses.

### RETURNING

Both PostgreSQL and SQLite support `RETURNING` on INSERT, UPDATE, and DELETE, so
`Returning(cols...)` renders for either dialect.

## Drivers are separate from dialects

The dialect controls how SQL is rendered. It does not open a connection. You
still blank-import a driver and pass a `*sql.DB` or `*sql.Tx` to the execution
terminals:

```go
import (
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL
	_ "modernc.org/sqlite"             // SQLite
)
```

Pairing the right driver with the right dialect is your responsibility; gooq
keeps the two concerns independent so it can stay dependency-free.
