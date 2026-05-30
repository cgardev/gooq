package gooq

// StringField is a Field[string] augmented with pattern-matching predicates that
// only make sense for text columns.
type StringField interface {
	Field[string]
	Like(pattern string) Condition
	NotLike(pattern string) Condition
	// ILike performs a case-insensitive match. It maps to ILIKE on PostgreSQL
	// and to a plain LIKE on MySQL and SQLite, whose LIKE is already
	// case-insensitive for ASCII text.
	ILike(pattern string) Condition
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
// new expression fields.
type NumericField[T any] interface {
	Field[T]
	Add(v T) Field[T]
	Sub(v T) Field[T]
	Mul(v T) Field[T]
	Div(v T) Field[T]
}

// numericField is the concrete NumericField.
type numericField[T any] struct {
	field[T]
}

func (n numericField[T]) arith(op string, v T) Field[T] {
	return field[T]{expr: &arithExpr{left: n.expr, op: op, right: bindOf(v)}, name: n.name}
}

func (n numericField[T]) Add(v T) Field[T] { return n.arith("+", v) }
func (n numericField[T]) Sub(v T) Field[T] { return n.arith("-", v) }
func (n numericField[T]) Mul(v T) Field[T] { return n.arith("*", v) }
func (n numericField[T]) Div(v T) Field[T] { return n.arith("/", v) }

// NewNumericField builds a qualified numeric column field.
func NewNumericField[T any](base TableImpl, name string) NumericField[T] {
	return numericField[T]{field: field[T]{expr: &columnNode{qualifier: base.Qualifier(), name: name}, name: name}}
}
