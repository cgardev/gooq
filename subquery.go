package gooq

// subquery.go models SELECT statements used as sub-expressions: EXISTS
// predicates, IN (SELECT ...) membership tests, and scalar subqueries usable as
// a Field. Every sub-statement renders into the same builder as the outer
// statement so that bind arguments interleave in render order and positional
// placeholders are numbered correctly under PostgreSQL.

// Subquery is a SELECT statement usable as a sub-expression. It is sealed
// through the embedded node interface, so only this package can provide
// implementations; the concrete *selectBuilder produced by the SelectN
// constructors satisfies it. The interface keeps the subquery-taking API typed
// while preserving the seal on the SELECT step chain.
type Subquery interface {
	node
	// renderSubquery writes the parenthesized "(SELECT ...)" form of this
	// statement into the given builder.
	renderSubquery(b *builder)
}

// subqueryRenderer is the unexported capability implemented by the SELECT
// builder to render itself parenthesized into an outer builder. It is the
// mechanism that lets a SELECT participate as a sub-expression without exposing
// any new public surface beyond the Subquery marker.
type subqueryRenderer interface {
	renderSubquery(b *builder)
}

// renderSubquery writes the parenthesized statement into the builder. Rendering
// into the supplied builder, rather than a fresh one, keeps the bind arguments
// of the inner statement interleaved with the outer statement in render order.
func (s *selectBuilder[R]) renderSubquery(b *builder) {
	b.writeString("(")
	s.queryNode().render(b)
	b.writeString(")")
}

// existsPredicate renders "EXISTS (SELECT ...)" or "NOT EXISTS (SELECT ...)".
type existsPredicate struct {
	sub     subqueryRenderer
	negated bool
}

func (p *existsPredicate) render(b *builder) {
	if p.negated {
		b.writeString("NOT EXISTS ")
	} else {
		b.writeString("EXISTS ")
	}
	p.sub.renderSubquery(b)
}

// Exists returns a Condition rendering "EXISTS (SELECT ...)" over the given
// subquery. The subquery renders into the same builder as the surrounding
// statement, so its bind arguments are numbered in render order.
func Exists(sub Subquery) Condition {
	return newCondition(&existsPredicate{sub: sub})
}

// NotExists returns a Condition rendering "NOT EXISTS (SELECT ...)" over the
// given subquery.
func NotExists(sub Subquery) Condition {
	return newCondition(&existsPredicate{sub: sub, negated: true})
}

// inSubqueryPredicate renders "operand [NOT] IN (SELECT ...)". It is kept
// distinct from the value-list inPredicate so the two membership forms never
// share rendering logic.
type inSubqueryPredicate struct {
	operand node
	sub     subqueryRenderer
	negated bool
}

func (p *inSubqueryPredicate) render(b *builder) {
	p.operand.render(b)
	if p.negated {
		b.writeString(" NOT IN ")
	} else {
		b.writeString(" IN ")
	}
	p.sub.renderSubquery(b)
}

// scalarSubqueryNode renders a parenthesized scalar subquery usable wherever a
// single-valued expression is expected.
type scalarSubqueryNode struct {
	sub subqueryRenderer
}

func (n *scalarSubqueryNode) render(b *builder) {
	n.sub.renderSubquery(b)
}

// ScalarSubquery adapts a single-column, single-row subquery to a typed
// Field[T], so it can be projected, compared, or otherwise used where a scalar
// expression is expected. The element type T is supplied by the caller and is
// not verified against the subquery's projection.
func ScalarSubquery[T any](sub Subquery) Field[T] {
	return field[T]{expr: &scalarSubqueryNode{sub: sub}, name: "subquery"}
}
