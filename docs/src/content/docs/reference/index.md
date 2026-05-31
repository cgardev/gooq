---
title: Reference Overview
description: A systematic reference for the gooq query builder, organized by feature area.
---

This reference documents the full feature set of gooq, the type-safe SQL builder
for Go. Each page covers one area of the API in detail, with real Go examples and
the SQL each statement renders. For task-oriented snippets, see the
[Cookbook](/gooq/cookbook/).

## How the reference is organized

- [SELECT statements](/gooq/reference/select/) — projections, joins,
  `DISTINCT`, `GROUP BY`/`HAVING`, ordering with `NULLS FIRST`/`LAST`,
  `LIMIT`/`OFFSET`, and the row-locking clauses.
- [Expressions and operators](/gooq/reference/expressions/) — field
  comparisons, arithmetic, string predicates, `IN`/`BETWEEN`, `IS NULL`, and the
  boolean combinators.
- [Functions, aggregates, CASE and CAST](/gooq/reference/functions/) — scalar
  functions, aggregate functions, conditional expressions, and type casts.
- [Window functions](/gooq/reference/window-functions/) — ranking and analytic
  functions with the `Over`, `PartitionBy`, and `OrderBy` builder.
- [Subqueries and set operations](/gooq/reference/subqueries-and-set-operations/)
  — `EXISTS`, `IN` subqueries, scalar subqueries, and
  `UNION`/`INTERSECT`/`EXCEPT`.
- [Data modification](/gooq/reference/dml/) — `INSERT`, `UPDATE`, `DELETE`,
  upserts, `RETURNING`, `INSERT ... SELECT`, `UPDATE ... FROM`,
  `DELETE ... USING`, and batch execution.
- [Fetching and mapping](/gooq/reference/fetching-and-mapping/) — the terminal
  methods, the struct-mapping helpers, and transactions.
- [Dialects](/gooq/reference/dialects/) — how SQL is rendered for PostgreSQL and
  SQLite, and how to choose a dialect.
- [Code generation](/gooq/reference/code-generation/) — generating typed table
  accessors with `gooq-gen`.

## The example schema

Every example uses the generated `db` package produced from the integration test
schema. The relevant accessors and their field types are:

```go
// db.Author: author table.
//   Id        StringField              (uuid)
//   Name      StringField
//   Email     StringField
//   Metadata  Field[[]byte]            (nullable jsonb)
//   CreatedAt Field[time.Time]

// db.Book: book table.
//   Id          StringField             (uuid)
//   AuthorId    StringField             (uuid)
//   EditorId    Field[sql.Null[string]] (nullable uuid)
//   Title       StringField
//   Subtitle    Field[sql.Null[string]] (nullable)
//   Price       NumericField[float64]   (numeric)
//   PageCount   NumericField[int64]     (integer)
//   InPrint     Field[bool]
//   Status      Field[BookStatus]       (enum)
//   Attributes  Field[json.RawMessage]  (jsonb)
//   PublishedAt Field[time.Time]
//   CreatedAt   Field[time.Time]

// db.Review: review table.
//   Id       StringField               (uuid)
//   BookId   StringField               (uuid)
//   Reviewer StringField
//   Rating   NumericField[int64]       (integer)
//   Body     Field[sql.Null[string]]   (nullable)
//   PostedAt Field[time.Time]

// db.BookOverview: a view joining book and author.
//   BookId, Title StringField; Status Field[BookStatus]; AuthorName StringField
```

Throughout the reference, `db` is the generated package, `ctx` is a
`context.Context`, and `conn` is a `*sql.DB` (or `*sql.Tx`) that satisfies the
`gooq.Querier` interface. Rendered SQL is shown for PostgreSQL, which quotes
identifiers and uses numbered placeholders (`$1`, `$2`, ...).
