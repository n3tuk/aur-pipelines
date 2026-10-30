package pipeline

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// yamlIndent is the number of spaces used per indentation level in generated
// pipeline YAML, matching the repository's YAML style.
const yamlIndent = 2

// Marshal renders the pipeline as Concourse-compatible YAML. The output begins
// with a document start marker and uses two-space indentation. Secret
// references and pass-through templates are emitted verbatim as string values.
func (p *Pipeline) Marshal() ([]byte, error) {
	var buffer bytes.Buffer

	// A document start marker keeps the output consistent with the rest of the
	// repository's YAML files.
	buffer.WriteString("---\n")

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(yamlIndent)

	err := encoder.Encode(p)
	if err != nil {
		return nil, fmt.Errorf("marshalling pipeline: %w", err)
	}

	err = encoder.Close()
	if err != nil {
		return nil, fmt.Errorf("closing pipeline encoder: %w", err)
	}

	return buffer.Bytes(), nil
}
