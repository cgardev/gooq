package gooq

import "testing"

func TestBuilderWriteIdentifierQuotingPerDialect(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		parts   []string
		want    string
	}{
		{"postgres qualified", Postgres(), []string{"book", "title"}, `"book"."title"`},
		{"sqlite qualified", SQLite(), []string{"book", "title"}, `"book"."title"`},
		{"mysql qualified", MySQL(), []string{"book", "title"}, "`book`.`title`"},
		{"postgres single", Postgres(), []string{"id"}, `"id"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newBuilder(tc.dialect)
			b.writeIdentifier(tc.parts...)
			if got := b.sql.String(); got != tc.want {
				t.Fatalf("writeIdentifier = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuilderIdentifierEscaping(t *testing.T) {
	b := newBuilder(Postgres())
	b.writeIdentifier(`we"ird`)
	if got, want := b.sql.String(), `"we""ird"`; got != want {
		t.Fatalf("escaping = %q, want %q", got, want)
	}

	b2 := newBuilder(MySQL())
	b2.writeIdentifier("ba`d")
	if got, want := b2.sql.String(), "`ba``d`"; got != want {
		t.Fatalf("mysql escaping = %q, want %q", got, want)
	}
}

func TestBuilderBindPlaceholdersIncrement(t *testing.T) {
	b := newBuilder(Postgres())
	b.bind(10)
	b.writeString(", ")
	b.bind("x")
	b.writeString(", ")
	b.bind(true)
	if got, want := b.sql.String(), "$1, $2, $3"; got != want {
		t.Fatalf("placeholders = %q, want %q", got, want)
	}
	if len(b.args) != 3 || b.args[0] != 10 || b.args[1] != "x" || b.args[2] != true {
		t.Fatalf("args = %v", b.args)
	}
}

func TestBuilderBindPlaceholdersMySQLSQLite(t *testing.T) {
	for _, d := range []Dialect{MySQL(), SQLite()} {
		b := newBuilder(d)
		b.bind(1)
		b.bind(2)
		if got, want := b.sql.String(), "??"; got != want {
			t.Fatalf("%s placeholders = %q, want %q", d.Name(), got, want)
		}
	}
}

func TestBuilderResultPropagatesError(t *testing.T) {
	b := newBuilder(Postgres())
	b.writeString("SELECT 1")
	b.setError(ErrTooManyRows)
	if _, _, err := b.result(); err != ErrTooManyRows {
		t.Fatalf("result err = %v, want ErrTooManyRows", err)
	}
}
