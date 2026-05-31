---
title: Functions, Aggregates, CASE and CAST
description: Scalar functions, aggregate functions, conditional expressions, and type casts in gooq.
---

gooq exposes SQL functions as typed free functions that return fields. Because
they return fields, their results can be projected, aliased, compared, or nested
inside other expressions.

## Scalar functions

```go
gooq.Upper(db.Author.Name)   // UPPER("author"."name")
gooq.Lower(db.Author.Email)  // LOWER("author"."email")
gooq.Length(db.Book.Title)   // LENGTH("book"."title")
gooq.Trim(db.Author.Name)    // TRIM("author"."name")
gooq.Abs(db.Book.Price)      // ABS("book"."price")
gooq.Round(db.Book.Price)    // ROUND("book"."price")
```

`Upper`, `Lower`, and `Trim` return a `StringField`; `Length` returns a
`NumericField[int64]`. `Abs` and `Round` are generic over the numeric type, for
example `gooq.Abs[float64]`; the type parameter is usually inferred from the
field.

For any function not covered by a dedicated helper, `Function[T]` builds a typed
call:

```go
coalesced := gooq.Function[string]("COALESCE", db.Book.Subtitle, db.Book.Title)
```

## Aggregate functions

Aggregates are typically paired with `GroupBy`, but they may also appear without
grouping to aggregate the whole result.

```go
gooq.Count(db.Book.Id)               // COUNT("book"."id")
gooq.CountStar()                     // COUNT(*)
gooq.CountDistinct(db.Book.AuthorId) // COUNT(DISTINCT "book"."author_id")
gooq.Sum(db.Book.Price)              // SUM("book"."price")
gooq.Avg(db.Book.Price)              // AVG("book"."price")
gooq.Min(db.Book.PageCount)          // MIN("book"."page_count")
gooq.Max(db.Book.PageCount)          // MAX("book"."page_count")
```

`Count`, `CountStar`, and `CountDistinct` return `NumericField[int64]`. `Sum` and
`Avg` are generic over the numeric result type; `Min` and `Max` are generic over
any type. A grouped report combines several:

```go
query := gooq.Select4(
    db.Book.AuthorId,
    gooq.Count(db.Book.Id),
    gooq.Avg[float64](db.Book.Price),
    gooq.Max[int64](db.Book.PageCount),
).
    From(db.Book).
    GroupBy(db.Book.AuthorId).
    Having(gooq.Count(db.Book.Id).GT(int64(1)))
```

```sql
SELECT "book"."author_id", COUNT("book"."id"), AVG("book"."price"), MAX("book"."page_count")
FROM "book"
GROUP BY "book"."author_id"
HAVING COUNT("book"."id") > $1
```

## Null-handling helpers

```go
gooq.Coalesce(db.Book.Subtitle, db.Book.Title) // first arg field, rest values or fields
gooq.NullIf(db.Book.PageCount, int64(0))       // second arg is a value
gooq.Greatest(db.Book.PageCount, db.Review.Rating)
gooq.Least(db.Book.PageCount, int64(0))
```

```sql
COALESCE("book"."subtitle", "book"."title")
NULLIF("book"."page_count", $1)
```

All four are generic over the result type `T`, usually inferred from the first
field argument. `Coalesce`, `Greatest`, and `Least` take a leading `Field[T]`
followed by further operands that may be values (which bind) or fields (which
render as identifiers). `NullIf` takes a field and a value of type `T`.

## CASE expressions

`Case[T]` opens a builder. Add `When(condition, value)` branches (or
`WhenField(condition, field)` to return a column), an optional `Else(value)`, and
always finish with `End`, which yields the typed `Field[T]`. `When`, `WhenField`,
and `Else` each return the builder; only `End` returns a field.

```go
bucket := gooq.Case[string]().
    When(db.Book.Price.LT(10.0), "budget").
    When(db.Book.Price.LT(30.0), "standard").
    Else("premium").
    End()

query := gooq.Select2(db.Book.Title, bucket.As("price_band")).
    From(db.Book)
```

```sql
SELECT "book"."title",
       CASE WHEN "book"."price" < $1 THEN $2
            WHEN "book"."price" < $3 THEN $4
            ELSE $5 END AS "price_band"
FROM "book"
```

Use `WhenField` when a branch should return the value of another column rather
than a literal, and omit `Else` when no default is needed (a CASE with no matching
branch and no ELSE yields SQL NULL).

## CAST

`Cast[T]` converts a field to a SQL type. The Go type parameter is required and
the SQL type name is passed as a string.

```go
asBigint := gooq.Cast[int64](db.Book.PageCount, "bigint")

query := gooq.Select1(asBigint).
    From(db.Book)
```

```sql
SELECT CAST("book"."page_count" AS bigint) FROM "book"
```

Choose a SQL type name that the target dialect understands; for example use
`integer` rather than a PostgreSQL-only type when the query must also run on
SQLite.
