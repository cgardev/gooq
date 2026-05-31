---
title: Inserts, Updates & Deletes
description: Write rows with INSERT, UPDATE, and DELETE, including RETURNING clauses and dialect-aware upserts.
---

gooq builds write statements with the same fluent, type-checked style as
queries. Each statement ends in a terminal that either renders the SQL
(`SQL`, `SQLFor`, `Using`) or executes it (`Execute`).

## INSERT

Begin with `InsertInto`, list the target columns with `Columns`, and supply one
or more rows with `Values`:

```go
result, err := gooq.
	InsertInto(db.Book).
	Columns(db.Book.Title, db.Book.Price, db.Book.AuthorId).
	Values("The Go Programming Language", 39.99, 7).
	Execute(ctx, conn)
```

### Multiple rows

Repeat `Values` to insert several rows in one statement:

```go
result, err := gooq.
	InsertInto(db.Book).
	Columns(db.Book.Title, db.Book.Price, db.Book.AuthorId).
	Values("Book One", 10.00, 1).
	Values("Book Two", 12.50, 1).
	Values("Book Three", 8.75, 2).
	Execute(ctx, conn)
```

### Assignments and default values

Instead of positional `Values`, you can set columns individually with `Set`,
or insert a row entirely from column defaults with `DefaultValues`:

```go
gooq.
	InsertInto(db.Book).
	Set(db.Book.Title.Set("Untitled")).
	Set(db.Book.Price.Set(0.0))

gooq.
	InsertInto(db.Book).
	DefaultValues()
```

### RETURNING

Append `Returning` to read columns back from the inserted rows. Support for
`RETURNING` is dialect-dependent; see [Dialects](/gooq/guides/dialects/).

```go
sql, args, err := gooq.
	InsertInto(db.Book).
	Columns(db.Book.Title, db.Book.Price).
	Values("New Title", 19.99).
	Returning(db.Book.Id, db.Book.Title).
	SQLFor(gooq.Postgres())
// INSERT INTO "book" ("title", "price") VALUES ($1, $2) RETURNING "id", "title"
```

## Upserts

An upsert resolves a unique-key conflict instead of failing. Both PostgreSQL and
SQLite express it with `ON CONFLICT`.

Use `OnConflict` with either `DoUpdateSet` or `DoNothing`. The helper
`gooq.SetToExcluded(field)` assigns a column to the value that would have been
inserted (the `excluded` pseudo-row):

```go
sql, args, err := gooq.
	InsertInto(db.Book).
	Columns(db.Book.Id, db.Book.Title, db.Book.Price).
	Values(1, "The Go Programming Language", 39.99).
	OnConflict(db.Book.Id).
	DoUpdateSet(
		gooq.SetToExcluded(db.Book.Title),
		gooq.SetToExcluded(db.Book.Price),
	).
	Returning(db.Book.Id).
	SQLFor(gooq.Postgres())
// INSERT INTO "book" ("id", "title", "price") VALUES ($1, $2, $3)
// ON CONFLICT ("id") DO UPDATE SET "title" = excluded."title", "price" = excluded."price"
// RETURNING "id"
```

To ignore conflicts entirely, use `DoNothing`:

```go
gooq.
	InsertInto(db.Book).
	Columns(db.Book.Id, db.Book.Title).
	Values(1, "Already Present").
	OnConflict(db.Book.Id).
	DoNothing()
```

## UPDATE

Build an update with `Update`, assign columns via the field `Set` method, and
constrain the rows with `Where`:

```go
result, err := gooq.
	Update(db.Book).
	Set(db.Book.Price.Set(24.99)).
	Where(db.Book.Id.EQ(42)).
	And(db.Book.Price.GT(24.99)).
	Execute(ctx, conn)
```

`Update` also supports `Returning` on dialects that allow it:

```go
sql, args, err := gooq.
	Update(db.Book).
	Set(db.Book.Price.Set(0.0)).
	Where(db.Book.AuthorId.EQ(7)).
	Returning(db.Book.Id, db.Book.Price).
	SQLFor(gooq.Postgres())
```

## DELETE

Delete rows with `DeleteFrom` and a `Where` predicate:

```go
result, err := gooq.
	DeleteFrom(db.Book).
	Where(db.Book.Price.LT(1.0)).
	Execute(ctx, conn)
```

`DeleteFrom` supports `Returning` as well, and can be rendered without
executing:

```go
sql, args, err := gooq.
	DeleteFrom(db.Book).
	Where(db.Book.Id.In(1, 2, 3)).
	Returning(db.Book.Id).
	SQL()
```

## Error values to expect

Write statements surface a few sentinel errors worth handling:

- `gooq.ErrEmptyInsert` — an INSERT was built with no rows.
- `gooq.ErrColumnValueMismatch` — the number of values does not match the
  number of columns.
- `gooq.ErrReturningUnsupported` — `Returning` was used on a dialect that does
  not support it.

These and the query-side error values are described on the
[Data Modification reference](/gooq/reference/dml/) page.
