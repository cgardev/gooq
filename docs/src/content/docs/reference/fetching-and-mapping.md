---
title: Fetching and Mapping
description: Terminal fetch methods, struct mapping helpers, transactions, and the Querier interface in gooq.
---

Once a query is built it must be turned into SQL or executed. This page covers the
terminal methods, the helpers that map rows into Go structs, and how to run work
inside a transaction.

## The Querier interface

Fetch and execute methods accept a `gooq.Querier`, which is satisfied by both
`*sql.DB` and `*sql.Tx`. The same query therefore runs unchanged on a connection
pool or inside a transaction.

## Rendering SQL

`SQL` renders for the default dialect; `SQLFor` renders for a specific dialect.
Both return `(string, []any, error)`.

```go
sqlText, args, err := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(db.Book.InPrint.EQ(true)).
    SQLFor(gooq.Postgres())
```

`Using(dialect)` binds a dialect to the query before a fetch, so the fetch renders
for that dialect rather than the default. See
[Dialects](/gooq/reference/dialects/).

## Fetching rows directly

The `SELECT` terminals return scanned values typed by the projected columns:

- `Fetch(ctx, conn)` returns all rows.
- `FetchOne(ctx, conn)` returns the first row, or `sql.ErrNoRows` when empty.
- `FetchSingle(ctx, conn)` requires exactly one row. It returns `sql.ErrNoRows`
  when the result is empty and `gooq.ErrTooManyRows` when more than one row
  matches.
- `FetchOptional(ctx, conn)` returns `(row, found, error)`, where `found` is
  false when no row matched and the error is `gooq.ErrTooManyRows` when more than
  one matched.

```go
rows, err := gooq.Select2(db.Book.Id, db.Book.Title).
    From(db.Book).
    Where(db.Book.InPrint.EQ(true)).
    Fetch(ctx, conn)
```

## Mapping into structs

The `FetchInto` family maps rows into Go structs by matching `db` struct tags to
the selected columns. Columns without a matching field are ignored, and fields
without a matching column keep their zero value. Each helper takes the context, a
`Querier`, and the query; the map and group helpers also take the key column
name.

`FetchInto[S]` returns a slice of structs:

```go
type BookRow struct {
    ID    string  `db:"id"`
    Title string  `db:"title"`
    Price float64 `db:"price"`
}

books, err := gooq.FetchInto[BookRow](ctx, conn,
    gooq.Select3(db.Book.Id, db.Book.Title, db.Book.Price).
        From(db.Book),
)
```

`FetchOneInto[S]` returns a single struct (the first row), or `sql.ErrNoRows`
when the result is empty.

```go
book, err := gooq.FetchOneInto[BookRow](ctx, conn,
    gooq.Select3(db.Book.Id, db.Book.Title, db.Book.Price).
        From(db.Book).
        Where(db.Book.Id.EQ("b0000000-0000-0000-0000-000000000001")),
)
```

### Nullable columns

Map nullable columns to `sql.Null[T]` (or a pointer) so absent values scan
cleanly. The `Subtitle` column, for example, is `Field[sql.Null[string]]`:

```go
type BookRow struct {
    Title    string           `db:"title"`
    Subtitle sql.Null[string] `db:"subtitle"`
}
```

### FetchMap and FetchGroups

`FetchMap[K, S]` keys each row struct by the value of one column, producing a
`map[K]S`. The key column name is passed as the final argument.

```go
byID, err := gooq.FetchMap[string, BookRow](ctx, conn,
    gooq.Select3(db.Book.Id, db.Book.Title, db.Book.Price).
        From(db.Book),
    "id",
)
```

`FetchGroups[K, S]` groups rows under a shared key, producing a `map[K][]S`,
which is useful for one-to-many relationships.

```go
byAuthor, err := gooq.FetchGroups[string, BookRow](ctx, conn,
    gooq.Select3(db.Book.AuthorId, db.Book.Title, db.Book.Price).
        From(db.Book),
    "author_id",
)
```

## Mapping RETURNING rows

For statements with a `Returning` clause, `ReturningInto[S]` maps all returned
rows into a slice and `ReturningOneInto[S]` maps a single returned row.

```go
inserted, err := gooq.ReturningOneInto[BookRow](ctx, conn,
    gooq.InsertInto(db.Book).
        Columns(db.Book.AuthorId, db.Book.Title).
        Values("b0000000-0000-0000-0000-000000000001", "Go in Action").
        Returning(db.Book.Id, db.Book.Title, db.Book.Price),
)
```

## Transactions

`WithTx` begins a transaction on a `*sql.DB`, runs the supplied function, and
commits if it returns nil or rolls back if it returns an error (or panics). The
`*sql.Tx` it passes satisfies `Querier`, so every query and statement runs
unchanged inside the transaction.

```go
err := gooq.WithTx(ctx, conn, func(tx *sql.Tx) error {
    author, err := gooq.ReturningOneInto[AuthorRow](ctx, tx,
        gooq.InsertInto(db.Author).
            Columns(db.Author.Name, db.Author.Email).
            Values("Ada Lovelace", "ada@example.com").
            Returning(db.Author.Id),
    )
    if err != nil {
        return err
    }

    _, err = gooq.InsertInto(db.Book).
        Columns(db.Book.AuthorId, db.Book.Title).
        Values(author.ID, "Go in Action").
        Execute(ctx, tx)
    return err
})
```

Returning an error from the function rolls the whole transaction back, so partial
writes never become visible. Note that `WithTx` requires a `*sql.DB`
specifically, since it begins the transaction itself.
