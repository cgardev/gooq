package gooq

// Assignment is a "column = value" pair used by UPDATE ... SET, by INSERT upsert
// update actions, and by INSERT ... SET. It is produced by Field[T].Set.
type Assignment interface {
	node
	assignment()
}

// assignmentNode renders "column = value" with the column unqualified, which is
// the correct form inside SET and conflict-update clauses.
type assignmentNode struct {
	column string
	val    node
}

func (a *assignmentNode) render(b *builder) {
	b.writeIdentifier(a.column)
	b.writeString(" = ")
	a.val.render(b)
}

func (a *assignmentNode) assignment() {}

// SetToExcluded builds an upsert assignment that sets a column to the value the
// conflicting INSERT attempted to write (EXCLUDED.col in PostgreSQL and
// excluded.col in SQLite). It is used inside DoUpdateSet.
func SetToExcluded(f AnyField) Assignment {
	return &assignmentNode{column: f.Name(), val: &excludedNode{column: f.Name()}}
}
