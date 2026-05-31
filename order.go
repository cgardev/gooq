package gooq

// OrderField is a single ORDER BY term: an expression paired with a sort
// direction and an optional NULLS ordering. It is produced by Field[T].Asc and
// Field[T].Desc.
type OrderField interface {
	node
	orderTerm()
	// NullsFirst orders NULL values before non-NULL values, rendering the
	// "NULLS FIRST" qualifier. It returns the same term for chaining.
	NullsFirst() OrderField
	// NullsLast orders NULL values after non-NULL values, rendering the
	// "NULLS LAST" qualifier. It returns the same term for chaining.
	NullsLast() OrderField
}

// orderTerm is the concrete OrderField.
type orderTerm struct {
	expr  node
	dir   string // "ASC" or "DESC"
	nulls string // "", "FIRST", or "LAST"
}

func (o *orderTerm) render(b *builder) {
	o.expr.render(b)
	b.writeString(" ")
	b.writeString(o.dir)
	if o.nulls != "" {
		b.writeString(" NULLS ")
		b.writeString(o.nulls)
	}
}

func (o *orderTerm) orderTerm() {}

func (o *orderTerm) NullsFirst() OrderField {
	o.nulls = "FIRST"
	return o
}

func (o *orderTerm) NullsLast() OrderField {
	o.nulls = "LAST"
	return o
}
