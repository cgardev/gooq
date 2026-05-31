---
title: Writing Data
description: Recipes for inserts, upserts, updates, deletes, transactions, and batch execution.
---

## Insert and read the new row back

You want to insert a row and get the database-assigned values.

```go
type BookRow struct {
    ID        string    `db:"id"`
    CreatedAt time.Time `db:"created_at"`
}

inserted, err := gooq.ReturningOneInto[BookRow](ctx, conn,
    gooq.InsertInto(db.Book).
        Columns(db.Book.AuthorId, db.Book.Title).
        Values("b0000000-0000-0000-0000-000000000001", "Go in Action").
        Returning(db.Book.Id, db.Book.CreatedAt),
)
```

```sql
INSERT INTO "book" ("author_id", "title") VALUES ($1, $2)
RETURNING "id", "created_at"
```

Gotcha: `RETURNING` works on both PostgreSQL and SQLite, and the returned columns
are emitted unqualified, so match the struct `db` tags to the bare column names.

## Bulk insert many rows

You want to insert several rows in one statement.

```go
result, err := gooq.InsertInto(db.Book).
    Columns(db.Book.AuthorId, db.Book.Title).
    Values("b0000000-0000-0000-0000-000000000001", "First").
    Values("b0000000-0000-0000-0000-000000000001", "Second").
    Values("b0000000-0000-0000-0000-000000000001", "Third").
    Execute(ctx, conn)
```

```sql
INSERT INTO "book" ("author_id", "title") VALUES ($1, $2), ($3, $4), ($5, $6)
```

Gotcha: every `Values` row must match the `Columns` count, or rendering returns
`gooq.ErrColumnValueMismatch`.

## Upsert with DoUpdateSet and SetToExcluded

You want to insert an author, updating the existing row on an email conflict.

```go
result, err := gooq.InsertInto(db.Author).
    Columns(db.Author.Name, db.Author.Email).
    Values("Ada Lovelace", "ada@example.com").
    OnConflict(db.Author.Email).
    DoUpdateSet(gooq.SetToExcluded(db.Author.Name)).
    Execute(ctx, conn)
```

```sql
INSERT INTO "author" ("name", "email") VALUES ($1, $2)
ON CONFLICT ("email") DO UPDATE SET "name" = EXCLUDED."name"
```

Gotcha: the `OnConflict` columns must be backed by a unique or primary key
constraint; `SetToExcluded` renders `EXCLUDED.` on PostgreSQL and `excluded.` on
SQLite automatically.

## Insert, ignoring conflicts

You want to insert only when the row does not already exist.

```go
result, err := gooq.InsertInto(db.Author).
    Columns(db.Author.Name, db.Author.Email).
    Values("Ada Lovelace", "ada@example.com").
    OnConflictDoNothing().
    Execute(ctx, conn)
```

```sql
INSERT INTO "author" ("name", "email") VALUES ($1, $2) ON CONFLICT DO NOTHING
```

Gotcha: a skipped insert reports zero affected rows, so do not treat that as an
error.

## Copy rows with INSERT ... SELECT

You want to clone out-of-print books as new entries.

```go
source := gooq.Select2(db.Book.AuthorId, db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusOutOfPrint))

result, err := gooq.InsertInto(db.Book).
    Columns(db.Book.AuthorId, db.Book.Title).
    Select(source).
    Execute(ctx, conn)
```

```sql
INSERT INTO "book" ("author_id", "title")
SELECT "book"."author_id", "book"."title" FROM "book" WHERE "book"."status" = $1
```

Gotcha: the subquery's projected columns must line up, in order, with the named
`Columns`.

## Update and read the changed rows back

You want to update rows and see the result.

```go
type BookRow struct {
    ID     string `db:"id"`
    Status string `db:"status"`
}

updated, err := gooq.ReturningInto[BookRow](ctx, conn,
    gooq.Update(db.Book).
        Set(db.Book.Status.Set(db.BookStatusOutOfPrint)).
        Set(db.Book.InPrint.Set(false)).
        Where(db.Book.PageCount.LT(int64(50))).
        Returning(db.Book.Id, db.Book.Status),
)
```

```sql
UPDATE "book" SET "status" = $1, "in_print" = $2 WHERE "book"."page_count" < $3
RETURNING "id", "status"
```

Gotcha: chain a `Set` call per column; there is no combined multi-column setter.

## Apply a conditional update

You want to clear the price only on expensive in-print books.

```go
result, err := gooq.Update(db.Book).
    Set(db.Book.Price.Set(0.0)).
    Where(db.Book.InPrint.EQ(true)).
    And(db.Book.Price.GT(100.0)).
    Execute(ctx, conn)
```

```sql
UPDATE "book" SET "price" = $1 WHERE "book"."in_print" = $2 AND "book"."price" > $3
```

Gotcha: an `UPDATE` without `Where` rewrites every row; always scope it.

## Delete and return the removed rows

You want to delete low-rated reviews and log what was removed.

```go
type ReviewRow struct {
    ID       string `db:"id"`
    Reviewer string `db:"reviewer"`
}

removed, err := gooq.ReturningInto[ReviewRow](ctx, conn,
    gooq.DeleteFrom(db.Review).
        Where(db.Review.Rating.LT(int64(2))).
        Returning(db.Review.Id, db.Review.Reviewer),
)
```

```sql
DELETE FROM "review" WHERE "review"."rating" < $1 RETURNING "id", "reviewer"
```

Gotcha: for `DELETE ... USING`, reference the other table with `UsingTable`;
`Using` selects the dialect, and the `USING` clause renders for PostgreSQL only.

## Run several statements in a transaction

You want two writes to commit or roll back together.

```go
err := gooq.WithTx(ctx, conn, func(tx *sql.Tx) error {
    author, err := gooq.ReturningOneInto[struct {
        ID string `db:"id"`
    }](ctx, tx,
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

Gotcha: returning an error rolls everything back; pass `tx` (not `conn`) to every
statement inside the function, and note that `WithTx` takes a `*sql.DB`.

## Batch several statements

You want to run a fixed sequence of statements on one connection.

```go
results, err := gooq.BatchExec(ctx, conn,
    gooq.InsertInto(db.Author).
        Columns(db.Author.Name, db.Author.Email).
        Values("Ada Lovelace", "ada@example.com"),
    gooq.Update(db.Book).
        Set(db.Book.InPrint.Set(true)).
        Where(db.Book.Status.EQ(db.BookStatusInPrint)),
)
```

Gotcha: `BatchExec` is not transactional on its own and stops at the first error;
pass a `*sql.Tx` as the `Querier` (via `WithTx`) when the steps must be atomic.
