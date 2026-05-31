---
title: Data Modification
description: INSERT, UPDATE, and DELETE in gooq, including upserts, RETURNING, INSERT...SELECT, and batch execution.
---

gooq provides typed builders for `INSERT`, `UPDATE`, and `DELETE`. Each builder
ends in a terminal method that renders SQL (`SQL`, `SQLFor`, `Using`) or executes
the statement (`Execute`). The statements that return rows expose `Returning`
together with the `ReturningInto`/`ReturningOneInto` mapping helpers documented in
[Fetching and mapping](/gooq/reference/fetching-and-mapping/).

## INSERT

Start with `InsertInto`, name the target `Columns`, and supply one or more
`Values` rows.

```go
result, err := gooq.InsertInto(db.Book).
    Columns(db.Book.AuthorId, db.Book.Title, db.Book.Price).
    Values("b0000000-0000-0000-0000-000000000001", "Go in Action", 39.99).
    Execute(ctx, conn)
```

```sql
INSERT INTO "book" ("author_id", "title", "price") VALUES ($1, $2, $3)
```

Multiple `Values` calls insert several rows in one statement:

```go
query := gooq.InsertInto(db.Book).
    Columns(db.Book.AuthorId, db.Book.Title).
    Values("b0000000-0000-0000-0000-000000000001", "First").
    Values("b0000000-0000-0000-0000-000000000001", "Second")
```

