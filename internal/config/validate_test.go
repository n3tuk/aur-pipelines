package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/config"
)

func TestLoadRejectsInvalidConfigs(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"invalid-missing-bucket.yaml",
		"invalid-empty-packages.yaml",
		"invalid-unknown-key.yaml",
		"invalid-webhook-enum.yaml",
		"invalid-webhook-secret.yaml",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(filepath.Join("testdata", fixture))
			if err == nil {
				t.Fatalf("Load(%q) = nil error, want a validation error", fixture)
			}

			if !errors.Is(err, config.ErrInvalidConfig) {
				t.Errorf("Load(%q) error = %v, want it to wrap ErrInvalidConfig", fixture, err)
			}
		})
	}
}

func TestLoadAcceptsValidConfigs(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"valid.yaml",
		"minimal.yaml",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(filepath.Join("testdata", fixture))
			if err != nil {
				t.Errorf("Load(%q) returned unexpected error: %v", fixture, err)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	valid := &config.Config{
		Container: config.Container{
			Build:   config.Image{Image: imageArchLinux, Tag: tagBaseDevel},
			Sign:    config.Image{Image: imageArchLinux, Tag: tagBaseDevel},
			Upload:  config.Image{Image: imageArchLinux, Tag: tagBaseDevel},
			Cleanup: config.Image{Image: "amazon/aws-cli", Tag: "latest"},
		},
		Bucket:   config.Bucket{Name: "bucket", Repository: "private"},
		Packages: []config.Package{{Name: packageKalcBin}},
	}

	err := valid.Validate()
	if err != nil {
		t.Errorf("Validate() on a valid config returned error: %v", err)
	}

	// An empty config must fail validation (missing required properties).
	empty := &config.Config{}

	err = empty.Validate()
	if err == nil {
		t.Error("Validate() on an empty config = nil, want an error")
	}

	if err != nil && !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("Validate() error = %v, want it to wrap ErrInvalidConfig", err)
	}
}

// TestEmbeddedSchemaMatchesCommitted guards against the embedded schema copy
// drifting from the canonical published schema. Both are produced by
// `task go:schema`; if this fails, regenerate them.
func TestEmbeddedSchemaMatchesCommitted(t *testing.T) {
	t.Parallel()

	embedded, err := os.ReadFile(filepath.Join("schema", "aur-pipelines.json"))
	if err != nil {
		t.Fatalf("reading embedded schema: %v", err)
	}

	canonical, err := os.ReadFile(filepath.Join("..", "..", "schemas", "aur-pipelines.json"))
	if err != nil {
		t.Fatalf("reading canonical schema: %v", err)
	}

	if string(embedded) != string(canonical) {
		t.Error("embedded schema differs from canonical schemas/aur-pipelines.json; run `task go:schema`")
	}
}
