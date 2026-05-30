package main

import (
	"fmt"
	"go/format"
	"strings"
	"testing"
)

// TestGeneratedRecordsAreValid asserts that generateRecords produces source that
// gofmt accepts and that every arity from minArity to maxArity is present.
func TestGeneratedRecordsAreValid(t *testing.T) {
	source := generateRecords()
	if _, err := format.Source([]byte(source)); err != nil {
		t.Fatalf("generateRecords output does not format: %v", err)
	}
	for n := minArity; n <= maxArity; n++ {
		marker := fmt.Sprintf("type Record%d[", n)
		if !strings.Contains(source, marker) {
			t.Errorf("missing %q in generated records", marker)
		}
	}
}

// TestGeneratedSelectsAreValid asserts that generateSelects produces source that
// gofmt accepts and that every arity from minArity to maxArity is present.
func TestGeneratedSelectsAreValid(t *testing.T) {
	source := generateSelects()
	if _, err := format.Source([]byte(source)); err != nil {
		t.Fatalf("generateSelects output does not format: %v", err)
	}
	for n := minArity; n <= maxArity; n++ {
		marker := fmt.Sprintf("func Select%d[", n)
		if !strings.Contains(source, marker) {
			t.Errorf("missing %q in generated selects", marker)
		}
	}
}
