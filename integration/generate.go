// Package integration contains the PostgreSQL integration tests for the jooq
// query builder. The typed table accessors under internal/db are produced by
// running the jooq code generator against a live PostgreSQL database; they are
// not written by hand.
//
// Regenerate the accessors after changing testdata/schema.sql by running the
// generator command, which is wired to go:generate below.
//
//go:generate go run ./internal/gendb
package integration
