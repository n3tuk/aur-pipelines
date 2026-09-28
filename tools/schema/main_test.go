package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestSchemaMatchesCommittedFiles is a golden test asserting that the schema
// produced by generate() matches every committed output file. The comparison
// is semantic (the parsed JSON structures are compared) rather than
// byte-for-byte, because the on-disk files are reformatted by Prettier as part
// of the repository's lint step, which changes their whitespace but not their
// meaning. If this fails, run `task go:schema` to regenerate the files.
//
//nolint:paralleltest // uses t.Chdir, which is incompatible with t.Parallel
func TestSchemaMatchesCommittedFiles(t *testing.T) {
	root := repositoryRoot(t)
	generated := decodeJSON(t, generateFromRoot(t))

	for _, rel := range outputPaths() {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading committed schema %q (run `task go:schema`?): %v", rel, err)
		}

		if !reflect.DeepEqual(decodeJSON(t, raw), generated) {
			t.Errorf("committed schema %q is out of date; run `task go:schema` to regenerate", rel)
		}
	}
}

// TestSchemaIsValidJSON asserts the generated schema is well-formed JSON.
//
//nolint:paralleltest // uses t.Chdir (via generateFromRoot), incompatible with t.Parallel
func TestSchemaIsValidJSON(t *testing.T) {
	_ = decodeJSON(t, generateFromRoot(t))
}

// decodeJSON parses JSON bytes into a generic structure, failing the test on
// error.
func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()

	var decoded any

	err := json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("decoding JSON: %v", err)
	}

	return decoded
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
