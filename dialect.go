package gooq

import (
	"strconv"
	"strings"
)

// Dialect supplies every database-specific SQL fragment that the otherwise
// dialect-agnostic abstract syntax tree requires while rendering itself. A
// single tree is rendered against a chosen Dialect at SQL time, which is how
// one query can target multiple databases.
//
// The interface is sealed: its operative methods are unexported, so callers
// obtain dialects only through Postgres and SQLite.
type Dialect interface {
	// Name returns the canonical dialect identifier.
	Name() string

	// quoteIdentifier quotes and escapes one identifier part (schema, table,
	// column, or alias). Qualification is handled by the caller, which joins
	// quoted parts with a period.
	quoteIdentifier(part string) string

	// placeholder returns the bind placeholder for the n-th argument, where n
	// is the one-based position of the argument in the final argument slice.
	placeholder(n int) string

	// renderLimit writes the dialect-specific LIMIT and OFFSET fragment. Either
	// bound may be nil; implementations handle the offset-without-limit case.
	renderLimit(b *builder, limit, offset *int64)

	// boolLiteral renders a boolean constant.
	boolLiteral(v bool) string

	// supportsReturning reports whether a RETURNING clause can be rendered.
	supportsReturning() bool

	// renderUpsert writes the conflict-resolution tail of an INSERT statement,
	// covering everything from ON CONFLICT onward.
	renderUpsert(b *builder, u *upsertClause)

	// excludedRef renders a reference to the conflicting row's column inside an
	// upsert update assignment.
	excludedRef(b *builder, column string)
}

// Postgres returns the PostgreSQL dialect: double-quoted identifiers, numbered
// placeholders ($1, $2, ...), native RETURNING, and ON CONFLICT upserts.
func Postgres() Dialect { return postgres{} }

// SQLite returns the SQLite dialect: double-quoted identifiers, anonymous
// placeholders (?), native RETURNING, and ON CONFLICT upserts.
func SQLite() Dialect { return sqlite{} }

// quoteWith quotes an identifier part using the given quote character, doubling
// any embedded occurrence of that character to escape it.
func quoteWith(part string, q byte) string {
	c := string(q)
	return c + strings.ReplaceAll(part, c, c+c) + c
}

// renderLimitOffset writes the LIMIT/OFFSET fragment shared by every dialect
// that uses the "LIMIT n OFFSET m" form. forceLimit supplies a synthetic upper
// bound for the offset-without-limit case; an empty value omits the synthetic
// LIMIT entirely.
func renderLimitOffset(b *builder, limit, offset *int64, forceLimit string) {
	switch {
	case limit != nil:
		b.writeString(" LIMIT ")
		b.writeString(strconv.FormatInt(*limit, 10))
	case offset != nil && forceLimit != "":
		b.writeString(" LIMIT ")
		b.writeString(forceLimit)
	}
	if offset != nil {
		b.writeString(" OFFSET ")
		b.writeString(strconv.FormatInt(*offset, 10))
	}
}

type postgres struct{}

func (postgres) Name() string                    { return "postgres" }
func (postgres) quoteIdentifier(p string) string { return quoteWith(p, '"') }
func (postgres) placeholder(n int) string        { return "$" + strconv.Itoa(n) }
func (postgres) supportsReturning() bool         { return true }

func (postgres) boolLiteral(v bool) string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}

func (postgres) renderLimit(b *builder, limit, offset *int64) {
	renderLimitOffset(b, limit, offset, "")
}

func (postgres) excludedRef(b *builder, column string) {
	b.writeString("EXCLUDED.")
	b.writeIdentifier(column)
}

func (d postgres) renderUpsert(b *builder, u *upsertClause) {
	renderOnConflict(b, d, u)
}

type sqlite struct{}

func (sqlite) Name() string                    { return "sqlite" }
func (sqlite) quoteIdentifier(p string) string { return quoteWith(p, '"') }
func (sqlite) placeholder(int) string          { return "?" }
func (sqlite) supportsReturning() bool         { return true }
func (sqlite) boolLiteral(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (sqlite) renderLimit(b *builder, limit, offset *int64) {
	// SQLite requires a LIMIT before OFFSET; -1 means "no limit".
	renderLimitOffset(b, limit, offset, "-1")
}

func (sqlite) excludedRef(b *builder, column string) {
	b.writeString("excluded.")
	b.writeIdentifier(column)
}

func (d sqlite) renderUpsert(b *builder, u *upsertClause) {
	renderOnConflict(b, d, u)
}

// renderOnConflict writes the PostgreSQL/SQLite "ON CONFLICT ... DO ..." tail.
func renderOnConflict(b *builder, d Dialect, u *upsertClause) {
	b.writeString(" ON CONFLICT")
	if len(u.conflictCols) > 0 {
		b.writeString(" (")
		for i, c := range u.conflictCols {
			if i > 0 {
				b.writeString(", ")
			}
			b.writeIdentifier(c)
		}
		b.writeString(")")
	}
	if u.doNothing {
		b.writeString(" DO NOTHING")
		return
	}
	b.writeString(" DO UPDATE SET ")
	for i, a := range u.assignments {
		if i > 0 {
			b.writeString(", ")
		}
		a.render(b)
	}
}
