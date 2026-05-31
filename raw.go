package gooq

import "strings"

// Raw is an escape hatch that splices verbatim SQL into the query as a typed
// field, binding no arguments. The text is emitted exactly as given, so the
// caller is responsible for any required quoting and for the dialect
// portability of the fragment.
func Raw[T any](sql string) Field[T] {
	return field[T]{expr: &literalNode{sql: sql}, name: ""}
}

// RawCondition is an escape hatch that splices verbatim SQL into the query as a
// boolean predicate, binding no arguments.
func RawCondition(sql string) Condition {
	return newCondition(&literalNode{sql: sql})
}

// RawValue is an escape hatch that splices verbatim SQL into the query as a
// typed field while binding the supplied arguments. Each '?' marker in the SQL
// text is replaced, in order, by the active dialect's placeholder and consumes
// one argument from args. Surplus markers or surplus arguments are tolerated:
// extra markers render with no bound value and extra arguments are ignored, so
// the marker count and argument count are expected to match.
func RawValue[T any](sql string, args ...any) Field[T] {
	return field[T]{expr: &rawValueNode{sql: sql, args: args}, name: ""}
}

// rawValueNode renders verbatim SQL while interleaving dialect placeholders for
// each '?' marker and binding the corresponding argument in order.
type rawValueNode struct {
	sql  string
	args []any
}

func (r *rawValueNode) render(b *builder) {
	segments := strings.Split(r.sql, "?")
	for i, segment := range segments {
		b.writeString(segment)
		// A placeholder sits between every pair of adjacent segments. Bind the
		// next argument when one remains; otherwise leave the marker position
		// empty so a mismatch does not panic.
		if i < len(segments)-1 {
			if i < len(r.args) {
				b.bind(r.args[i])
			}
		}
	}
}
