---
title: Expressions and Operators
description: Field comparisons, arithmetic, string predicates, and boolean combinators in gooq.
---

Conditions and value expressions are built from methods on the generated fields.
Every field is a `Field[T]`; string and numeric fields add extra methods. This
page lists the operators that produce conditions and the helpers that combine
them.

## Comparisons against values

Each `Field[T]` exposes the standard comparison operators against a value of its
type. Each returns a `Condition`.

```go
db.Book.PageCount.EQ(int64(300)) // "book"."page_count" = $1
db.Book.PageCount.NE(int64(0))   // "book"."page_count" <> $1
db.Book.Price.GT(9.99)           // "book"."price" > $1
db.Book.Price.LT(50.0)           // "book"."price" < $1
db.Book.Price.GE(5.0)            // "book"."price" >= $1
db.Book.Price.LE(100.0)          // "book"."price" <= $1
```

## Comparisons against another field

The `*Field` variants compare two columns instead of a column and a value.

```go
db.Author.Id.EQField(db.Book.AuthorId) // "author"."id" = "book"."author_id"
db.Book.PageCount.GTField(db.Review.Rating)
```

The full set is `EQField`, `NEField`, `GTField`, `LTField`, `GEField`, and
`LEField`.

## IN, BETWEEN, and NULL checks

```go
db.Book.Status.In(db.BookStatusDraft, db.BookStatusInPrint)
db.Book.Status.NotIn(db.BookStatusOutOfPrint)
db.Book.PageCount.Between(int64(100), int64(500))
db.Book.PageCount.NotBetween(int64(0), int64(10))
db.Book.Subtitle.IsNull()
db.Book.Subtitle.IsNotNull()
```

```sql
"book"."status" IN ($1, $2)
"book"."page_count" BETWEEN $1 AND $2
"book"."subtitle" IS NULL
```

`InSubquery` and `NotInSubquery` compare against a subquery rather than a value
list; see [Subqueries](/gooq/reference/subqueries-and-set-operations/).

## String predicates

`StringField` adds pattern matching and concatenation. `Like` and `NotLike` are
case-sensitive; `ILike` is the PostgreSQL case-insensitive variant.

```go
db.Author.Name.Like("Go%")
db.Author.Name.NotLike("%draft%")
db.Author.Name.ILike("%go%")
db.Author.Name.Concat(db.Book.Title)
```

```sql
"author"."name" LIKE $1
"author"."name" ILIKE $1
"author"."name" || "book"."title"
```

## Numeric arithmetic

`NumericField[T]` supports arithmetic both against scalar values and against
other numeric fields. The value forms are `Add`, `Sub`, `Mul`, `Div`, and
`ModVal`; the field forms are `AddField`, `SubField`, `MulField`, `DivField`, and
`ModField`. `Neg` negates the field.

```go
db.Book.Price.Mul(1.2)                       // "book"."price" * $1
db.Book.PageCount.AddField(db.Review.Rating) // "book"."page_count" + "review"."rating"
db.Book.PageCount.ModVal(int64(2))           // "book"."page_count" % $1
db.Book.Price.Neg()                          // -"book"."price"
```

Arithmetic expressions are themselves numeric fields, so they can be projected,
aliased, or compared:

```go
query := gooq.Select1(db.Book.Price.Mul(1.2).As("price_with_tax")).
    From(db.Book).
    Where(db.Book.Price.Mul(1.2).GT(100.0))
```

```sql
SELECT "book"."price" * $1 AS "price_with_tax"
FROM "book"
WHERE "book"."price" * $2 > $3
```

## Combining conditions

Conditions compose in two ways. On the builder, chain `And` and `Or` after
`Where`. On a `Condition` value, use the `And`, `Or`, and `Not` methods to build
nested logic, or the free functions for the same.

```go
// Method form on a Condition.
recent := db.Book.PublishedAt.IsNotNull().
    And(db.Book.InPrint.EQ(true))

// Free-function form, useful for nesting and dynamic assembly.
filter := gooq.And(
    db.Book.Status.EQ(db.BookStatusInPrint),
    gooq.Or(
        db.Book.Price.LT(10.0),
        db.Book.PageCount.GT(int64(500)),
    ),
)

query := gooq.Select1(db.Book.Id).
    From(db.Book).
    Where(filter)
```

```sql
SELECT "book"."id" FROM "book"
WHERE "book"."status" = $1 AND ("book"."price" < $2 OR "book"."page_count" > $3)
```

`gooq.Not(cond)` negates a condition; `gooq.True()` and `gooq.False()` are
constant conditions, handy as the seed of a dynamically built filter. Calling
`gooq.And()` or `gooq.Or()` with no arguments returns `True()` and `False()`
respectively.

## Raw expressions

When a needed operator is not exposed, drop to raw SQL. `Raw[T]` builds a typed
field from a SQL fragment, `RawValue[T]` wraps a literal value as a bound
placeholder, and `RawCondition` builds a condition. `Concat` joins fields with
the `||` operator.

```go
expr := gooq.Raw[float64]("ROUND(book.price, 0)")
cond := gooq.RawCondition("book.attributes ? 'featured'")
bound := gooq.RawValue[int64](42)
```

A `Raw` fragment is emitted verbatim with no placeholder translation, so it must
already be valid for the target dialect. Prefer the typed builders where they
exist, since they participate in dialect rendering and argument binding.