Alternatively, build a row with `Set` assignments, insert defaults with
`DefaultValues`, or insert the result of a subquery with `Select` (see
[INSERT ... SELECT](#insert--select) below).

```go
query := gooq.InsertInto(db.Author).
    Set(db.Author.Name.Set("Ada Lovelace")).
    Set(db.Author.Email.Set("ada@example.com"))
```

An insert with no columns or rows returns `gooq.ErrEmptyInsert`; a row whose
value count differs from the column count returns `gooq.ErrColumnValueMismatch`.

## ON CONFLICT (upsert)

`OnConflict` names the conflict-target columns. Follow it with `DoUpdateSet` to
update the existing row, or `DoNothing` to ignore the conflict. The shortcut
`OnConflictDoNothing` ignores any conflict without naming columns.

```go
query := gooq.InsertInto(db.Author).
    Columns(db.Author.Name, db.Author.Email).
    Values("Ada Lovelace", "ada@example.com").
    OnConflict(db.Author.Email).
    DoUpdateSet(
        db.Author.Name.Set("Ada Lovelace"),
    )
```

```sql
INSERT INTO "author" ("name", "email") VALUES ($1, $2)
ON CONFLICT ("email") DO UPDATE SET "name" = $3
```

To copy the rejected row's value into the update, use `SetToExcluded`, which
references the special excluded row. On PostgreSQL this renders as `EXCLUDED.`;
on SQLite it renders as `excluded.`.

```go
query := gooq.InsertInto(db.Author).
    Columns(db.Author.Name, db.Author.Email).
    Values("Ada Lovelace", "ada@example.com").
    OnConflict(db.Author.Email).
    DoUpdateSet(gooq.SetToExcluded(db.Author.Name))
```

```sql
INSERT INTO "author" ("name", "email") VALUES ($1, $2)
ON CONFLICT ("email") DO UPDATE SET "name" = EXCLUDED."name"
```

`DoNothing` (or `OnConflictDoNothing`) skips conflicting rows silently:

```go
query := gooq.InsertInto(db.Author).
    Columns(db.Author.Name, db.Author.Email).
    Values("Ada Lovelace", "ada@example.com").
    OnConflictDoNothing()
```

## RETURNING

`Returning` appends a `RETURNING` clause so the statement yields the affected
rows. Both PostgreSQL and SQLite render it natively. A call with no columns
renders `RETURNING *`.

```go
query := gooq.InsertInto(db.Book).
    Columns(db.Book.AuthorId, db.Book.Title).
    Values("b0000000-0000-0000-0000-000000000001", "Go in Action").
    Returning(db.Book.Id, db.Book.CreatedAt)
```

```sql
INSERT INTO "book" ("author_id", "title") VALUES ($1, $2) RETURNING "id", "created_at"
```

`RETURNING` columns are emitted unqualified. Map the returned rows with
`gooq.ReturningInto` or `gooq.ReturningOneInto`. The sentinel
`gooq.ErrReturningUnsupported` exists as a guard for any dialect that does not
support `RETURNING`; neither supported dialect triggers it.

## INSERT ... SELECT

`Select` inserts the rows produced by a subquery. The subquery's columns must
line up with the named `Columns`.

```go
source := gooq.Select2(db.Book.AuthorId, db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusOutOfPrint))

query := gooq.InsertInto(db.Book).
    Columns(db.Book.AuthorId, db.Book.Title).
    Select(source)
```

```sql
INSERT INTO "book" ("author_id", "title")
SELECT "book"."author_id", "book"."title" FROM "book" WHERE "book"."status" = $1
```

## UPDATE

`Update` targets a table, `Set` applies assignments, and `Where` scopes the
change. `From` adds a secondary table whose columns can drive the update
(`UPDATE ... FROM`), and `Returning` yields the modified rows.

```go
result, err := gooq.Update(db.Book).
    Set(db.Book.InPrint.Set(false)).
    Set(db.Book.Status.Set(db.BookStatusOutOfPrint)).
    Where(db.Book.PageCount.LT(int64(50))).
    Execute(ctx, conn)
```

```sql
UPDATE "book" SET "in_print" = $1, "status" = $2 WHERE "book"."page_count" < $3
```

An `UPDATE ... FROM` joins another table into the update:

```go
query := gooq.Update(db.Book).
    Set(db.Book.InPrint.Set(false)).
    From(db.Author).
    Where(db.Author.Id.EQField(db.Book.AuthorId)).
    And(db.Author.Name.EQ("Retired Author"))
```

```sql
UPDATE "book" SET "in_print" = $1 FROM "author"
WHERE "author"."id" = "book"."author_id" AND "author"."name" = $2
```

## DELETE

`DeleteFrom` targets a table and `Where` scopes the deletion. `UsingTable` adds a
table for the `DELETE ... USING` form, and `Returning` yields the deleted rows.

```go
result, err := gooq.DeleteFrom(db.Review).
    Where(db.Review.Rating.LT(int64(2))).
    Execute(ctx, conn)
```

```sql
DELETE FROM "review" WHERE "review"."rating" < $1
```

`DELETE ... USING` references another table in the predicate. The method is
`UsingTable`; note that `Using` selects the dialect and is unrelated. The `USING`
clause is rendered for PostgreSQL only; on SQLite it is omitted, so a portable
delete should express the relationship with a subquery instead.

```go
query := gooq.DeleteFrom(db.Review).
    UsingTable(db.Book).
    Where(db.Book.Id.EQField(db.Review.BookId)).
    And(db.Book.Status.EQ(db.BookStatusOutOfPrint)).
    Returning(db.Review.Id)
```

```sql
DELETE FROM "review" USING "book"
WHERE "book"."id" = "review"."book_id" AND "book"."status" = $1
RETURNING "id"
```

## Batch execution

`BatchExec` renders and executes several statements in order against a single
`Querier`, stopping at the first error. Pass the executable steps as variadic
arguments.

```go
err := gooq.BatchExec(ctx, conn,
    gooq.InsertInto(db.Author).
        Columns(db.Author.Name, db.Author.Email).
        Values("Ada Lovelace", "ada@example.com"),
    gooq.Update(db.Book).
        Set(db.Book.InPrint.Set(true)).
        Where(db.Book.Status.EQ(db.BookStatusInPrint)),
)
```

`BatchExec` is not transactional on its own. To run the statements atomically,
wrap the call in `WithTx`; see
[Fetching and mapping](/gooq/reference/fetching-and-mapping/).
