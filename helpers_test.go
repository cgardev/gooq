package gooq

// This file provides hand-written table definitions shaped exactly like the
// output of cmd/jooq-gen, so the tests exercise the same exported code paths a
// generated schema would: they embed TableImpl and build columns through the
// exported NewField/NewStringField/NewNumericField constructors.

type bookTable struct {
	TableImpl
	ID       Field[int64]
	Title    StringField
	Price    NumericField[float64]
	AuthorID Field[int64]
}

func newBookTable(alias string) *bookTable {
	base := NewTable("book").WithAlias(alias)
	return &bookTable{
		TableImpl: base,
		ID:        NewField[int64](base, "id"),
		Title:     NewStringField(base, "title"),
		Price:     NewNumericField[float64](base, "price"),
		AuthorID:  NewField[int64](base, "author_id"),
	}
}

// As returns the book table under an alias, with every column re-qualified.
func (b *bookTable) As(alias string) *bookTable { return newBookTable(alias) }

type authorTable struct {
	TableImpl
	ID   Field[int64]
	Name StringField
}

func newAuthorTable(alias string) *authorTable {
	base := NewTable("author").WithAlias(alias)
	return &authorTable{
		TableImpl: base,
		ID:        NewField[int64](base, "id"),
		Name:      NewStringField(base, "name"),
	}
}

func (a *authorTable) As(alias string) *authorTable { return newAuthorTable(alias) }

// Package-level singletons mirror the generated convention.
var (
	Book   = newBookTable("")
	Author = newAuthorTable("")
)
