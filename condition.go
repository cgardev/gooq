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

// And combines the given conditions into a single conjunction rendered as one
// flat, parenthesized "(a AND b AND c)" group rather than nested pairs. With a
// single condition it returns that condition unchanged; with no conditions it
// returns True.
func And(conds ...Condition) Condition {
	return combine("AND", True(), conds)
}

// Or combines the given conditions into a single disjunction rendered as one
// flat, parenthesized "(a OR b OR c)" group rather than nested pairs. With a
// single condition it returns that condition unchanged; with no conditions it
// returns False.
func Or(conds ...Condition) Condition {
	return combine("OR", False(), conds)
}

// combine builds a flat boolean predicate over the given conditions, returning
// the supplied empty value for an empty slice and the lone condition unchanged
// for a single-element slice.
func combine(op string, empty Condition, conds []Condition) Condition {
	switch len(conds) {
	case 0:
		return empty
	case 1:
		return conds[0]
	}
	parts := make([]node, len(conds))
	for i, c := range conds {
		parts[i] = c
	}
	return newCondition(&boolPredicate{op: op, parts: parts})
}

// Not negates the given condition, rendering "NOT (condition)".
func Not(c Condition) Condition {
	return c.Not()
}

// True returns a condition that always holds, rendered as the constant
// predicate "1 = 1".
func True() Condition {
	return newCondition(&literalNode{sql: "1 = 1"})
}

// False returns a condition that never holds, rendered as the constant
// predicate "1 = 0".
func False() Condition {
	return newCondition(&literalNode{sql: "1 = 0"})
}
