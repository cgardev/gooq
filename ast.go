package gooq

// This file holds the small, unexported nodes of the expression abstract
// syntax tree. Each node renders itself, delegating only the genuinely
// dialect-specific fragments to the active Dialect through the builder.

// columnNode is a (optionally qualified) reference to a table column.
type columnNode struct {
	qualifier string // table name or table alias; empty for an unqualified column
	name      string
}

func (c *columnNode) render(b *builder) {
	if c.qualifier != "" {
		b.writeIdentifier(c.qualifier, c.name)
		return
	}
	b.writeIdentifier(c.name)
}

// aliasNode wraps an expression with an alias. The alias is declared
// ("inner AS alias") only inside a SELECT projection; everywhere else only the
// underlying expression is rendered, which keeps the SQL valid when an aliased
// field is reused in a predicate.
type aliasNode struct {
	inner node
	alias string
}

func (a *aliasNode) render(b *builder) {
	a.inner.render(b)
	if b.declareAlias {
		b.writeString(" AS ")
		b.writeIdentifier(a.alias)
	}
}

// bindParam is a value bound as a placeholder argument.
type bindParam struct {
	val any
}

func (p *bindParam) render(b *builder) {
	b.bind(p.val)
}

// bindOf wraps a value as a bind parameter node.
func bindOf(v any) node { return &bindParam{val: v} }

// exprOf resolves an operand into an expression node. When the value is one of
// gooq's own field or condition types, its underlying expression node is reused
// directly, so it renders as an identifier or expression rather than a bound
// argument. Any other value is wrapped as a bind parameter. This backs the
// expression-taking operator variants (the *Field methods and Concat) while the
// value-typed methods keep binding their argument for compile-time type safety.
func exprOf(v any) node {
	switch e := v.(type) {
	case node:
		// All field and condition types implement node through their embedded
		// field[T]; reuse the operand's own rendering directly.
		return e
	default:
		return bindOf(v)
	}
}

// bindsOf wraps a slice of values as bind parameter nodes.
func bindsOf[T any](vs []T) []node {
	out := make([]node, len(vs))
	for i, v := range vs {
		out[i] = &bindParam{val: v}
	}
	return out
}

// literalNode is raw SQL spliced verbatim into the output with no arguments.
type literalNode struct {
	sql string
}

func (l *literalNode) render(b *builder) { b.writeString(l.sql) }

// binaryPredicate renders "left op right" for comparison operators.
type binaryPredicate struct {
	left  node
	op    string
	right node
}

func (p *binaryPredicate) render(b *builder) {
	p.left.render(b)
	b.writeString(" ")
	b.writeString(p.op)
	b.writeString(" ")
	p.right.render(b)
}

// boolPredicate renders a parenthesized AND/OR chain of sub-predicates.
type boolPredicate struct {
	op    string // "AND" or "OR"
	parts []node
}

func (p *boolPredicate) render(b *builder) {
	b.writeString("(")
	for i, part := range p.parts {
		if i > 0 {
			b.writeString(" ")
			b.writeString(p.op)
			b.writeString(" ")
		}
		part.render(b)
	}
	b.writeString(")")
}

// notPredicate renders "NOT (inner)".
type notPredicate struct {
	inner node
}

func (p *notPredicate) render(b *builder) {
	b.writeString("NOT (")
	p.inner.render(b)
	b.writeString(")")
}

// nullPredicate renders "operand IS [NOT] NULL".
type nullPredicate struct {
	operand node
	negated bool
}

func (p *nullPredicate) render(b *builder) {
	p.operand.render(b)
	if p.negated {
		b.writeString(" IS NOT NULL")
		return
	}
	b.writeString(" IS NULL")
}

// inPredicate renders "operand [NOT] IN (v1, v2, ...)". An empty value list
// renders a constant predicate so the resulting SQL stays valid.
type inPredicate struct {
	operand node
	vals    []node
	negated bool
}

func (p *inPredicate) render(b *builder) {
	if len(p.vals) == 0 {
		if p.negated {
			b.writeString("1 = 1")
		} else {
			b.writeString("1 = 0")
		}
		return
	}
	p.operand.render(b)
	if p.negated {
		b.writeString(" NOT IN (")
	} else {
		b.writeString(" IN (")
	}
	renderList(b, p.vals)
	b.writeString(")")
}

// likePredicate renders "operand LIKE/NOT LIKE/ILIKE pattern".
type likePredicate struct {
	operand node
	op      string
	pattern node
}

func (p *likePredicate) render(b *builder) {
	p.operand.render(b)
	b.writeString(" ")
	b.writeString(p.op)
	b.writeString(" ")
	p.pattern.render(b)
}

// betweenPredicate renders "operand BETWEEN lo AND hi", or "operand NOT BETWEEN
// lo AND hi" when negated.
type betweenPredicate struct {
	operand node
	lo      node
	hi      node
	negated bool
}

func (p *betweenPredicate) render(b *builder) {
	p.operand.render(b)
	if p.negated {
		b.writeString(" NOT BETWEEN ")
	} else {
		b.writeString(" BETWEEN ")
	}
	p.lo.render(b)
	b.writeString(" AND ")
	p.hi.render(b)
}

// arithExpr renders a parenthesized arithmetic expression "(left op right)".
type arithExpr struct {
	left  node
	op    string
	right node
}

func (e *arithExpr) render(b *builder) {
	b.writeString("(")
	e.left.render(b)
	b.writeString(" ")
	b.writeString(e.op)
	b.writeString(" ")
	e.right.render(b)
	b.writeString(")")
}

// negExpr renders a unary arithmetic negation "-(operand)". The operand is
// parenthesized so that negating a compound expression keeps the intended
// precedence.
type negExpr struct {
	operand node
}

func (e *negExpr) render(b *builder) {
	b.writeString("-(")
	e.operand.render(b)
	b.writeString(")")
}

// concatExpr renders a parenthesized chain of operands joined by the SQL string
// concatenation operator "||", which both PostgreSQL and SQLite support.
type concatExpr struct {
	parts []node
}

func (e *concatExpr) render(b *builder) {
	b.writeString("(")
	for i, part := range e.parts {
		if i > 0 {
			b.writeString(" || ")
		}
		part.render(b)
	}
	b.writeString(")")
}

// excludedNode renders a reference to the conflicting row's column within an
// upsert update assignment, delegating the exact spelling to the dialect.
type excludedNode struct {
	column string
}

func (e *excludedNode) render(b *builder) {
	b.dialect.excludedRef(b, e.column)
}
