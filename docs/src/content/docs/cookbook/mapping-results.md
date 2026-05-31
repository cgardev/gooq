---
title: Mapping Results
description: Recipes for mapping rows into structs, handling nulls, keying and grouping, and reusing conditions.
---

## Map rows into a struct with db tags

You want fetched rows as a slice of structs.

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

Gotcha: a `db` tag matches a selected column by name; an untagged field falls back
to its case-insensitive Go name, and unmatched columns are skipped.

## Handle nullable columns

You want to read a column that may be null.

```go
type BookRow struct {
    Title    string           `db:"title"`
    Subtitle sql.Null[string] `db:"subtitle"`
}

books, err := gooq.FetchInto[BookRow](ctx, conn,
    gooq.Select2(db.Book.Title, db.Book.Subtitle).
        From(db.Book),
)
```

Gotcha: scanning a null into a plain `string` fails; use `sql.Null[T]` or a
pointer for nullable columns such as `subtitle` and `editor_id`.

## Index rows by key with FetchMap

You want each row keyed by its identifier.

```go
type BookRow struct {
    ID    string `db:"id"`
    Title string `db:"title"`
}

byID, err := gooq.FetchMap[string, BookRow](ctx, conn,
    gooq.Select2(db.Book.Id, db.Book.Title).
        From(db.Book),
    "id",
)
```

Gotcha: the key column name is the SQL column ("id"), and on a duplicate key the
later row overwrites the earlier one.

## Group rows by key with FetchGroups

You want books grouped under their author.

```go
type BookRow struct {
    AuthorID string `db:"author_id"`
    Title    string `db:"title"`
}

byAuthor, err := gooq.FetchGroups[string, BookRow](ctx, conn,
    gooq.Select2(db.Book.AuthorId, db.Book.Title).
        From(db.Book),
    "author_id",
)
```

Gotcha: `FetchGroups` returns `map[K][]S` for one-to-many data; `FetchMap` would
keep only the last row per key.

## Require exactly one row, or allow none

You want to load a single record and react to its absence.

```go
title, err := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Id.EQ("b0000000-0000-0000-0000-000000000001")).
    FetchSingle(ctx, conn)
if errors.Is(err, sql.ErrNoRows) {
    // no such book
}

// FetchOptional reports absence through its found return instead of an error.
maybe, found, err := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusDraft)).
    FetchOptional(ctx, conn)
_ = maybe
_ = found
```

Gotcha: `FetchSingle` returns `sql.ErrNoRows` when empty and
`gooq.ErrTooManyRows` when more than one row matches; `FetchOptional` returns
`(row, found, error)` so zero rows is not an error.

## Reuse a stored Condition across queries

You want to apply the same filter to several queries.

```go
activeBooks := db.Book.InPrint.EQ(true).
    And(db.Book.Status.EQ(db.BookStatusInPrint))

count, err := gooq.Select1(gooq.Count(db.Book.Id)).
    From(db.Book).
    Where(activeBooks).
    FetchOne(ctx, conn)

titles, err := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(activeBooks).
    Fetch(ctx, conn)

_ = count
_ = titles
```

Gotcha: a `Condition` is an ordinary value with no mutable per-query state, so the
same one can be reused safely across queries.
