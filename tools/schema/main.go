// Command schema generates the JSON Schema describing the aur-pipelines
// configuration file and writes it to schemas/aur-pipelines.json (and an
// embedded copy under internal/config).
//
// The schema is reflected directly from the configuration structs in
// internal/config using github.com/invopop/jsonschema, so it always tracks the
// Go types: constraints (required, minLength, format, and so on) are expressed
// as `jsonschema:"..."` struct tags, and property descriptions are taken from
// the Go doc comments on each field. Run `task go:schema` whenever the
// configuration structs change so the generated files stay in step.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"

	"github.com/n3tuk/aur-pipelines/internal/config"
)

const (
	// schemaID is the canonical identifier for the configuration schema.
	schemaID = "https://raw.githubusercontent.com/n3tuk/aur-pipelines/main/schemas/aur-pipelines.json"
	// configModule is the module import path used as the base when parsing Go
	// comments; AddGoComments joins it with the relative configPath to form the
	// package path used as comment-map keys.
	configModule = "github.com/n3tuk/aur-pipelines"
	// configPath is the filesystem location (relative to the repository root)
	// of the package whose comments are parsed.
	configPath = "./internal/config"
	// filePerm is the permission mode used when writing the schema file.
	filePerm = 0o644
	// dirPerm is the permission mode used when creating the output directory.
	dirPerm = 0o755
)

func main() {
	err := run(os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// outputPaths returns the locations, relative to the repository root, that the
// generated schema is written to. The first is the canonical, published
// artefact referenced by editors and the schema $id; the second is the copy
// embedded into the config package for runtime validation. Both are written
// from the same document, and a test asserts they remain identical.
func outputPaths() []string {
	return []string{
		"schemas/aur-pipelines.json",
		"internal/config/schema/aur-pipelines.json",
	}
}

// run reflects the configuration schema, marshals it, and writes it to every
// output location, reporting progress to the provided writer.
func run(out *os.File) error {
	encoded, err := generate()
	if err != nil {
		return err
	}

	for _, path := range outputPaths() {
		err = write(path, encoded)
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "Wrote %s\n", path)
	}

	return nil
}

// generate reflects the configuration structs into a JSON Schema document and
// returns its marshalled, newline-terminated bytes.
func generate() ([]byte, error) {
	reflector := &jsonschema.Reflector{
		// Inline all definitions so the published schema is a single,
		// self-contained document rather than a set of $ref/$defs.
		DoNotReference: true,
	}

	// Use the Go doc comments on the config fields as property descriptions,
	// keeping the descriptions in a single place alongside the types.
	err := reflector.AddGoComments(configModule, configPath)
	if err != nil {
		return nil, fmt.Errorf("parsing Go comments: %w", err)
	}

	schema := reflector.Reflect(&config.Config{})
	schema.ID = schemaID

	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling schema: %w", err)
	}

	// Terminate the file with a trailing newline to satisfy the repository's
	// end-of-file conventions.
	encoded = append(encoded, '\n')

	return encoded, nil
}

// write persists the encoded schema to the given path, creating the parent
// directory if required.
func write(path string, encoded []byte) error {
	err := os.MkdirAll(filepath.Dir(path), dirPerm)
	if err != nil {
		return fmt.Errorf("creating output directory for %q: %w", path, err)
	}

	err = os.WriteFile(path, encoded, filePerm)
	if err != nil {
		return fmt.Errorf("writing schema to %q: %w", path, err)
	}

	return nil
}
