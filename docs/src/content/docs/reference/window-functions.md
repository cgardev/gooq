---
title: Window Functions
description: Ranking and analytic functions in gooq, using the Over, PartitionBy, and OrderBy builder.
---

Window functions compute a value across a set of rows related to the current row
without collapsing them as `GROUP BY` does. In gooq each window function returns
a `WindowField`, which exposes an `Over` builder for the `OVER (...)` clause.

## The OVER builder

Call `Over` on a window field, then optionally `PartitionBy` with the partition
columns and `OrderBy` with order expressions, and close with `End`. `End`
returns a `Field[T]` usable in a projection.

```go
ranked := gooq.RowNumber().
    Over().
    PartitionBy(db.Book.AuthorId).
    OrderBy(db.Book.Price.Desc()).
    End()
```

```sql
ROW_NUMBER() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" DESC)
```

Both `PartitionBy` and `OrderBy` are optional. Omit `PartitionBy` to compute over
the whole result set; omit `OrderBy` for unordered windows.

## Ranking functions

`RowNumber`, `Rank`, and `DenseRank` all return `WindowField[int64]`.

```go
gooq.RowNumber() // ROW_NUMBER()
gooq.Rank()      // RANK()
gooq.DenseRank() // DENSE_RANK()
```

A typical "rank rows within a group" query projects the rank alongside the row:

```go
query := gooq.Select3(
    db.Book.AuthorId,
    db.Book.Title,
    gooq.Rank().
        Over().
        PartitionBy(db.Book.AuthorId).
        OrderBy(db.Book.Price.Desc()).
        End().
        As("price_rank"),
).
    From(db.Book)
```

```sql
SELECT "book"."author_id", "book"."title",
       RANK() OVER (PARTITION BY "book"."author_id" ORDER BY "book"."price" DESC) AS "price_rank"
FROM "book"
```

To return only the top rows per group, wrap the ranked query as a subquery and
filter on the rank; see
[Subqueries](/gooq/reference/subqueries-and-set-operations/).

## Offset and value functions

`Lead`, `Lag`, `FirstValue`, and `LastValue` are generic over the column type.

```go
gooq.Lead[float64](db.Book.Price)       // LEAD("book"."price")
gooq.Lag[float64](db.Book.Price)        // LAG("book"."price")
gooq.FirstValue[float64](db.Book.Price) // FIRST_VALUE("book"."price")
gooq.LastValue[float64](db.Book.Price)  // LAST_VALUE("book"."price")
```

```go
prev := gooq.Lag[float64](db.Book.Price).
    Over().
    PartitionBy(db.Book.AuthorId).
    OrderBy(db.Book.PublishedAt.Asc()).
    End()

query := gooq.Select2(db.Book.Title, prev.As("previous_price")).
    From(db.Book)
```

```sql
SELECT "book"."title",
       LAG("book"."price") OVER (PARTITION BY "book"."author_id" ORDER BY "book"."published_at" ASC) AS "previous_price"
FROM "book"
```

## Aggregate window functions

`SumOver`, `AvgOver`, and `CountOver` apply an aggregate as a window function.
`SumOver` and `AvgOver` are generic over the numeric column type; `CountOver`
renders `COUNT(*)` and returns `WindowField[int64]`.

```go
runningTotal := gooq.SumOver[float64](db.Book.Price).
    Over().
    PartitionBy(db.Book.AuthorId).
    OrderBy(db.Book.PublishedAt.Asc()).
    End()

query := gooq.Select2(db.Book.Title, runningTotal.As("running_total")).
    From(db.Book)
```

```sql
SELECT "book"."title",
       SUM("book"."price") OVER (PARTITION BY "book"."author_id" ORDER BY "book"."published_at" ASC) AS "running_total"
FROM "book"
```

`CountOver()` is useful for adding a total-rows column to each row of a result.
