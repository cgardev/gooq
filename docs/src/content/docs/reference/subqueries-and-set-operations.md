---
title: Subqueries and Set Operations
description: EXISTS, IN subqueries, scalar subqueries, and UNION, INTERSECT, and EXCEPT in gooq.
---

gooq lets a `SELECT` appear inside another query as a predicate, a value, or as
one side of a set operation. A select builder satisfies the `gooq.Subquery`
interface directly, so it can be passed straight to `Exists`, `InSubquery`, and
`ScalarSubquery` without any conversion call.

## EXISTS and NOT EXISTS

`Exists` and `NotExists` wrap a subquery as a condition. Correlate the subquery
to the outer query through its `Where` clause.

```go
sub := gooq.Select1(db.Review.Id).
    From(db.Review).
    Where(db.Review.BookId.EQField(db.Book.Id))

query := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(gooq.Exists(sub))
```

```sql
SELECT "book"."title"
FROM "book"
WHERE EXISTS (SELECT "review"."id" FROM "review" WHERE "review"."book_id" = "book"."id")
```

`NotExists` produces `NOT EXISTS (...)`, for example to find books without any
reviews.

## IN subqueries

`InSubquery` and `NotInSubquery` compare a field against the single-column result
of a subquery.

```go
prolific := gooq.Select1(db.Book.AuthorId).
    From(db.Book).
    GroupBy(db.Book.AuthorId).
    Having(gooq.Count(db.Book.Id).GT(int64(5)))

query := gooq.Select2(db.Author.Id, db.Author.Name).
    From(db.Author).
    Where(db.Author.Id.InSubquery(prolific))
```

```sql
SELECT "author"."id", "author"."name"
FROM "author"
WHERE "author"."id" IN (
    SELECT "book"."author_id" FROM "book" GROUP BY "book"."author_id" HAVING COUNT("book"."id") > $1
)
```

## Scalar subqueries

`ScalarSubquery[T]` turns a single-row, single-column subquery into a typed
field. The type parameter is the column type of the subquery result.

```go
sub := gooq.Select1(gooq.Max[int64](db.Book.PageCount)).
    From(db.Book)

query := gooq.Select2(
    db.Book.Title,
    gooq.ScalarSubquery[int64](sub).As("max_pages"),
).
    From(db.Book)
```

```sql
SELECT "book"."title", (SELECT MAX("book"."page_count") FROM "book") AS "max_pages"
FROM "book"
```

A scalar subquery is an ordinary field, so it can also appear in a `Where` clause
or anywhere else a `Field[T]` is accepted.

## Set operations

`Union`, `UnionAll`, `Intersect`, and `Except` combine two complete `SELECT`
statements. Both sides must project the same number and types of columns, and the
combined result preserves that row type `R`. Apply the operation to a final
select step, passing the other query.

```go
drafts := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusDraft))

published := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusInPrint))

combined := drafts.Union(published)
```

```sql
SELECT "book"."title" FROM "book" WHERE "book"."status" = $1
UNION
SELECT "book"."title" FROM "book" WHERE "book"."status" = $2
```

`UnionAll` keeps duplicate rows, `Intersect` keeps rows present in both sides, and
`Except` keeps rows in the first side that are absent from the second. The
combined query exposes the usual terminal methods such as `Fetch` and `SQLFor`.
