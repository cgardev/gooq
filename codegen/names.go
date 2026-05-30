package codegen

import (
	"strings"
	"unicode"
)

// goKeywords is the set of reserved Go identifiers. A generated identifier that
// collides with one of these would not compile, so safeIdent rewrites it.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// splitSegments splits a snake_case (or kebab-case) identifier into its
// non-empty segments. Doubled or leading separators yield no empty segments.
func splitSegments(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	return fields
}

// camel converts a snake_case identifier to UpperCamelCase, skipping empty
// segments produced by doubled underscores. For example, "author_id" becomes
// "AuthorId" and "a__b" becomes "AB".
func camel(s string) string {
	var b strings.Builder
	for _, seg := range splitSegments(s) {
		runes := []rune(seg)
		b.WriteRune(unicode.ToUpper(runes[0]))
		if len(runes) > 1 {
			b.WriteString(string(runes[1:]))
		}
	}
	return b.String()
}

// lowerCamel converts a snake_case identifier to lowerCamelCase. The first
// segment is lowercased and subsequent segments are upper-cased on their first
// rune, so "book_table" becomes "bookTable".
func lowerCamel(s string) string {
	c := camel(s)
	if c == "" {
		return ""
	}
	runes := []rune(c)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// structName returns the unexported generated struct type name for a table,
// such as "bookTable" for the table "book".
func structName(table string) string {
	return safeIdent(lowerCamel(table)+"Table", "tbl")
}

// exportName returns the exported package-level accessor name for a table, such
// as "Book" for the table "book".
func exportName(table string) string {
	return safeIdent(camel(table), "Tbl")
}

// fieldName returns the exported struct field name for a column, such as
// "AuthorId" for the column "author_id".
func fieldName(col string) string {
	return safeIdent(camel(col), "Col")
}

// safeIdent guarantees that name is a valid, non-reserved Go identifier. An
// empty name (or a name that becomes empty after stripping invalid characters)
// falls back to fallback. Characters that are not letters, digits, or
// underscores are removed. A name beginning with a digit is prefixed with an
// underscore, and a name colliding with a Go keyword is suffixed with an
// underscore.
func safeIdent(name, fallback string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		}
	}
	name = b.String()

	if name == "" {
		name = fallback
	}
	runes := []rune(name)
	if unicode.IsDigit(runes[0]) {
		name = "_" + name
	}
	if goKeywords[name] {
		name = name + "_"
	}
	return name
}
