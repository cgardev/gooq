---
title: Cookbook Overview
description: Task-oriented gooq recipes for common querying, writing, mapping, and setup problems.
---

The cookbook collects short, copy-ready recipes. Each one states the task,
provides a single complete Go snippet, shows the rendered SQL where it helps, and
ends with a one-line gotcha. For the systematic API, see the
[Reference](/gooq/reference/).

Every recipe uses the generated `db` package, a `context.Context` named `ctx`,
and a connection (`*sql.DB` or `*sql.Tx`) named `conn`. Rendered SQL is shown for
PostgreSQL.

## Recipes

### [Querying data](/gooq/cookbook/queries/)

- Paginate a result set
- Build a filter dynamically with And/Or
- Case-insensitive search with ILike
- Top-N rows per group with a window function
- A grouped report with GROUP BY and HAVING
- Bucket values with Coalesce and CASE
- Join three tables
- Left join that keeps unmatched rows
- Filter with EXISTS
- Filter with an IN subquery
- Combine results with UNION

### [Writing data](/gooq/cookbook/writing-data/)

- Insert and read the new row back
- Bulk insert many rows
- Upsert with DoUpdateSet and SetToExcluded
- Insert, ignoring conflicts
- Copy rows with INSERT ... SELECT
- Update and read the changed rows back
- Apply a conditional update
- Delete and return the removed rows
- Run several statements in a transaction
- Batch several statements

### [Mapping results](/gooq/cookbook/mapping-results/)

- Map rows into a struct with db tags
- Handle nullable columns
- Index rows by key with FetchMap
- Group rows by key with FetchGroups
- Require exactly one row, or allow none
- Reuse a stored Condition across queries

### [Code generation and setup](/gooq/cookbook/codegen-and-setup/)

- Generate accessors with a blank-imported driver
- Override a SQL type (uuid to a custom type)
- Use a generated enum
- Query a generated view
- Inspect foreign keys through metadata
- Switch dialects with SQLFor and Using
