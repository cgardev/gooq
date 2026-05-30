package gooq

import "strings"

// builder accumulates the rendered SQL text together with its ordered bind
// arguments while a statement tree renders itself node by node. A single
// builder owns both the SQL buffer and the argument slice so that placeholder
// numbering always matches argument order, regardless of dialect.
type builder struct {
	dialect Dialect
	sql     strings.Builder
	args    []any
	err     error

	// declareAlias is true while the projection list of a SELECT is rendered,
	// signalling aliased expressions to emit their "AS alias" declaration.
	// Everywhere else an aliased expression renders only its underlying value.
	declareAlias bool
}

// newBuilder creates a builder bound to the given dialect.
func newBuilder(d Dialect) *builder {
	return &builder{dialect: d}
}

// writeString appends raw SQL text to the buffer.
func (b *builder) writeString(s string) {
	b.sql.WriteString(s)
}

// writeIdentifier appends one or more identifier parts, each quoted and escaped
// by the active dialect and joined with a period.
func (b *builder) writeIdentifier(parts ...string) {
	for i, p := range parts {
		if i > 0 {
			b.sql.WriteByte('.')
		}
		b.sql.WriteString(b.dialect.quoteIdentifier(p))
	}
}

// bind appends a bind argument and writes its dialect-specific placeholder.
func (b *builder) bind(v any) {
	b.args = append(b.args, v)
	b.sql.WriteString(b.dialect.placeholder(len(b.args)))
}

// setError records the first deferred error encountered while rendering. A
// rendering error (for example, an unsupported clause for the active dialect)
// is surfaced by SQL, SQLFor, Fetch, and Execute rather than panicking.
func (b *builder) setError(err error) {
	if b.err == nil {
		b.err = err
	}
}

// result returns the rendered SQL, the ordered argument slice, and any deferred
// rendering error.
func (b *builder) result() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	return b.sql.String(), b.args, nil
}
