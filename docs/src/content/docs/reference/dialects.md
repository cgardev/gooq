---
title: Dialects
description: How gooq renders SQL for PostgreSQL and SQLite, and how to select a dialect.
---

gooq targets two SQL dialects: PostgreSQL and SQLite. Any statement is an
abstract definition that can be rendered for either; the generated SQL differs in
placeholder style, boolean literals, and the set of clauses each engine supports.

## Selecting a dialect

`gooq.Postgres()` and `gooq.SQLite()` return `gooq.Dialect` values. Pass one to
`SQLFor` to render, or to `Using` to bind it to a statement before fetching or
executing.

```go
sqlText, args, err := gooq.Select1(db.Book.Id).
    From(db.Book).
    SQLFor(gooq.SQLite())
```

```go
rows, err := gooq.Select1(db.Book.Id).
    From(db.Book).
    Using(gooq.Postgres()).
    Fetch(ctx, conn)
```

The same builder thus serves both engines; choose the dialect that matches the
connection you fetch with. Builders default to PostgreSQL when no dialect is
bound.

## Placeholder styles

PostgreSQL uses numbered placeholders (`$1`, `$2`, ...); SQLite uses anonymous
positional `?`.

```go
query := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(db.Book.PageCount.GT(int64(100)))
```

```sql
-- PostgreSQL
SELECT "book"."id" FROM "book" WHERE "book"."page_count" > $1

-- SQLite
SELECT "book"."id" FROM "book" WHERE "book"."page_count" > ?
```

## Identifier quoting and boolean literals

Both dialects quote identifiers with double quotes, so a column renders as
`"book"."title"` for either engine. Boolean literals differ: PostgreSQL renders
`TRUE` and `FALSE`, while SQLite has no boolean type and renders `1` and `0`.

## Dialect-specific clauses

Some features are rendered for PostgreSQL only:

- Row-locking clauses `ForUpdate`, `ForShare`, and `SkipLocked`. SQLite has no
  row-locking clause, so these are silently omitted when rendering for SQLite.
- `DELETE ... USING` (the `UsingTable` method). On SQLite the `USING` clause is
  omitted; express the relationship with a subquery for portability.
- `ILIKE`. The case-insensitive `ILike` predicate is PostgreSQL specific; on
  SQLite use `Like`, whose default collation is already case-insensitive for
  ASCII text.

Both engines support `RETURNING` on `INSERT`, `UPDATE`, and `DELETE`, and both
express upserts with `ON CONFLICT ... DO UPDATE SET` or `DO NOTHING`. The only
difference in the upsert is the excluded-row reference: `EXCLUDED.` on PostgreSQL
and `excluded.` on SQLite, which `gooq.SetToExcluded` handles automatically.

When portability matters, prefer features available on both engines and pick SQL
type names for `Cast` that both engines understand, for example `integer` rather
than a PostgreSQL-only type.

## Drivers are separate from dialects

A dialect controls only how SQL is rendered; it does not open a connection. Blank-
import the driver appropriate to the database and pass a `*sql.DB` or `*sql.Tx` to
the terminals. Pairing the right driver with the right dialect is the caller's
responsibility, which keeps the core library free of database dependencies.
