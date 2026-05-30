package gooq

// Table is a SQL table reference usable in FROM and JOIN clauses. Generated
// table types embed the exported TableImpl base and expose typed Field columns.
//
// Aliasing is intentionally not part of this interface: a concrete table type
// declares its own As(alias) method returning its own concrete type, so that
// the aliased table still exposes typed columns (qualified by the alias). The
// aliased value continues to satisfy Table for use in FROM and JOIN.
type Table interface {
	node
	// TableName returns the unqualified table name.
	TableName() string
}

// TableImpl is the embeddable base for generated table types. It renders the
// table reference in a FROM or JOIN position, including an optional schema and
// alias. External generated code embeds TableImpl, which supplies the
// unexported render method and thereby satisfies the sealed Table interface.
type TableImpl struct {
	schema string
	name   string
	alias  string
}

// NewTable creates a table base with the given name.
func NewTable(name string) TableImpl {
	return TableImpl{name: name}
}

// WithSchema returns a copy of the table base qualified by the given schema.
func (t TableImpl) WithSchema(schema string) TableImpl {
	t.schema = schema
	return t
}

// WithAlias returns a copy of the table base carrying the given alias.
// Generated table types call this from their own As method, then rebuild their
// typed columns so that each column is qualified by the alias.
func (t TableImpl) WithAlias(alias string) TableImpl {
	t.alias = alias
	return t
}

// TableName returns the unqualified table name.
func (t TableImpl) TableName() string { return t.name }

// Qualifier returns the identifier used to qualify this table's columns: the
// alias when aliased, otherwise the table name.
func (t TableImpl) Qualifier() string {
	if t.alias != "" {
		return t.alias
	}
	return t.name
}

// render writes the table reference in a FROM or JOIN position. It remains
// unexported so that embedding TableImpl is the only way for an external type
// to satisfy the sealed Table interface.
func (t TableImpl) render(b *builder) {
	if t.schema != "" {
		b.writeIdentifier(t.schema, t.name)
	} else {
		b.writeIdentifier(t.name)
	}
	if t.alias != "" {
		b.writeString(" ")
		b.writeIdentifier(t.alias)
	}
}
