---
title: API Reference
description: Where to find the full Go API documentation and the error values exported by gooq.
---

## Full Go API documentation

The complete, always-current API reference is generated from the source and
published on pkg.go.dev:

- [pkg.go.dev/github.com/cgardev/gooq](https://pkg.go.dev/github.com/cgardev/gooq)

That page documents every exported type, function, and method, including the
full set of `Select1` through `Select22` entry points, the `Record1` through
`Record22` row types, the field types (`Field[T]`, `StringField`,
`NumericField[T]`), the `Condition` and `Dialect` types, and the INSERT,
UPDATE, and DELETE builders.

## Exported error values

gooq returns sentinel error values that you can compare with `errors.Is`. The
table below lists each one and when it occurs.

| Error | Raised when |
| --- | --- |
| `gooq.ErrTooManyRows` | `FetchSingle` matches more than one row. |
| `gooq.ErrReturningUnsupported` | `Returning` is used on a dialect that does not support `RETURNING`. |
| `gooq.ErrEmptyInsert` | An INSERT is built with no rows to insert. |
| `gooq.ErrColumnValueMismatch` | The number of values does not match the number of columns in an INSERT. |

### Handling errors

```go
row, err := gooq.
	Select1(db.Book.Title).
	From(db.Book).
	Where(db.Book.AuthorId.EQ(7)).
	FetchSingle(ctx, conn)
if errors.Is(err, gooq.ErrTooManyRows) {
	// More than one row matched; refine the predicate.
}
```

## Execution interface

The execution terminals accept any value satisfying `gooq.Querier`, which is
implemented by the standard `database/sql` types `*sql.DB` and `*sql.Tx`. gooq
imports no database driver itself; you blank-import the driver appropriate to
your database, as described in [Code Generation](/gooq/guides/code-generation/)
and [Dialects](/gooq/guides/dialects/).
