package aur_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/aur"
)

func parseFixture(t *testing.T, name string) *aur.SrcInfo {
	t.Helper()

	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("opening fixture %q: %v", name, err)
	}

	t.Cleanup(func() {
		closeErr := file.Close()
		if closeErr != nil {
			t.Errorf("closing fixture: %v", closeErr)
		}
	})

	srcInfo, err := aur.ParseSrcInfo(file)
	if err != nil {
		t.Fatalf("ParseSrcInfo(%q) returned unexpected error: %v", name, err)
	}

	return srcInfo
}

func TestParseSrcInfoSingle(t *testing.T) {
	t.Parallel()

	got := parseFixture(t, "single.SRCINFO")

	if got.PackageBase != pkgKalcBin {
		t.Errorf("PackageBase = %q, want %q", got.PackageBase, pkgKalcBin)
	}

	if got.Version != "1.5.1-2" {
		t.Errorf("Version = %q, want %q", got.Version, "1.5.1-2")
	}

	if !slices.Equal(got.Packages, []string{pkgKalcBin}) {
		t.Errorf("Packages = %v, want [kalc-bin]", got.Packages)
	}

	// libgcc>=13.0 must have its constraint stripped to "libgcc".
	if !slices.Equal(got.Depends, []string{pkgGlibc, pkgLibgcc}) {
		t.Errorf("Depends = %v, want [glibc libgcc]", got.Depends)
	}

	if !slices.Equal(got.MakeDepends, []string{"cargo"}) {
		t.Errorf("MakeDepends = %v, want [cargo]", got.MakeDepends)
	}

	if !slices.Equal(got.Provides, []string{provKalc}) {
		t.Errorf("Provides = %v, want [kalc]", got.Provides)
	}
}

func TestParseSrcInfoSplit(t *testing.T) {
	t.Parallel()

	got := parseFixture(t, "split.SRCINFO")

	if got.PackageBase != "example-suite" {
		t.Errorf("PackageBase = %q, want %q", got.PackageBase, "example-suite")
	}

	// epoch:pkgver-pkgrel composition.
	if got.Version != "1:2.0.0-1" {
		t.Errorf("Version = %q, want %q", got.Version, "1:2.0.0-1")
	}

	if !slices.Equal(got.Packages, []string{"example-cli", "example-lib"}) {
		t.Errorf("Packages = %v, want [example-cli example-lib]", got.Packages)
	}

	// Depends aggregate across base + both pkgname sections, de-duplicated,
	// constraints stripped, and the arch-suffixed depends_x86_64 included.
	assertContains(t, "Depends", got.Depends, pkgGlibc, "libcurl", "intel-oneapi-mkl")

	assertContains(t, "MakeDepends", got.MakeDepends, "cmake", "ninja")

	// checkdepends with an "=" constraint stripped.
	assertContains(t, "CheckDepends", got.CheckDepends, "gtest")

	// provides aggregated across split packages.
	assertContains(t, "Provides", got.Provides, "example", "libexample.so")

	// glibc appears three times across sections but must be de-duplicated.
	if count := countOccurrences(got.Depends, pkgGlibc); count != 1 {
		t.Errorf("Depends contains glibc %d times, want 1 (de-duplicated)", count)
	}
}

func TestParseSrcInfoConstraints(t *testing.T) {
	t.Parallel()

	got := parseFixture(t, "constraints.SRCINFO")

	wantDepends := []string{"foo", "bar", "baz", "qux", "plain"}
	if !slices.Equal(got.Depends, wantDepends) {
		t.Errorf("Depends = %v, want %v (all constraints stripped)", got.Depends, wantDepends)
	}

	if !slices.Equal(got.MakeDepends, []string{"build-tool"}) {
		t.Errorf("MakeDepends = %v, want [build-tool]", got.MakeDepends)
	}

	if !slices.Equal(got.Provides, []string{"virtual", "another"}) {
		t.Errorf("Provides = %v, want [virtual another]", got.Provides)
	}
}

func TestParseSrcInfoEmpty(t *testing.T) {
	t.Parallel()

	got, err := aur.ParseSrcInfo(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseSrcInfo(empty) returned unexpected error: %v", err)
	}

	if got.PackageBase != "" {
		t.Errorf("PackageBase = %q, want empty", got.PackageBase)
	}

	if got.Version != "" {
		t.Errorf("Version = %q, want empty", got.Version)
	}
}

func TestParseSrcInfoNoPkgrel(t *testing.T) {
	t.Parallel()

	// A pkgver without pkgrel should compose to just the pkgver.
	got, err := aur.ParseSrcInfo(strings.NewReader("pkgbase = x\n\tpkgver = 1.0\n\npkgname = x\n"))
	if err != nil {
		t.Fatalf("ParseSrcInfo returned unexpected error: %v", err)
	}

	if got.Version != "1.0" {
		t.Errorf("Version = %q, want %q", got.Version, "1.0")
	}
}

func assertContains(t *testing.T, field string, got []string, want ...string) {
	t.Helper()

	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s = %v, want it to contain %q", field, got, w)
		}
	}
}

func countOccurrences(items []string, target string) int {
	count := 0

	for _, item := range items {
		if item == target {
			count++
		}
	}

	return count
}
