package gooq

import "strings"

// This file holds the SQL function and expression catalog: a generic function
// node and node-local constructors for aggregates, conditional and null-handling
// expressions, a CASE builder, a CAST, and a small string and math catalog. Each
// construct is a self-rendering node that delegates only genuinely
// dialect-specific fragments to the active Dialect, and every public constructor
// returns a typed Field so the results compose with the rest of the builder.

// funcNode renders a generic SQL function call: "NAME(" followed by the
// comma-separated arguments and a closing parenthesis. When distinct is set, the
// argument list is prefixed with "DISTINCT ", which backs aggregates such as
// COUNT(DISTINCT column).
type funcNode struct {
	name     string
	args     []node
	distinct bool
}

func (f *funcNode) render(b *builder) {
	b.writeString(f.name)
	b.writeString("(")
	if f.distinct {
		b.writeString("DISTINCT ")
	}
	renderList(b, f.args)
	b.writeString(")")
}

// starNode renders an unqualified "*", used as the argument to COUNT(*) and
// anywhere a wildcard projection is required.
type starNode struct{}

func (starNode) render(b *builder) { b.writeString("*") }

// Star returns a field rendering an unqualified "*". It is type-erased because a
// wildcard has no single column type; use it as the argument to Function or in a
// projection where the element type does not matter.
func Star() AnyField {
	return field[any]{expr: starNode{}, name: "*"}
}

// Function builds a typed call to an arbitrary SQL function whose arguments are
// fields or expressions. The result type T is chosen by the caller, mirroring
// jOOQ's DSL.function. Each argument renders as its own column or expression
// rather than binding a value.
func Function[T any](name string, args ...AnyField) Field[T] {
	nodes := make([]node, len(args))
	for i, a := range args {
		nodes[i] = a
	}
	return field[T]{expr: &funcNode{name: name, args: nodes}, name: name}
}

// Count returns the COUNT of the given field's non-null values as a Field[int64].
func Count(f AnyField) Field[int64] {
	return field[int64]{expr: &funcNode{name: "COUNT", args: []node{f}}, name: "count"}
}

// CountStar returns COUNT(*), counting every row regardless of nulls, as a
// Field[int64].
func CountStar() Field[int64] {
	return field[int64]{expr: &funcNode{name: "COUNT", args: []node{starNode{}}}, name: "count"}
}

// CountDistinct returns COUNT(DISTINCT field), counting the distinct non-null
// values of the field, as a Field[int64].
func CountDistinct(f AnyField) Field[int64] {
	return field[int64]{expr: &funcNode{name: "COUNT", args: []node{f}, distinct: true}, name: "count"}
}

// Sum returns the SUM aggregate of the given field, preserving the field's type.
func Sum[T any](f Field[T]) Field[T] {
	return field[T]{expr: &funcNode{name: "SUM", args: []node{f}}, name: "sum"}
}

// Avg returns the AVG aggregate of the given field as a Field[float64], since an
// average is computed in floating point regardless of the operand type.
func Avg[T any](f Field[T]) Field[float64] {
	return field[float64]{expr: &funcNode{name: "AVG", args: []node{f}}, name: "avg"}
}

// Min returns the MIN aggregate of the given field, preserving the field's type.
func Min[T any](f Field[T]) Field[T] {
	return field[T]{expr: &funcNode{name: "MIN", args: []node{f}}, name: "min"}
}

// Max returns the MAX aggregate of the given field, preserving the field's type.
func Max[T any](f Field[T]) Field[T] {
	return field[T]{expr: &funcNode{name: "MAX", args: []node{f}}, name: "max"}
}

// Coalesce returns the first non-null operand, rendered as COALESCE(a, b, ...).
// The first operand is a typed field; the remaining operands may be values
// (which bind) or fields and expressions (which render as identifiers), exactly
// like the field-operand operator variants elsewhere in the builder.
func Coalesce[T any](first Field[T], rest ...any) Field[T] {
	args := make([]node, 0, len(rest)+1)
	args = append(args, first)
	for _, r := range rest {
		args = append(args, exprOf(r))
	}
	return field[T]{expr: &funcNode{name: "COALESCE", args: args}, name: "coalesce"}
}

// NullIf returns NULLIF(a, b): the SQL NULL when the two operands are equal and
// the first operand otherwise. The comparison value binds as an argument.
func NullIf[T any](a Field[T], b T) Field[T] {
	return field[T]{expr: &funcNode{name: "NULLIF", args: []node{a, bindOf(b)}}, name: "nullif"}
}

// Greatest returns the greatest of its operands, rendered as GREATEST(a, b, ...).
// The first operand is a typed field; the remaining operands may be values
// (which bind) or fields and expressions (which render as identifiers).
func Greatest[T any](first Field[T], rest ...any) Field[T] {
	args := make([]node, 0, len(rest)+1)
	args = append(args, first)
	for _, r := range rest {
		args = append(args, exprOf(r))
	}
	return field[T]{expr: &funcNode{name: "GREATEST", args: args}, name: "greatest"}
}

