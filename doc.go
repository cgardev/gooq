// Package jooq provides a type-safe, fluent SQL query builder for Go inspired
// by the Java library jOOQ. It combines parametric Field[T] columns, positional
// RecordN row types, step interfaces that make the clause order a compile-time
// concern, and runtime dialect translation from a single abstract syntax tree.
//
// The package has no external dependencies. Queries are built as a detached
// abstract syntax tree and rendered to dialect-specific SQL at execution time,
// so the same query can target PostgreSQL, MySQL, or SQLite.
package gooq

//go:generate go run ./internal/gen
