package gooq

// StringField is a Field[string] augmented with pattern-matching predicates that
// only make sense for text columns.
type StringField interface {
	Field[string]
	Like(pattern string) Condition
	NotLike(pattern string) Condition
	// ILike performs a case-insensitive match. It maps to ILIKE on PostgreSQL
	// and to a plain LIKE on SQLite, whose LIKE is already case-insensitive for
	// ASCII text.
	ILike(pattern string) Condition
	// Concat returns a field that concatenates this field with the given parts
	// using the SQL "||" operator. String literals are bound as arguments,
	// while StringField and Field[string] operands render as their column or
	// expression.
	Concat(others ...any) Field[string]
}

// stringField is the concrete StringField.
type stringField struct {
	field[string]
}

func (s stringField) Like(pattern string) Condition {
	return newCondition(&likePredicate{operand: s.expr, op: "LIKE", pattern: bindOf(pattern)})
}

func (s stringField) NotLike(pattern string) Condition {
	return newCondition(&likePredicate{operand: s.expr, op: "NOT LIKE", pattern: bindOf(pattern)})
}

func (s stringField) ILike(pattern string) Condition {
	return newCondition(&ilikePredicate{operand: s.expr, pattern: bindOf(pattern)})
}

func (s stringField) Concat(others ...any) Field[string] {
	parts := make([]node, 0, len(others)+1)
	parts = append(parts, s.expr)
	for _, o := range others {
		parts = append(parts, exprOf(o))
	}
	return field[string]{expr: &concatExpr{parts: parts}, name: s.name}
}

// Concat returns a field that concatenates the given parts using the SQL "||"
// operator. String literals are bound as arguments, while StringField and
// Field[string] operands render as their column or expression. Both PostgreSQL
// and SQLite support the "||" operator.
func Concat(parts ...any) Field[string] {
	nodes := make([]node, len(parts))
	for i, p := range parts {
		nodes[i] = exprOf(p)
	}
	return field[string]{expr: &concatExpr{parts: nodes}}
}

// NewStringField builds a qualified text column field.
func NewStringField(base TableImpl, name string) StringField {
	return stringField{field: field[string]{expr: &columnNode{qualifier: base.Qualifier(), name: name}, name: name}}
}

// ilikePredicate renders a case-insensitive LIKE, choosing ILIKE on PostgreSQL
// and LIKE elsewhere.
type ilikePredicate struct {
	operand node
	pattern node
}

func (p *ilikePredicate) render(b *builder) {
	op := "LIKE"
	if b.dialect.Name() == "postgres" {
		op = "ILIKE"
	}
	p.operand.render(b)
	b.writeString(" ")
	b.writeString(op)
	b.writeString(" ")
	p.pattern.render(b)
}

// NumericField is a Field[T] augmented with arithmetic operators that produce
// new expression fields. Each operator has a value-taking form that binds its
// argument and a field-taking form (suffixed "Field") that renders the other
// field or expression as an identifier instead of binding a value.
type NumericField[T any] interface {
	Field[T]
	Add(v T) Field[T]
	Sub(v T) Field[T]
	Mul(v T) Field[T]
	Div(v T) Field[T]
	ModVal(v T) Field[T]

	AddField(other Field[T]) Field[T]
	SubField(other Field[T]) Field[T]
	MulField(other Field[T]) Field[T]
	DivField(other Field[T]) Field[T]
	ModField(other Field[T]) Field[T]

	// Neg returns a field that negates this expression, rendering "-(expr)".
	Neg() Field[T]
}

// numericField is the concrete NumericField.
type numericField[T any] struct {
	field[T]
}

func (n numericField[T]) arith(op string, v T) Field[T] {
	return field[T]{expr: &arithExpr{left: n.expr, op: op, right: bindOf(v)}, name: n.name}
}

// arithField builds an arithmetic expression whose right operand is another
// field's expression node, so the rendered SQL references the other column or
// expression directly instead of binding a value.
func (n numericField[T]) arithField(op string, other Field[T]) Field[T] {
	return field[T]{expr: &arithExpr{left: n.expr, op: op, right: exprOf(other)}, name: n.name}
}

func (n numericField[T]) Add(v T) Field[T]    { return n.arith("+", v) }
func (n numericField[T]) Sub(v T) Field[T]    { return n.arith("-", v) }
func (n numericField[T]) Mul(v T) Field[T]    { return n.arith("*", v) }
func (n numericField[T]) Div(v T) Field[T]    { return n.arith("/", v) }
func (n numericField[T]) ModVal(v T) Field[T] { return n.arith("%", v) }

func (n numericField[T]) AddField(other Field[T]) Field[T] { return n.arithField("+", other) }
func (n numericField[T]) SubField(other Field[T]) Field[T] { return n.arithField("-", other) }
func (n numericField[T]) MulField(other Field[T]) Field[T] { return n.arithField("*", other) }
func (n numericField[T]) DivField(other Field[T]) Field[T] { return n.arithField("/", other) }
func (n numericField[T]) ModField(other Field[T]) Field[T] { return n.arithField("%", other) }

func (n numericField[T]) Neg() Field[T] {
	return field[T]{expr: &negExpr{operand: n.expr}, name: n.name}
}

// NewNumericField builds a qualified numeric column field.
func NewNumericField[T any](base TableImpl, name string) NumericField[T] {
	return numericField[T]{field: field[T]{expr: &columnNode{qualifier: base.Qualifier(), name: name}, name: name}}
}
