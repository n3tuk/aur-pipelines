package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	_ "embed"
)

// schemaURL is the in-memory resource name the embedded schema is registered
// under with the compiler. It does not need to be resolvable over the network.
const schemaURL = "https://raw.githubusercontent.com/n3tuk/aur-pipelines/main/schemas/aur-pipelines.json"

var (
	// schemaJSON is the embedded copy of the configuration JSON Schema. It is
	// generated from tools/schema/main.go via `task go:schema` and kept in
	// step with the canonical schemas/aur-pipelines.json file (a test asserts
	// the two are identical).
	//
	//go:embed schema/aur-pipelines.json
	schemaJSON []byte

	// ErrInvalidConfig is returned when the configuration does not conform to
	// the schema. The underlying schema error is wrapped for detail.
	ErrInvalidConfig = errors.New("configuration failed schema validation")
)

// compileSchema compiles the embedded configuration schema. It is called once
// per validation and the result used to validate a single document.
func compileSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parsing embedded schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()

	err = compiler.AddResource(schemaURL, doc)
	if err != nil {
		return nil, fmt.Errorf("registering embedded schema: %w", err)
	}

	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compiling embedded schema: %w", err)
	}

	return schema, nil
}

// validateDocument validates an already-decoded configuration document (as
// produced by a YAML/JSON decoder into Go-native types) against the embedded
// JSON Schema. Validating the raw parsed document, rather than the decoded
// Config struct, means unknown keys and other structural issues in the user's
// actual input are reported rather than silently discarded during struct
// decoding.
func validateDocument(document any) error {
	schema, err := compileSchema()
	if err != nil {
		return err
	}

	err = schema.Validate(document)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	return nil
}

// validateSettings normalises a raw settings map (as returned by viper's
// AllSettings) into the JSON-native types the validator expects, then
// validates it against the embedded schema.
func validateSettings(settings map[string]any) error {
	encoded, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encoding configuration for validation: %w", err)
	}

	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("decoding configuration for validation: %w", err)
	}

	return validateDocument(document)
}

// Validate checks the configuration against the embedded JSON Schema by
// round-tripping it through JSON into the JSON-native types the validator
// expects. It returns nil when the configuration is valid, or an error
// wrapping ErrInvalidConfig describing why validation failed.
//
// Note that this validates the current in-memory Config; because struct
// decoding discards unknown keys, Load validates the raw parsed document
// instead so it can also report unknown keys in the source file.
func (c *Config) Validate() error {
	// errchkjson proves that *Config (composed only of strings, structs, and
	// slices thereof) cannot fail to marshal, so the error is always nil here;
	// it is still checked defensively.
	encoded, err := json.Marshal(c) //nolint:errchkjson // *Config marshalling is statically safe
	if err != nil {
		return fmt.Errorf("encoding configuration for validation: %w", err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("decoding configuration for validation: %w", err)
	}

	return validateDocument(instance)
}
