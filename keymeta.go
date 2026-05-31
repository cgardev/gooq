package gooq

// ForeignKeyMeta describes a single foreign key constraint discovered during
// code generation and surfaced on the generated table accessors. It is plain
// data with no behavior so that the core package remains free of external
// dependencies and the generated metadata can be inspected at runtime.
type ForeignKeyMeta struct {
	// Name is the constraint name as stored in the database catalog.
	Name string
	// Columns are the local column names that participate in the foreign key,
	// in the order they appear in the constraint definition.
	Columns []string
	// RefTable is the unqualified name of the referenced table.
	RefTable string
	// RefColumns are the referenced column names, aligned positionally with
	// Columns.
	RefColumns []string
}

// UniqueKeyMeta describes a single unique constraint discovered during code
// generation. Like ForeignKeyMeta it is plain data with no behavior.
type UniqueKeyMeta struct {
	// Name is the constraint name as stored in the database catalog.
	Name string
	// Columns are the column names that participate in the unique constraint,
	// in the order they appear in the constraint definition.
	Columns []string
}
