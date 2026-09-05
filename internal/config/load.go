package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ErrEmptyPath is returned by Load when it is called with an empty path.
var ErrEmptyPath = errors.New("no configuration file path provided")

// Load reads and parses the aur-pipelines configuration from the YAML file at
// the given path, then validates it against the embedded JSON Schema. It
// returns a fully populated Config, or an error describing why the
// configuration could not be read, parsed, or validated.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}

	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading configuration file %q: %w", path, err)
	}

	parser := viper.New()
	parser.SetConfigFile(path)
	parser.SetConfigType("yaml")

	err = parser.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("parsing configuration file %q: %w", path, err)
	}

	// Validate the raw parsed document (which still contains any unknown keys)
	// against the schema before decoding into the struct, so structural issues
	// in the source file are reported rather than silently discarded.
	err = validateSettings(parser.AllSettings())
	if err != nil {
		return nil, fmt.Errorf("validating configuration file %q: %w", path, err)
	}

	cfg := &Config{}

	err = parser.Unmarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("decoding configuration file %q: %w", path, err)
	}

	return cfg, nil
}
