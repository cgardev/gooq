package gooq

import "testing"

// TestStringConcatMethod verifies StringField.Concat mixes bound literals and
// field operands, rendering "||" chains with field operands as identifiers.
func TestStringConcatMethod(t *testing.T) {
	// field || literal || field: the literal binds, the fields render as columns.
	c1 := Author.Name.Concat(" ", Book.Title)
	assertSQL(t, Postgres(), c1, `("author"."name" || $1 || "book"."title")`, " ")
	assertSQL(t, SQLite(), c1, `("author"."name" || ? || "book"."title")`, " ")

	// field || field: both render as columns, nothing binds.
	c2 := Author.Name.Concat(Book.Title)
	assertSQL(t, Postgres(), c2, `("author"."name" || "book"."title")`)
	assertSQL(t, SQLite(), c2, `("author"."name" || "book"."title")`)
}

// TestStringConcatFunction verifies the free Concat function mixes value and
// field operands the same way as the method.
func TestStringConcatFunction(t *testing.T) {
	c := Concat("Mr. ", Author.Name, "!")
	assertSQL(t, Postgres(), c, `($1 || "author"."name" || $2)`, "Mr. ", "!")
	assertSQL(t, SQLite(), c, `(? || "author"."name" || ?)`, "Mr. ", "!")
}

// TestConcatAcceptsFieldString verifies Concat accepts a plain Field[string]
// (not only StringField) as a field operand.
func TestConcatAcceptsFieldString(t *testing.T) {
	var plain Field[string] = NewField[string](NewTable("t"), "code")
	c := Concat("X-", plain)
	assertSQL(t, Postgres(), c, `($1 || "t"."code")`, "X-")
	assertSQL(t, SQLite(), c, `(? || "t"."code")`, "X-")
}
