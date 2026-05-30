package gooq

// OrderField is a single ORDER BY term: an expression paired with a sort
// direction. It is produced by Field[T].Asc and Field[T].Desc.
type OrderField interface {
	node
	orderTerm()
}

// orderTerm is the concrete OrderField.
type orderTerm struct {
	expr node
	dir  string // "ASC" or "DESC"
}

func (o *orderTerm) render(b *builder) {
	o.expr.render(b)
	b.writeString(" ")
	b.writeString(o.dir)
}

func (o *orderTerm) orderTerm() {}
