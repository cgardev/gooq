package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cgardev/gooq/codegen"
)

// committedDBDir resolves the committed generated package directory relative to
// this test source file.
func committedDBDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "internal", "db")
}

// TestGeneratedAccessorsAreUpToDate proves the suite uses the code generator end
// to end: it regenerates the typed accessors from the live database schema into a
// temporary directory and asserts the output is byte-for-byte identical to the
// committed files under internal/db. A mismatch means the committed accessors are
// stale and must be regenerated with "go generate ./...".
func TestGeneratedAccessorsAreUpToDate(t *testing.T) {
	requireDatabase(t)
	ctx := context.Background()

	tempDir := t.TempDir()
	written, err := codegen.Generate(ctx, sharedDB, codegen.Options{
		Schema:  "public",
		OutDir:  tempDir,
		Package: "db",
	})
	noError(t, "generate", err)
	if len(written) == 0 {
		t.Fatal("generate produced no files")
	}

	committedDir := committedDBDir()
	for _, generatedPath := range written {
		name := filepath.Base(generatedPath)

		got, err := os.ReadFile(generatedPath)
		noError(t, "read generated "+name, err)
		want, err := os.ReadFile(filepath.Join(committedDir, name))
		noError(t, "read committed "+name, err)

		if string(got) != string(want) {
			t.Errorf("committed %s is out of date; run \"go generate ./...\"\n--- generated ---\n%s\n--- committed ---\n%s", name, got, want)
		}
	}
}
