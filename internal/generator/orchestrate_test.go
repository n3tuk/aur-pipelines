package generator_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/generator"
	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

// fakeSource is an in-memory resolver.Source for end-to-end tests, so the
// generate flow can be exercised without any network access.
type fakeSource struct {
	packages map[string]resolver.PackageInfo
}

func (f fakeSource) Lookup(_ context.Context, names ...string) (map[string]resolver.PackageInfo, error) {
	found := make(map[string]resolver.PackageInfo)

	for _, name := range names {
		pkg, ok := f.packages[name]
		if ok {
			found[name] = pkg
		}
	}

	return found, nil
}

func buildConfig(packages ...string) *config.Config {
	cfg := testConfig()
	cfg.Packages = make([]config.Package, 0, len(packages))

	for _, name := range packages {
		cfg.Packages = append(cfg.Packages, config.Package{Name: name})
	}

	return cfg
}

func TestBuildProducesPackageAndCleanupPipelines(t *testing.T) {
	t.Parallel()

	cfg := buildConfig(baseApp)
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		baseApp:    {Name: baseApp, PackageBase: baseApp, Dependencies: []string{baseLibDep}},
		baseLibDep: {Name: baseLibDep, PackageBase: baseLibDep},
	}}

	pipelines, result, err := generator.Build(t.Context(), cfg, source)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	bases := make([]string, 0, len(pipelines))
	for _, p := range pipelines {
		bases = append(bases, p.PackageBase)
	}

	// libdep, app (dependency-first), then cleanup.
	want := []string{baseLibDep, baseApp, "cleanup"}
	if !slices.Equal(bases, want) {
		t.Errorf("pipeline bases = %v, want %v", bases, want)
	}

	if !slices.Contains(result.PackageBases, baseLibDep) {
		t.Errorf("resolver result missing dependency base: %v", result.PackageBases)
	}
}

func TestWriteDirWritesOneFilePerPipeline(t *testing.T) {
	t.Parallel()

	cfg := buildConfig(baseKalcBin)
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		baseKalcBin: {Name: baseKalcBin, PackageBase: baseKalcBin},
	}}

	pipelines, _, err := generator.Build(t.Context(), cfg, source)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	dir := t.TempDir()

	written, err := generator.WriteDir(dir, pipelines)
	if err != nil {
		t.Fatalf("WriteDir() returned unexpected error: %v", err)
	}

	want := []string{"cleanup.yaml", "kalc-bin.yaml"}
	if !slices.Equal(written, want) {
		t.Errorf("written files = %v, want %v", written, want)
	}

	for _, name := range want {
		data, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			t.Errorf("expected file %q to be written: %v", name, readErr)

			continue
		}

		if !strings.HasPrefix(string(data), "---\n") {
			t.Errorf("file %q should start with a YAML document marker", name)
		}
	}
}

func TestWriteDirIsIdempotent(t *testing.T) {
	t.Parallel()

	cfg := buildConfig(baseKalcBin)
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		baseKalcBin: {Name: baseKalcBin, PackageBase: baseKalcBin},
	}}

	pipelines, _, err := generator.Build(t.Context(), cfg, source)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	dir := t.TempDir()

	_, err = generator.WriteDir(dir, pipelines)
	if err != nil {
		t.Fatalf("first WriteDir() error: %v", err)
	}

	first, err := os.ReadFile(filepath.Join(dir, "kalc-bin.yaml"))
	if err != nil {
		t.Fatalf("reading first output: %v", err)
	}

	_, err = generator.WriteDir(dir, pipelines)
	if err != nil {
		t.Fatalf("second WriteDir() error: %v", err)
	}

	second, err := os.ReadFile(filepath.Join(dir, "kalc-bin.yaml"))
	if err != nil {
		t.Fatalf("reading second output: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("WriteDir() output is not idempotent across runs")
	}
}

func TestWriteStreamDryRun(t *testing.T) {
	t.Parallel()

	cfg := buildConfig(baseKalcBin)
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		baseKalcBin: {Name: baseKalcBin, PackageBase: baseKalcBin},
	}}

	pipelines, _, err := generator.Build(t.Context(), cfg, source)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}

	var buffer bytes.Buffer

	err = generator.WriteStream(&buffer, pipelines)
	if err != nil {
		t.Fatalf("WriteStream() returned unexpected error: %v", err)
	}

	out := buffer.String()

	// Both the package pipeline and the cleanup pipeline must be present, each
	// preceded by a comment naming its file.
	for _, marker := range []string{"# kalc-bin.yaml", "# cleanup.yaml"} {
		if !strings.Contains(out, marker) {
			t.Errorf("dry-run output missing %q:\n%s", marker, out)
		}
	}
}
