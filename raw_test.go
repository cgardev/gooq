package gooq

import "testing"

// TestRaw verifies the verbatim Raw field and RawCondition escape hatches, which
// bind no arguments and render identically across dialects.
func TestRaw(t *testing.T) {
	for _, d := range []Dialect{Postgres(), SQLite()} {
		t.Run(d.Name(), func(t *testing.T) {
			f := Raw[int64]("COUNT(*)")
			if got, args := renderNode(d, f); got != "COUNT(*)" || len(args) != 0 {
				t.Errorf("Raw: got %q args=%v, want %q with no args", got, args, "COUNT(*)")
			}

			c := RawCondition("1 = 1")
			if got, args := renderNode(d, c); got != "1 = 1" || len(args) != 0 {
				t.Errorf("RawCondition: got %q args=%v, want %q with no args", got, args, "1 = 1")
			}
		})
	}
}

// TestRawValue verifies that RawValue interleaves dialect placeholders for each
// '?' marker and binds the supplied arguments in order.
func TestRawValue(t *testing.T) {
	tests := []struct {
		name     string
		dialect  Dialect
		want     string
		wantArgs []any
	}{
		{
			name:     "postgres two markers",
			dialect:  Postgres(),
			want:     `id + $1 > $2`,
			wantArgs: []any{int64(5), int64(10)},
		},
		{
			name:     "sqlite two markers",
			dialect:  SQLite(),
			want:     `id + ? > ?`,
			wantArgs: []any{int64(5), int64(10)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := RawValue[int64]("id + ? > ?", int64(5), int64(10))
			got, args := renderNode(tt.dialect, f)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("got %d args, want %d", len(args), len(tt.wantArgs))
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Errorf("arg %d: got %v, want %v", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

// TestRawValueNoMarkers verifies that RawValue with no markers and no arguments
// behaves like a plain literal.
func TestRawValueNoMarkers(t *testing.T) {
	f := RawValue[int64]("42")
	if got, args := renderNode(Postgres(), f); got != "42" || len(args) != 0 {
		t.Errorf("got %q args=%v, want %q with no args", got, args, "42")
	}
}
