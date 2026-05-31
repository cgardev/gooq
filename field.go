package gooq

// AnyField is a type-erased field usable where the element type does not matter:
// projection column lists, GROUP BY, INSERT column lists, and RETURNING.
type AnyField interface {
	node
	// Name returns the unqualified column or expression name.
	Name() string
}

// Field is a typed column or expression. Comparison methods take a value of the
// field's own type T, so the compiler rejects mismatched comparisons. Field is
// the Go counterpart of jOOQ's Field<T>.
type Field[T any] interface {
	node
	Name() string

	// Comparisons against a value of the field's type.
	EQ(v T) Condition
	NE(v T) Condition
	GT(v T) Condition
	LT(v T) Condition
	GE(v T) Condition
	LE(v T) Condition

	// Set membership, nullability, and range.
	In(vs ...T) Condition
	NotIn(vs ...T) Condition
	IsNull() Condition
	IsNotNull() Condition
	Between(lo, hi T) Condition
	NotBetween(lo, hi T) Condition

	// Field-to-field comparisons. Each compares this field against another
	// field or expression, rendering the right operand as an identifier or
	// expression rather than a bound argument.
	EQField(other Field[T]) Condition
	NEField(other Field[T]) Condition
	GTField(other Field[T]) Condition
	LTField(other Field[T]) Condition
	GEField(other Field[T]) Condition
	LEField(other Field[T]) Condition

	// Aliasing, ordering, and assignment.
	As(alias string) Field[T]
	Asc() OrderField
	Desc() OrderField
	Set(v T) Assignment
}

// field is the single concrete implementation of Field[T]. It is parameterized
// by an underlying expression node (a column, an alias, or an arithmetic
// expression), to which all rendering is delegated. Operator methods build
// predicate nodes using that expression as the left operand.
type field[T any] struct {
	expr node
	name string
}

func (f field[T]) render(b *builder) { f.expr.render(b) }
func (f field[T]) Name() string      { return f.name }

func (f field[T]) cmp(op string, v T) Condition {
	return newCondition(&binaryPredicate{left: f.expr, op: op, right: bindOf(v)})
}

func (f field[T]) EQ(v T) Condition { return f.cmp("=", v) }
func (f field[T]) NE(v T) Condition { return f.cmp("<>", v) }
func (f field[T]) GT(v T) Condition { return f.cmp(">", v) }
func (f field[T]) LT(v T) Condition { return f.cmp("<", v) }
func (f field[T]) GE(v T) Condition { return f.cmp(">=", v) }
func (f field[T]) LE(v T) Condition { return f.cmp("<=", v) }

func (f field[T]) In(vs ...T) Condition {
	return newCondition(&inPredicate{operand: f.expr, vals: bindsOf(vs)})
}

func (f field[T]) NotIn(vs ...T) Condition {
	return newCondition(&inPredicate{operand: f.expr, vals: bindsOf(vs), negated: true})
}

func (f field[T]) IsNull() Condition {
	return newCondition(&nullPredicate{operand: f.expr})
}

func (f field[T]) IsNotNull() Condition {
	return newCondition(&nullPredicate{operand: f.expr, negated: true})
}

func (f field[T]) Between(lo, hi T) Condition {
	return newCondition(&betweenPredicate{operand: f.expr, lo: bindOf(lo), hi: bindOf(hi)})
}

func (f field[T]) NotBetween(lo, hi T) Condition {
	return newCondition(&betweenPredicate{operand: f.expr, lo: bindOf(lo), hi: bindOf(hi), negated: true})
}

// cmpField builds a comparison predicate whose right operand is another field's
// expression node, so the rendered SQL references the other column or expression
// directly instead of binding a value.
func (f field[T]) cmpField(op string, other Field[T]) Condition {
	return newCondition(&binaryPredicate{left: f.expr, op: op, right: exprOf(other)})
}

func (f field[T]) EQField(other Field[T]) Condition { return f.cmpField("=", other) }
func (f field[T]) NEField(other Field[T]) Condition { return f.cmpField("<>", other) }
func (f field[T]) GTField(other Field[T]) Condition { return f.cmpField(">", other) }
func (f field[T]) LTField(other Field[T]) Condition { return f.cmpField("<", other) }
func (f field[T]) GEField(other Field[T]) Condition { return f.cmpField(">=", other) }
func (f field[T]) LEField(other Field[T]) Condition { return f.cmpField("<=", other) }

func (f field[T]) As(alias string) Field[T] {
	return field[T]{expr: &aliasNode{inner: f.expr, alias: alias}, name: alias}
}

func (f field[T]) Asc() OrderField  { return &orderTerm{expr: f.expr, dir: "ASC"} }
func (f field[T]) Desc() OrderField { return &orderTerm{expr: f.expr, dir: "DESC"} }

func (f field[T]) Set(v T) Assignment {
	return &assignmentNode{column: f.name, val: bindOf(v)}
}

// NewField builds a typed column field qualified by the owning table's name or
// alias. Generated table code calls this for each column.
func NewField[T any](base TableImpl, name string) Field[T] {
	return field[T]{expr: &columnNode{qualifier: base.Qualifier(), name: name}, name: name}
}
