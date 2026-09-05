package config_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/config"
)

const (
	imageArchLinux = "archlinux"
	imageReference = "archlinux:base-devel"
	tagBaseDevel   = "base-devel"
	packageKalcBin = "kalc-bin"
)

func TestLoadValid(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Bucket.Name != "your-bucket-name" {
		t.Errorf("Bucket.Name = %q, want %q", cfg.Bucket.Name, "your-bucket-name")
	}

	if cfg.Bucket.Repository != "private" {
		t.Errorf("Bucket.Repository = %q, want %q", cfg.Bucket.Repository, "private")
	}

	if got := cfg.Container.Build.Reference(); got != imageReference {
		t.Errorf("Container.Build.Reference() = %q, want %q", got, imageReference)
	}

	if len(cfg.Webhooks) != 1 {
		t.Fatalf("len(Webhooks) = %d, want 1", len(cfg.Webhooks))
	}

	webhook := cfg.Webhooks[0]
	if webhook.Name != "ntfy" {
		t.Errorf("Webhooks[0].Name = %q, want %q", webhook.Name, "ntfy")
	}

	if len(webhook.Headers) != 4 {
		t.Fatalf("len(Webhooks[0].Headers) = %d, want 4", len(webhook.Headers))
	}

	if webhook.Headers[0].Name != "Title" {
		t.Errorf("Headers[0].Name = %q, want %q", webhook.Headers[0].Name, "Title")
	}

	// The header value must be preserved verbatim, including templating.
	if !strings.Contains(webhook.Headers[0].Value, "{{ .PackageName }}") {
		t.Errorf("Headers[0].Value = %q, want it to contain the pass-through template", webhook.Headers[0].Value)
	}

	if !strings.Contains(webhook.Template, "{{ if eq .PipelineStatus") {
		t.Errorf("Webhooks[0].Template did not preserve the pass-through template: %q", webhook.Template)
	}

	wantPackages := []string{packageKalcBin, "ntfysh-bin", "keybase-bin"}
	if len(cfg.Packages) != len(wantPackages) {
		t.Fatalf("len(Packages) = %d, want %d", len(cfg.Packages), len(wantPackages))
	}

	for i, want := range wantPackages {
		if cfg.Packages[i].Name != want {
			t.Errorf("Packages[%d].Name = %q, want %q", i, cfg.Packages[i].Name, want)
		}
	}
}

func TestLoadMinimal(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(filepath.Join("testdata", "minimal.yaml"))
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if len(cfg.Webhooks) != 0 {
		t.Errorf("len(Webhooks) = %d, want 0 for a config without webhooks", len(cfg.Webhooks))
	}

	if len(cfg.Packages) != 1 {
		t.Errorf("len(Packages) = %d, want 1", len(cfg.Packages))
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path    string
		wantErr error
	}{
		"empty path": {
			path:    "",
			wantErr: config.ErrEmptyPath,
		},
		"missing file": {
			path:    filepath.Join("testdata", "does-not-exist.yaml"),
			wantErr: nil,
		},
		"malformed yaml": {
			path:    filepath.Join("testdata", "malformed.yaml"),
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Load(tc.path)
			if err == nil {
				t.Fatalf("Load(%q) = %+v, want an error", tc.path, cfg)
			}

			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("Load(%q) error = %v, want it to wrap %v", tc.path, err, tc.wantErr)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	var builder strings.Builder

	err = cfg.Summary(&builder)
	if err != nil {
		t.Fatalf("Summary() returned unexpected error: %v", err)
	}

	got := builder.String()

	wantContains := []string{
		"your-bucket-name",
		imageReference,
		"Webhooks:   1",
		"ntfy (4 header(s))",
		"Packages:   3",
		"kalc-bin",
	}

	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("Summary() output missing %q\noutput:\n%s", want, got)
		}
	}
}

func TestImageReference(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		image config.Image
		want  string
	}{
		"image and tag": {
			image: config.Image{Image: imageArchLinux, Tag: tagBaseDevel},
			want:  imageReference,
		},
		"image only": {
			image: config.Image{Image: imageArchLinux, Tag: ""},
			want:  imageArchLinux,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tc.image.Reference(); got != tc.want {
				t.Errorf("Reference() = %q, want %q", got, tc.want)
			}
		})
	}
}
