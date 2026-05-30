package gooq

import "errors"

// ErrTooManyRows is returned by FetchOne and FetchSingle when a query yields
// more rows than the caller expected.
var ErrTooManyRows = errors.New("jooq: query returned more than one row")

// ErrReturningUnsupported is recorded when a RETURNING clause is requested for a
// dialect that does not support it (currently MySQL).
var ErrReturningUnsupported = errors.New("jooq: RETURNING is not supported by this dialect")

// ErrEmptyInsert is recorded when an INSERT statement has neither columns nor a
// DEFAULT VALUES marker.
var ErrEmptyInsert = errors.New("jooq: INSERT has no columns or values")

// ErrColumnValueMismatch is recorded when an inserted row has a different number
// of values than there are columns.
var ErrColumnValueMismatch = errors.New("jooq: column count does not match value count")
