---
title: Building Queries
description: Construct SELECT statements with the fluent step interfaces and typed RecordN result rows.
---

A query in gooq is built by chaining methods, where each method returns the
next valid step. The chain reads in the same order as the SQL it produces, and
the type system prevents clauses from appearing out of order.

## Entry points

A SELECT begins with one of the numbered `Select` functions. The number matches
the count of projected columns, and the column types flow through to the result
rows:

```go
// Select a single column: result rows are Record1[string].
q1 := gooq.Select1(db.Book.Title)

// Select two columns: result rows are Record2[string, float64].
q2 := gooq.Select2(db.Book.Title, db.Book.Price)
```

`Select1` through `Select22` are available, returning a
`SelectFromStep[RecordN[...]]`. The `RecordN` type parameter records the exact
shape of each result row.

## The fluent chain

From the entry point you proceed through the clauses. Optional clauses may be
skipped, but their relative order is fixed:

```text
.From(table)
  → .Join(t) / .InnerJoin(t) / .LeftJoin(t) / .RightJoin(t)  then  .On(cond)
  → .Where(cond)
  → .And(cond) / .Or(cond)
  → .GroupBy(fields...)
  → .Having(cond)
  → .OrderBy(orders...)
  → .Limit(n int64)
  → .Offset(n int64)
```

Because each step exposes only the methods that may legally follow it, you
cannot, for example, call `.Where` before `.From` or `.Having` before
`.GroupBy`. Such mistakes do not compile.

## A complete SELECT

```go
rows, err := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Price.GT(10)).
	And(db.Book.Title.Like("The %")).
	OrderBy(db.Book.Price.Desc(), db.Book.Title.Asc()).
	Limit(20).
	Offset(40).
	Fetch(ctx, conn)
```

## Joins

Start a join with one of the join methods, then complete it with `.On`:

```go
rows, err := gooq.
	Select2(db.Book.Title, db.Author.Name).
	From(db.Book).
	InnerJoin(db.Author).On(db.Book.AuthorId.EQField(db.Author.Id)).
	Where(db.Author.Name.ILike("a%")).
	Fetch(ctx, conn)
```

`EQField` compares two columns to each other rather than a column to a literal,
which is the common case for join predicates.

## Grouping and aggregation

```go
sql, args, err := gooq.
	Select1(db.Book.AuthorId).
	From(db.Book).
	GroupBy(db.Book.AuthorId).
	Having(db.Book.AuthorId.GT(0)).
	SQL()
```

## Aliasing

Both fields and tables support aliasing. A field alias uses `As`, and a table
alias is produced by the generated `As(alias)` method:

```go
b := db.Book.As("b")

sql, args, err := gooq.
	Select1(b.Title.As("book_title")).
	From(b).
	SQL()
```

## Result rows: the RecordN types

Every row returned by `Fetch` is a positional record whose fields are typed:

```go
rows, err := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Fetch(ctx, conn)

for _, row := range rows {
	// row is gooq.Record2[string, float64].
	title := row.V1 // string
	price := row.V2 // float64
	_ = title
	_ = price
}
```

`Record1[T1]` through `Record22[...]` expose their values as `.V1`, `.V2`, and
so on. Two helper methods are also available when positional access is more
convenient:

```go
values := row.Values() // []any{title, price}
first := row.Get(0)    // any, the first value
```

## Fetching one row

When a query is expected to return at most one row, use one of the single-row
terminals instead of `Fetch`:

```go
// FetchOne returns the first row, or a zero record if there are none.
row, err := gooq.
	Select2(db.Book.Title, db.Book.Price).
	From(db.Book).
	Where(db.Book.Id.EQ(42)).
	FetchOne(ctx, conn)

// FetchSingle requires exactly one row and returns gooq.ErrTooManyRows
// if more than one row matches.
only, err := gooq.
	Select1(db.Book.Title).
	From(db.Book).
	Where(db.Book.Id.EQ(42)).
	FetchSingle(ctx, conn)
```

## Terminals

A built query can be finished in several ways:

| Terminal | Result |
| --- | --- |
| `.SQL()` | `(string, []any, error)` using the default rendering |
| `.SQLFor(d gooq.Dialect)` | `(string, []any, error)` rendered for a specific dialect |
| `.Using(d gooq.Dialect)` | binds a dialect, then `Fetch`/`FetchOne`/`FetchSingle` |
| `.Fetch(ctx, db)` | `([]RecordN, error)` |
| `.FetchOne(ctx, db)` | `(RecordN, error)` |
| `.FetchSingle(ctx, db)` | `(RecordN, error)`, errors if more than one row |

See [Dialects](/gooq/guides/dialects/) for how `SQLFor` and `Using` adapt the
rendered SQL to each database.
