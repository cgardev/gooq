---
title: SELECT Statements
description: Building SELECT queries with gooq, including joins, grouping, ordering, pagination, and row locking.
---

A `SELECT` statement starts from one of the `Select1` through `Select22` free
functions. The number in the name is the number of projected columns, and the
returned builder is typed by those columns so that each fetched row carries the
correct Go types.

## Projection

```go
query := gooq.Select2(db.Book.Id, db.Book.Title).
    From(db.Book)
```

```sql
SELECT "book"."id", "book"."title" FROM "book"
```

Use `Star` to project an unqualified `*`. It takes no arguments and is
type-erased, so it serves as a projection column or as an argument to a function:

```go
query := gooq.Select1(gooq.Star()).
    From(db.Book)
```

```sql
SELECT * FROM "book"
```

Columns may be aliased with `As`:

```go
query := gooq.Select1(db.Book.Title.As("book_title")).
    From(db.Book)
```

```sql
SELECT "book"."title" AS "book_title" FROM "book"
```

## FROM and joins

`From` sets the primary table. Joins are added with `Join`, `InnerJoin`,
`LeftJoin`, or `RightJoin`, each followed by `On` to supply the join condition.

```go
query := gooq.Select2(db.Book.Title, db.Author.Name).
    From(db.Book).
    InnerJoin(db.Author).On(db.Author.Id.EQField(db.Book.AuthorId))
```

```sql
SELECT "book"."title", "author"."name"
FROM "book"
INNER JOIN "author" ON "author"."id" = "book"."author_id"
```

`Join` is an alias for `InnerJoin`. Multiple joins chain naturally:

```go
query := gooq.Select3(db.Book.Title, db.Author.Name, db.Review.Rating).
    From(db.Book).
    InnerJoin(db.Author).On(db.Author.Id.EQField(db.Book.AuthorId)).
    LeftJoin(db.Review).On(db.Review.BookId.EQField(db.Book.Id))
```

A `LeftJoin` may leave right-hand columns null. Project nullable fields (for
example `Field[sql.Null[string]]`) or map into pointer or `sql.Null[T]` struct
fields to capture the absent rows.

## WHERE

`Where` takes a condition. Chain `And` and `Or` on the builder to extend it.

```go
query := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(db.Book.InPrint.EQ(true)).
    And(db.Book.PageCount.GT(int64(100)))
```

```sql
SELECT "book"."id" FROM "book"
WHERE "book"."in_print" = $1 AND "book"."page_count" > $2
```

To build nested boolean logic, combine conditions with the free functions `And`,
`Or`, and `Not` before passing them to `Where`. See
[Expressions](/gooq/reference/expressions/) for details.

## DISTINCT

`Distinct` removes duplicate rows from the result.

```go
query := gooq.Select1(db.Book.Status).
    From(db.Book).
    Distinct()
```

```sql
SELECT DISTINCT "book"."status" FROM "book"
```

## GROUP BY and HAVING

`GroupBy` takes one or more grouping fields. `Having` filters the grouped rows
and accepts the same conditions as `Where`, typically over an aggregate.

```go
query := gooq.Select2(db.Book.AuthorId, gooq.Count(db.Book.Id)).
    From(db.Book).
    GroupBy(db.Book.AuthorId).
    Having(gooq.Count(db.Book.Id).GT(int64(3)))
```

```sql
SELECT "book"."author_id", COUNT("book"."id")
FROM "book"
GROUP BY "book"."author_id"
HAVING COUNT("book"."id") > $1
```

## ORDER BY

`OrderBy` accepts order expressions produced by `Asc` and `Desc` on any field.

```go
query := gooq.Select2(db.Book.Title, db.Book.Price).
    From(db.Book).
    OrderBy(db.Book.Price.Desc(), db.Book.Title.Asc())
```

```sql
SELECT "book"."title", "book"."price"
FROM "book"
ORDER BY "book"."price" DESC, "book"."title" ASC
```

### NULLS FIRST and NULLS LAST

Order expressions expose `NullsFirst` and `NullsLast` to control where null
values appear.

```go
query := gooq.Select2(db.Book.Title, db.Book.PublishedAt).
    From(db.Book).
    OrderBy(db.Book.PublishedAt.Desc().NullsLast())
```

```sql
SELECT "book"."title", "book"."published_at"
FROM "book"
ORDER BY "book"."published_at" DESC NULLS LAST
```

## LIMIT and OFFSET

`Limit` and `Offset` both take an `int64`. Combine them for pagination.

```go
const pageSize = int64(20)
page := int64(2)

query := gooq.Select2(db.Book.Id, db.Book.Title).
    From(db.Book).
    OrderBy(db.Book.CreatedAt.Desc()).
    Limit(pageSize).
    Offset((page - 1) * pageSize)
```

```sql
SELECT "book"."id", "book"."title"
FROM "book"
ORDER BY "book"."created_at" DESC
LIMIT 20 OFFSET 20
```

LIMIT and OFFSET are rendered as inline integers, not bind parameters.

## Row locking

The locking clauses `ForUpdate`, `ForShare`, and `SkipLocked` are emitted for
PostgreSQL. SQLite has no row-locking clause, so they are silently omitted when
the query is rendered for SQLite. Apply them at the end of the chain.

```go
query := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusDraft)).
    ForUpdate().
    SkipLocked()
```

```sql
SELECT "book"."id" FROM "book" WHERE "book"."status" = $1 FOR UPDATE SKIP LOCKED
```

Use `ForShare` for a shared lock that blocks writers but allows other readers.

## Terminal methods

A finished `SELECT` is turned into SQL or executed with one of:

- `SQL()` — render SQL and arguments for the default dialect, returning
  `(string, []any, error)`.
- `SQLFor(dialect)` — render for a specific dialect, for example
  `gooq.Postgres()` or `gooq.SQLite()`.
- `Using(dialect)` — bind the query to a dialect before fetching.
- `Fetch(ctx, conn)` — return all rows.
- `FetchOne(ctx, conn)` — return the first row, or `sql.ErrNoRows` when empty.
- `FetchSingle(ctx, conn)` — return exactly one row; returns `sql.ErrNoRows` when
  empty and `gooq.ErrTooManyRows` when more than one row matches.
- `FetchOptional(ctx, conn)` — return `(row, found, error)`, with `found` false
  when no row matches, and `gooq.ErrTooManyRows` when more than one matches.

```go
rows, err := gooq.Select2(db.Book.Id, db.Book.Title).
    From(db.Book).
    Where(db.Book.InPrint.EQ(true)).
    Fetch(ctx, conn)
```

For mapping rows into structs, see
[Fetching and mapping](/gooq/reference/fetching-and-mapping/).
