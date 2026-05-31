---
title: Querying Data
description: Recipes for pagination, dynamic filters, joins, grouping, window functions, subqueries, and set operations.
---

## Paginate a result set

You want to fetch one page of results in a stable order.

```go
const pageSize = int64(20)
page := int64(3)

rows, err := gooq.Select2(db.Book.Id, db.Book.Title).
    From(db.Book).
    OrderBy(db.Book.CreatedAt.Desc(), db.Book.Id.Asc()).
    Limit(pageSize).
    Offset((page - 1) * pageSize).
    Fetch(ctx, conn)
```

```sql
SELECT "book"."id", "book"."title" FROM "book"
ORDER BY "book"."created_at" DESC, "book"."id" ASC
LIMIT 20 OFFSET 40
```

Gotcha: always order by a unique tiebreaker, or rows can shift between pages.

## Build a filter dynamically with And/Or

You want to add conditions only when the caller supplied them.

```go
filter := gooq.True()
if onlyInPrint {
    filter = filter.And(db.Book.InPrint.EQ(true))
}
if minPages > 0 {
    filter = filter.And(db.Book.PageCount.GE(minPages))
}

rows, err := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(filter).
    Fetch(ctx, conn)
```

Gotcha: seed the filter with `gooq.True()` so the first `And` has something to
attach to even when no condition applies.

## Case-insensitive search with ILike

You want a case-insensitive title search on PostgreSQL.

```go
term := "%go%"

rows, err := gooq.Select2(db.Book.Id, db.Book.Title).
    From(db.Book).
    Where(db.Book.Title.ILike(term)).
    Fetch(ctx, conn)
```

```sql
SELECT "book"."id", "book"."title" FROM "book" WHERE "book"."title" ILIKE $1
```

Gotcha: `ILike` renders `ILIKE` on PostgreSQL and a plain `LIKE` on SQLite, whose
default collation is case-insensitive for ASCII text.

## Top-N rows per group with a window function

You want the single highest-priced book per author, computed with a ranking
window function applied inside a subquery.

```go
ranked := gooq.Select3(
    db.Book.AuthorId,
    db.Book.Title,
    gooq.RowNumber().
        Over().
        PartitionBy(db.Book.AuthorId).
        OrderBy(db.Book.Price.Desc()).
        End().
        As("rn"),
).
    From(db.Book)

sql, args, err := ranked.SQL()
```

```sql
SELECT "book"."author_id", "book"."title",
       ROW_NUMBER() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" DESC) AS "rn"
FROM "book"
```

Gotcha: a window function cannot be filtered in the same `WHERE`; project the rank
in an inner query, then filter `rn = 1` in an outer query over it.

## A grouped report with GROUP BY and HAVING

You want per-author book counts, keeping only prolific authors.

```go
rows, err := gooq.Select2(db.Book.AuthorId, gooq.Count(db.Book.Id)).
    From(db.Book).
    GroupBy(db.Book.AuthorId).
    Having(gooq.Count(db.Book.Id).GT(int64(3))).
    OrderBy(gooq.Count(db.Book.Id).Desc()).
    Fetch(ctx, conn)
```

```sql
SELECT "book"."author_id", COUNT("book"."id") FROM "book"
GROUP BY "book"."author_id"
HAVING COUNT("book"."id") > $1
ORDER BY COUNT("book"."id") DESC
```

Gotcha: filter aggregates in `Having`, not `Where`; `Where` runs before grouping.

## Bucket values with Coalesce and CASE

You want a price band, treating a missing price as zero.

```go
band := gooq.Case[string]().
    When(gooq.Coalesce(db.Book.Price, 0.0).LT(10.0), "budget").
    When(gooq.Coalesce(db.Book.Price, 0.0).LT(30.0), "standard").
    Else("premium").
    End()

rows, err := gooq.Select2(db.Book.Title, band.As("price_band")).
    From(db.Book).
    Fetch(ctx, conn)
```

```sql
SELECT "book"."title",
       CASE WHEN COALESCE("book"."price", $1) < $2 THEN $3
            WHEN COALESCE("book"."price", $4) < $5 THEN $6
            ELSE $7 END AS "price_band"
FROM "book"
```

Gotcha: close the `Case` builder with `.End()`; `When` and `Else` return the
builder, only `End` returns a usable field.

## Join three tables

You want each review with its book title and author name.

```go
rows, err := gooq.Select3(db.Author.Name, db.Book.Title, db.Review.Rating).
    From(db.Review).
    InnerJoin(db.Book).On(db.Book.Id.EQField(db.Review.BookId)).
    InnerJoin(db.Author).On(db.Author.Id.EQField(db.Book.AuthorId)).
    Fetch(ctx, conn)
```

```sql
SELECT "author"."name", "book"."title", "review"."rating"
FROM "review"
INNER JOIN "book" ON "book"."id" = "review"."book_id"
INNER JOIN "author" ON "author"."id" = "book"."author_id"
```

Gotcha: each `On` references only tables already in the chain, so order the joins
accordingly.

## Left join that keeps unmatched rows

You want every book, with a review rating where one exists.

```go
type Row struct {
    Title  string          `db:"title"`
    Rating sql.Null[int64] `db:"rating"`
}

rows, err := gooq.FetchInto[Row](ctx, conn,
    gooq.Select2(db.Book.Title, db.Review.Rating).
        From(db.Book).
        LeftJoin(db.Review).On(db.Review.BookId.EQField(db.Book.Id)),
)
```

Gotcha: columns from the left-joined table can be null, so map them to
`sql.Null[T]` or a pointer.

## Filter with EXISTS

You want books that have at least one review.

```go
sub := gooq.Select1(db.Review.Id).
    From(db.Review).
    Where(db.Review.BookId.EQField(db.Book.Id))

rows, err := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(gooq.Exists(sub)).
    Fetch(ctx, conn)
```

```sql
SELECT "book"."title" FROM "book"
WHERE EXISTS (SELECT "review"."id" FROM "review" WHERE "review"."book_id" = "book"."id")
```

Gotcha: correlate the subquery with `EQField` to the outer table, or `EXISTS`
matches every row.

## Filter with an IN subquery

You want authors who have published at least one book.

```go
authorsWithBooks := gooq.Select1(db.Book.AuthorId).From(db.Book)

rows, err := gooq.Select2(db.Author.Id, db.Author.Name).
    From(db.Author).
    Where(db.Author.Id.InSubquery(authorsWithBooks)).
    Fetch(ctx, conn)
```

```sql
SELECT "author"."id", "author"."name" FROM "author"
WHERE "author"."id" IN (SELECT "book"."author_id" FROM "book")
```

Gotcha: the subquery must project exactly one column whose type matches the outer
field.

## Combine results with UNION

You want draft and published titles in one list, without duplicates.

```go
drafts := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusDraft))

published := gooq.Select1(db.Book.Title).
    From(db.Book).
    Where(db.Book.Status.EQ(db.BookStatusInPrint))

rows, err := drafts.Union(published).Fetch(ctx, conn)
```

```sql
SELECT "book"."title" FROM "book" WHERE "book"."status" = $1
UNION
SELECT "book"."title" FROM "book" WHERE "book"."status" = $2
```

Gotcha: both sides must project the same column count and types; use `UnionAll`
when you want to keep duplicates.
