---
title: Window Functions
description: Ranking and offset window functions — RowNumber, Rank, DenseRank, Lead, Lag, FirstValue, LastValue, and windowed aggregates — built with the Over/PartitionBy/OrderBy/End builder.
---

Window functions compute a value across a set of rows related to the current
row without collapsing them into a group. In gooq each window function is a
package-level function that returns a `WindowField[T]`. The `WindowField` is not
yet a usable column: it must be completed with an `OVER` clause through the
`Over().PartitionBy(...).OrderBy(...).End()` builder chain, which produces a
regular `Field[T]`.

## The OVER builder

`Over()` begins the `OVER` clause and returns an `OverBuilder[T]`. `PartitionBy`
and `OrderBy` are both optional, and `End()` closes the clause and yields the
`Field[T]` that can appear in a projection like any other column.

```go
rn := gooq.RowNumber().
	Over().
	PartitionBy(db.Book.AuthorId).
	OrderBy(db.Book.Price.Desc()).
	End()

rows, err := gooq.
	Select2(db.Book.Title, rn.As("rn")).
	From(db.Book).
	Fetch(ctx, conn)
```

The window field is aliased with `As` and placed in the projection, so the
result is a `Record2[string, int64]` whose `.F2` holds the row number:

```sql
SELECT "title", ROW_NUMBER() OVER (PARTITION BY "author_id" ORDER BY "price" DESC) AS "rn" FROM "book"
```

`PartitionBy` accepts any number of columns, and `OrderBy` accepts the same
`OrderField` terms as a top-level `OrderBy`, so `Asc`, `Desc`, `NullsFirst`, and
`NullsLast` all apply inside the window. Both clauses may be omitted: a bare
`Over().End()` produces an empty `OVER ()` that ranks or aggregates across the
entire result set.

## Ranking functions

`RowNumber`, `Rank`, and `DenseRank` assign an ordinal to each row within its
partition. All three return `WindowField[int64]`.

```go
rank := gooq.Rank().
	Over().
	PartitionBy(db.Book.AuthorId).
	OrderBy(db.Book.Price.Desc()).
	End()
```

`RowNumber` numbers rows sequentially starting at one. `Rank` assigns the same
rank to ties and leaves gaps afterward, while `DenseRank` assigns the same rank
to ties without leaving gaps.

## Offset functions

`Lead` and `Lag` read a value from a row at a fixed offset from the current row
within the window, which is useful for period-over-period comparisons.
`FirstValue` and `LastValue` read the value from the first or last row of the
window frame. All four are generic and preserve the source field type, returning
`WindowField[T]`.

```go
previousPrice := gooq.Lag(db.Book.Price).
	Over().
	PartitionBy(db.Book.AuthorId).
	OrderBy(db.Book.PublishedAt.Asc()).
	End()
```

```sql
LAG("price") OVER (PARTITION BY "author_id" ORDER BY "published_at" ASC)
```

## Windowed aggregates

`SumOver`, `AvgOver`, and `CountOver` apply an aggregate over the window rather
than collapsing rows into a `GROUP BY`. `SumOver` preserves the field type,
`AvgOver` returns `WindowField[float64]`, and `CountOver` returns
`WindowField[int64]`.

```go
runningTotal := gooq.SumOver(db.Book.Price).
	Over().
	PartitionBy(db.Book.AuthorId).
	OrderBy(db.Book.PublishedAt.Asc()).
	End()
```

## Reference

| Function | SQL | Returns |
| --- | --- | --- |
| `RowNumber()` | `ROW_NUMBER()` | `WindowField[int64]` |
| `Rank()` | `RANK()` | `WindowField[int64]` |
| `DenseRank()` | `DENSE_RANK()` | `WindowField[int64]` |
| `Lead(f)` | `LEAD(f)` | `WindowField[T]` |
| `Lag(f)` | `LAG(f)` | `WindowField[T]` |
| `FirstValue(f)` | `FIRST_VALUE(f)` | `WindowField[T]` |
| `LastValue(f)` | `LAST_VALUE(f)` | `WindowField[T]` |
| `SumOver(f)` | `SUM(f) OVER (...)` | `WindowField[T]` |
| `AvgOver(f)` | `AVG(f) OVER (...)` | `WindowField[float64]` |
| `CountOver(f)` | `COUNT(f) OVER (...)` | `WindowField[int64]` |

Window functions render identically for the PostgreSQL and SQLite dialects;
both support the `OVER` clause natively. See the
[Dialects reference](/gooq/reference/dialects/) for the differences that do
apply between the two.
