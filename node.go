package gooq

// node is the sealed contract every element of the abstract syntax tree
// satisfies. Because render is unexported, only types declared inside this
// package can satisfy interfaces that embed node. This seals the public
// Field, Condition, Table, OrderField, AnyField, and Assignment interfaces:
// callers may use them but cannot provide their own implementations.
type node interface {
	// render writes this element's SQL fragment into the builder, appending any
	// bind arguments in left-to-right order.
	render(b *builder)
}

// renderList renders a comma-separated list of nodes into the builder.
func renderList(b *builder, nodes []node) {
	for i, n := range nodes {
		if i > 0 {
			b.writeString(", ")
		}
		n.render(b)
	}
}
