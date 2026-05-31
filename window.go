package gooq

// This file holds the window-function catalog: an OVER (...) node, the
// node-local constructors for ranking, value, and windowed-aggregate functions,
// and the builder that attaches a PARTITION BY / ORDER BY window specification.
// Each construct is a self-rendering node that delegates only genuinely
// dialect-specific fragments to the active Dialect, and every public
// constructor returns a typed WindowField so the result composes as a
// projection column with the rest of the builder.

// overNode renders a window function call followed by its OVER (...) clause:
// "FN(args) OVER (PARTITION BY ... ORDER BY ... frame)". A bare OVER () is
// rendered when no partitioning, ordering, or frame is supplied, which both
// PostgreSQL and SQLite accept.
type overNode struct {
	fn          *funcNode
	partitionBy []node
	orderBy     []*orderTerm
	frame       string
}

func (o *overNode) render(b *builder) {
	o.fn.render(b)
	b.writeString(" OVER (")
	wrote := false
	if len(o.partitionBy) > 0 {
		b.writeString("PARTITION BY ")
		renderList(b, o.partitionBy)
		wrote = true
	}
	if len(o.orderBy) > 0 {
		if wrote {
			b.writeString(" ")
		}
		b.writeString("ORDER BY ")
		for i, t := range o.orderBy {
			if i > 0 {
				b.writeString(", ")
			}
			t.render(b)
		}
		wrote = true
	}
	if o.frame != "" {
		if wrote {
			b.writeString(" ")
		}
		b.writeString(o.frame)
	}
	b.writeString(")")
}

// WindowField is a window-function expression awaiting its window specification.
// It is produced by the ranking, value, and windowed-aggregate constructors and
// finalised through Over, which yields a typed Field[T] usable as a projection
// column. When Over is not called the WindowField is not itself a Field; callers
// must always finalise it with Over().End(), even when the window is empty.
type WindowField[T any] struct {
	fn   *funcNode
	name string
}

// Over begins the window specification for this window function, returning a
// builder on which PartitionBy, OrderBy, and Frame may be set before End
// finalises the expression.
func (w WindowField[T]) Over() *OverBuilder[T] {
	return &OverBuilder[T]{
		over: &overNode{fn: w.fn},
		name: w.name,
	}
}

// OverBuilder accumulates the PARTITION BY, ORDER BY, and frame components of a
// window specification. It is returned by WindowField.Over and finalised with
// End, which yields a typed Field[T].
type OverBuilder[T any] struct {
	over *overNode
	name string
}

// PartitionBy adds one or more PARTITION BY expressions to the window
// specification. Each field renders as its own column or expression rather than
// binding a value. It returns the same builder for chaining.
func (o *OverBuilder[T]) PartitionBy(fields ...AnyField) *OverBuilder[T] {
	for _, f := range fields {
		o.over.partitionBy = append(o.over.partitionBy, f)
	}
	return o
}

// OrderBy adds one or more ORDER BY terms to the window specification, each
// produced by Field[T].Asc or Field[T].Desc. It returns the same builder for
// chaining.
func (o *OverBuilder[T]) OrderBy(orders ...OrderField) *OverBuilder[T] {
	for _, ord := range orders {
		if t, ok := ord.(*orderTerm); ok {
			o.over.orderBy = append(o.over.orderBy, t)
		}
	}
	return o
}

// Frame sets the verbatim frame clause of the window specification, for example
// "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW". The clause is spliced
// verbatim, so the caller supplies a spelling valid for the target dialect; both
// PostgreSQL and SQLite share the same frame syntax for the common cases.
func (o *OverBuilder[T]) Frame(frame string) *OverBuilder[T] {
	o.over.frame = frame
	return o
}

// End finalises the window specification and returns it as a typed Field[T]
// usable as a projection column.
func (o *OverBuilder[T]) End() Field[T] {
	return field[T]{expr: o.over, name: o.name}
}

// RowNumber returns the ROW_NUMBER() window function, assigning each row a
// distinct sequential number within its window, as a WindowField[int64].
func RowNumber() WindowField[int64] {
	return WindowField[int64]{fn: &funcNode{name: "ROW_NUMBER"}, name: "row_number"}
}

// Rank returns the RANK() window function, assigning each row a rank within its
// window with gaps after ties, as a WindowField[int64].
func Rank() WindowField[int64] {
	return WindowField[int64]{fn: &funcNode{name: "RANK"}, name: "rank"}
}

// DenseRank returns the DENSE_RANK() window function, assigning each row a rank
// within its window without gaps after ties, as a WindowField[int64].
func DenseRank() WindowField[int64] {
	return WindowField[int64]{fn: &funcNode{name: "DENSE_RANK"}, name: "dense_rank"}
}

// Lead returns the LEAD(field) window function, accessing the value of the field
// in a following row of the window, preserving the field's type.
func Lead[T any](f Field[T]) WindowField[T] {
	return WindowField[T]{fn: &funcNode{name: "LEAD", args: []node{f}}, name: "lead"}
}

// Lag returns the LAG(field) window function, accessing the value of the field
// in a preceding row of the window, preserving the field's type.
func Lag[T any](f Field[T]) WindowField[T] {
	return WindowField[T]{fn: &funcNode{name: "LAG", args: []node{f}}, name: "lag"}
}

// FirstValue returns the FIRST_VALUE(field) window function, the value of the
// field in the first row of the window, preserving the field's type.
func FirstValue[T any](f Field[T]) WindowField[T] {
	return WindowField[T]{fn: &funcNode{name: "FIRST_VALUE", args: []node{f}}, name: "first_value"}
}

// LastValue returns the LAST_VALUE(field) window function, the value of the
// field in the last row of the window, preserving the field's type.
func LastValue[T any](f Field[T]) WindowField[T] {
	return WindowField[T]{fn: &funcNode{name: "LAST_VALUE", args: []node{f}}, name: "last_value"}
}

// SumOver returns the SUM(field) aggregate evaluated as a window function,
// preserving the field's type.
func SumOver[T any](f Field[T]) WindowField[T] {
	return WindowField[T]{fn: &funcNode{name: "SUM", args: []node{f}}, name: "sum"}
}

// AvgOver returns the AVG(field) aggregate evaluated as a window function, as a
// WindowField[float64], since an average is computed in floating point
// regardless of the operand type.
func AvgOver[T any](f Field[T]) WindowField[float64] {
	return WindowField[float64]{fn: &funcNode{name: "AVG", args: []node{f}}, name: "avg"}
}

// CountOver returns the COUNT(field) aggregate evaluated as a window function,
// as a WindowField[int64].
func CountOver(f AnyField) WindowField[int64] {
	return WindowField[int64]{fn: &funcNode{name: "COUNT", args: []node{f}}, name: "count"}
}