// Least returns the least of its operands, rendered as LEAST(a, b, ...). The
// first operand is a typed field; the remaining operands may be values (which
// bind) or fields and expressions (which render as identifiers).
func Least[T any](first Field[T], rest ...any) Field[T] {
	args := make([]node, 0, len(rest)+1)
	args = append(args, first)
	for _, r := range rest {
		args = append(args, exprOf(r))
	}
	return field[T]{expr: &funcNode{name: "LEAST", args: args}, name: "least"}
}

// caseWhen is one WHEN/THEN branch of a CASE expression.
type caseWhen struct {
	cond node
	then node
}

// caseNode renders a searched CASE expression: a sequence of WHEN/THEN branches
// followed by an optional ELSE and a terminating END.
type caseNode struct {
	whens   []caseWhen
	hasElse bool
	elseVal node
}

func (c *caseNode) render(b *builder) {
	b.writeString("CASE")
	for _, w := range c.whens {
		b.writeString(" WHEN ")
		w.cond.render(b)
		b.writeString(" THEN ")
		w.then.render(b)
	}
	if c.hasElse {
		b.writeString(" ELSE ")
		c.elseVal.render(b)
	}
	b.writeString(" END")
}

// CaseBuilder accumulates the branches of a searched CASE expression. It is
// returned by Case and finalised with End, which yields a typed Field[T].
type CaseBuilder[T any] struct {
	node *caseNode
}

// Case begins a searched CASE expression producing values of type T.
func Case[T any]() CaseBuilder[T] {
	return CaseBuilder[T]{node: &caseNode{}}
}

// When adds a "WHEN condition THEN value" branch whose result value binds as an
// argument.
func (c CaseBuilder[T]) When(cond Condition, then T) CaseBuilder[T] {
	c.node.whens = append(c.node.whens, caseWhen{cond: cond, then: bindOf(then)})
	return c
}

// WhenField adds a "WHEN condition THEN field" branch whose result renders as the
// given field's column or expression rather than binding a value.
func (c CaseBuilder[T]) WhenField(cond Condition, then Field[T]) CaseBuilder[T] {
	c.node.whens = append(c.node.whens, caseWhen{cond: cond, then: then})
	return c
}

// Else sets the ELSE result of the CASE expression, binding its value as an
// argument. Calling Else more than once replaces the previous ELSE value.
func (c CaseBuilder[T]) Else(v T) CaseBuilder[T] {
	c.node.hasElse = true
	c.node.elseVal = bindOf(v)
	return c
}

// End finalises the CASE expression and returns it as a typed Field[T].
func (c CaseBuilder[T]) End() Field[T] {
	return field[T]{expr: c.node, name: "case"}
}

// castNode renders "CAST(expr AS sqltype)". The target type is verbatim SQL, so
// the caller chooses a spelling valid for the active dialect.
type castNode struct {
	inner   node
	sqlType string
}

func (c *castNode) render(b *builder) {
	b.writeString("CAST(")
	c.inner.render(b)
	b.writeString(" AS ")
	b.writeString(c.sqlType)
	b.writeString(")")
}

// Cast converts the given field or expression to the named SQL type, rendered as
// CAST(expr AS sqltype), and returns it as a Field[T]. The SQL type is spliced
// verbatim, so the caller supplies a spelling valid for the target dialect.
func Cast[T any](f AnyField, sqlType string) Field[T] {
	return field[T]{expr: &castNode{inner: f, sqlType: strings.TrimSpace(sqlType)}, name: "cast"}
}

// Upper returns UPPER(field), the uppercase form of the text field.
func Upper(f Field[string]) Field[string] {
	return field[string]{expr: &funcNode{name: "UPPER", args: []node{f}}, name: "upper"}
}

// Lower returns LOWER(field), the lowercase form of the text field.
func Lower(f Field[string]) Field[string] {
	return field[string]{expr: &funcNode{name: "LOWER", args: []node{f}}, name: "lower"}
}

// Length returns LENGTH(field), the character length of the text field, as a
// Field[int64].
func Length(f Field[string]) Field[int64] {
	return field[int64]{expr: &funcNode{name: "LENGTH", args: []node{f}}, name: "length"}
}

// Trim returns TRIM(field), the text field with leading and trailing whitespace
// removed.
func Trim(f Field[string]) Field[string] {
	return field[string]{expr: &funcNode{name: "TRIM", args: []node{f}}, name: "trim"}
}

// Abs returns ABS(field), the absolute value of the numeric field, preserving the
// field's type.
func Abs[T any](f Field[T]) Field[T] {
	return field[T]{expr: &funcNode{name: "ABS", args: []node{f}}, name: "abs"}
}

// Round returns ROUND(field), the numeric field rounded to the nearest integer,
// preserving the field's type.
func Round[T any](f Field[T]) Field[T] {
	return field[T]{expr: &funcNode{name: "ROUND", args: []node{f}}, name: "round"}
}
