package main

import (
	"testing"

	"github.com/cgardev/gooq/codegen"
)

// TestTypeOverrideFlagSet verifies that the repeatable -type flag parses both a
// fully qualified column key and a SQL type key into a TypeOverride with the
// import path and package-qualified type split at the final dot.
func TestTypeOverrideFlagSet(t *testing.T) {
	flagValue := typeOverrideFlag{}

	if err := flagValue.Set("public.book.id=github.com/google/uuid.UUID"); err != nil {
		t.Fatalf("Set column key: %v", err)
	}
	if err := flagValue.Set("uuid=github.com/google/uuid.UUID"); err != nil {
		t.Fatalf("Set type key: %v", err)
	}

	want := codegen.TypeOverride{GoType: "uuid.UUID", Import: "github.com/google/uuid"}
	if got := flagValue["public.book.id"]; got != want {
		t.Errorf("column override = %+v, want %+v", got, want)
	}
	if got := flagValue["uuid"]; got != want {
		t.Errorf("type override = %+v, want %+v", got, want)
	}
}

// TestTypeOverrideFlagBareType verifies that a specification without a slash is
// treated as a builtin or already-imported type and produces no import.
func TestTypeOverrideFlagBareType(t *testing.T) {
	flagValue := typeOverrideFlag{}
	if err := flagValue.Set("public.book.flags=int64"); err != nil {
		t.Fatalf("Set bare type: %v", err)
	}
	want := codegen.TypeOverride{GoType: "int64"}
	if got := flagValue["public.book.flags"]; got != want {
		t.Errorf("bare override = %+v, want %+v", got, want)
	}
}

// TestTypeOverrideFlagInvalid verifies that malformed specifications are
// rejected with an error rather than silently accepted.
func TestTypeOverrideFlagInvalid(t *testing.T) {
	cases := []string{
		"",                                      // empty
		"public.book.id",                        // missing '='
		"=github.com/google/uuid.UUID",          // missing key
		"public.book.id=",                       // missing spec
		"public.book.id=github.com/google/uuid", // missing type name after final segment
	}
	for _, c := range cases {
		flagValue := typeOverrideFlag{}
		if err := flagValue.Set(c); err == nil {
			t.Errorf("Set(%q) = nil error, want a parse error", c)
		}
	}
}

// TestQualifiedRoundTrip verifies that qualified reconstructs the fully
// qualified type expression used to display the flag's current value.
func TestQualifiedRoundTrip(t *testing.T) {
	override := codegen.TypeOverride{GoType: "uuid.UUID", Import: "github.com/google/uuid"}
	if got, want := qualified(override), "github.com/google/uuid.UUID"; got != want {
		t.Errorf("qualified = %q, want %q", got, want)
	}
	bare := codegen.TypeOverride{GoType: "int64"}
	if got, want := qualified(bare), "int64"; got != want {
		t.Errorf("qualified bare = %q, want %q", got, want)
	}
}
