---
title: Predicates & Conditions
description: Build WHERE, ON, and HAVING predicates from typed field operators and combine them with And, Or, and Not.
---

Predicates are produced by calling comparison methods on fields. Every method
returns a `Condition`, which can be passed to `.Where`, `.On`, `.And`, `.Or`,
and `.Having`. Because the operators are defined on `Field[T]`, the value you
compare against must match the column's type.

## Comparison operators

All fields expose the standard comparison operators. Each takes a value of the
field's type and returns a `Condition`:

```go
db.Book.Price.EQ(9.99) // price =  $1
db.Book.Price.NE(9.99) // price <> $1
db.Book.Price.GT(10)   // price >  $1
db.Book.Price.LT(10)   // price <  $1
db.Book.Price.GE(10)   // price >= $1
db.Book.Price.LE(10)   // price <= $1
```

## Comparing two columns

`EQField` compares a field to another field of the same type, which is the usual
shape of a join predicate:

```go
db.Book.AuthorId.EQField(db.Author.Id)
```

## Sets and ranges

```go
db.Book.Id.In(1, 2, 3)             // id IN ($1, $2, $3)
db.Book.Id.NotIn(4, 5)             // id NOT IN ($1, $2)
db.Book.Price.Between(10.0, 20.0)  // price BETWEEN $1 AND $2
```

## Null checks

```go
db.Book.PublishedAt.IsNull()
db.Book.PublishedAt.IsNotNull()
```

## String matching

`StringField` adds pattern-matching operators. `ILike` is the case-insensitive
variant:

```go
db.Book.Title.Like("The %")      // title LIKE $1
db.Book.Title.NotLike("% Draft") // title NOT LIKE $1
db.Book.Title.ILike("the %")     // case-insensitive match
```

## Arithmetic on numeric fields

`NumericField[T]` supports arithmetic that yields a new `Field[T]`, which can be
projected or compared:

```go
discounted := db.Book.Price.Mul(0.9) // price * $1, still a Field[float64]

sql, args, err := gooq.
	Select1(discounted.As("sale_price")).
	From(db.Book).
	Where(db.Book.Price.Sub(2).GT(5)). // (price - $1) > $2
	SQL()
```

The full set is `Add`, `Sub`, `Mul`, and `Div`.

## Combining conditions

A `Condition` is itself a `Field[bool]`, so conditions compose with `And`,
`Or`, and `Not`:

```go
expensive := db.Book.Price.GT(50)
recent := db.Book.PublishedAt.IsNotNull()

predicate := expensive.And(recent).Or(db.Book.Title.ILike("classic%"))
negated := expensive.Not()
```

You can either combine conditions explicitly with these methods or chain
`.And` / `.Or` directly on the query after `.Where`:

```go
rows, err := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Price.GT(10)).
	And(db.Book.Price.LT(100)).
	Or(db.Book.Title.ILike("free%")).
	Fetch(ctx, conn)
```

## A worked predicate

```go
rows, err := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(
		db.Book.Price.Between(10.0, 80.0).
			And(db.Book.Title.NotLike("%(draft)")).
			And(db.Book.PublishedAt.IsNotNull()),
	).
	OrderBy(db.Book.Price.Asc()).
	Fetch(ctx, conn)
```

Continue to [Inserts, Updates & Deletes](/gooq/guides/inserts-updates-deletes/)
for write statements, including `RETURNING` and upserts.
