package gooq

// Condition is a boolean predicate. Mirroring jOOQ, a Condition is itself a
// Field[bool], so it can be projected, compared, and stored in a variable, in
// addition to being combined with And, Or, and Not.
type Condition interface {
	Field[bool]
	And(other Condition) Condition
	Or(other Condition) Condition
	Not() Condition
}

// condition is the concrete Condition. It embeds field[bool] to inherit the
// full Field[bool] method set (so a Condition really is a Field[bool]) and adds
// the boolean combinators on top.
type condition struct {
	field[bool]
}

// newCondition wraps a predicate node as a Condition.
func newCondition(n node) Condition {
	return &condition{field: field[bool]{expr: n}}
}

func (c *condition) And(other Condition) Condition {
	return newCondition(&boolPredicate{op: "AND", parts: []node{c.expr, other}})
}

func (c *condition) Or(other Condition) Condition {
	return newCondition(&boolPredicate{op: "OR", parts: []node{c.expr, other}})
}

func (c *condition) Not() Condition {
	return newCondition(&notPredicate{inner: c.expr})
}
