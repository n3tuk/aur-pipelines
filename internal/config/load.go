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
// the given path. It returns a fully populated Config, or an error describing
// why the configuration could not be read or parsed.
//
// JSON-schema validation of the parsed configuration is added in a later task;
// Load currently performs structural YAML parsing only.
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

	cfg := &Config{}

	err = parser.Unmarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("decoding configuration file %q: %w", path, err)
	}

	return cfg, nil
}
