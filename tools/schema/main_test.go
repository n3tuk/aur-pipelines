package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaMatchesCommittedFiles is a golden test asserting that the schema
// produced by generate() exactly matches every committed output file. If this
// fails, run `task go:schema` to regenerate the files.
//
// This test uses t.Chdir, which is incompatible with t.Parallel.
//
//nolint:paralleltest // t.Chdir cannot be used in a parallel test
func TestSchemaMatchesCommittedFiles(t *testing.T) {
	root := repositoryRoot(t)
	encoded := generateFromRoot(t)

	for _, rel := range outputPaths() {
		path := filepath.Join(root, rel)

		committed, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading committed schema %q (run `task go:schema`?): %v", rel, err)
		}

		if string(committed) != string(encoded) {
			t.Errorf("committed schema %q is out of date; run `task go:schema` to regenerate", rel)
		}
	}
}

// TestSchemaIsValidJSON asserts the generated schema is well-formed JSON.
//
// This test uses t.Chdir (via generateFromRoot), which is incompatible with
// t.Parallel.
//
//nolint:paralleltest // t.Chdir cannot be used in a parallel test
func TestSchemaIsValidJSON(t *testing.T) {
	encoded := generateFromRoot(t)

	var decoded any

	err := json.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("generated schema is not valid JSON: %v", err)
	}
}

// repositoryRoot returns the absolute path to the repository root, which is two
// directories above this package (tools/schema).
func repositoryRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}

	return root
}

// generateFromRoot runs generate() with the working directory set to the
// repository root (as `task go:schema` does), because generate() reflects Go
// comments using a path relative to that root. t.Chdir scopes the change to
// the test and restores the original directory automatically.
func generateFromRoot(t *testing.T) []byte {
	t.Helper()

	t.Chdir(repositoryRoot(t))

	encoded, err := generate()
	if err != nil {
		t.Fatalf("generating schema: %v", err)
	}

	return encoded
}
