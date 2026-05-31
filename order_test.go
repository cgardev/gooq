package gooq

import "testing"

// TestOrderByNulls verifies the NULLS FIRST and NULLS LAST qualifiers. Their
// rendering is dialect-independent, so a single dialect suffices for assertion.
func TestOrderByNulls(t *testing.T) {
	tests := []struct {
		name string
		stmt interface{ SQL() (string, []any, error) }
		want string
	}{
		{
			name: "asc nulls first",
			stmt: Select1(Book.ID).From(Book).OrderBy(Book.ID.Asc().NullsFirst()),
			want: `SELECT "book"."id" FROM "book" ORDER BY "book"."id" ASC NULLS FIRST`,
		},
		{
			name: "desc nulls last",
			stmt: Select1(Book.ID).From(Book).OrderBy(Book.ID.Desc().NullsLast()),
			want: `SELECT "book"."id" FROM "book" ORDER BY "book"."id" DESC NULLS LAST`,
		},
		{
			name: "mixed terms",
			stmt: Select1(Book.ID).From(Book).OrderBy(Book.Title.Asc().NullsFirst(), Book.ID.Desc()),
			want: `SELECT "book"."id" FROM "book" ORDER BY "book"."title" ASC NULLS FIRST, "book"."id" DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := tt.stmt.SQL()
			if err != nil {
				t.Fatalf("SQL error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
