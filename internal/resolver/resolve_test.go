package resolver_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

// fakeSource is an in-memory Source for tests. Any name not present is treated
// as a non-AUR (official repository) package and omitted from Lookup results.
type fakeSource struct {
	packages map[string]resolver.PackageInfo
}

const (
	pkgKalcBin  = "kalc-bin"
	pkgGlibc    = "glibc"
	pkgExampleL = "example-lib"
)

// compile-time assertion that fakeSource satisfies Source.
var _ resolver.Source = fakeSource{}

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

func pkg(name string, deps []string, provides []string) resolver.PackageInfo {
	return resolver.PackageInfo{
		Name:         name,
		PackageBase:  name,
		Version:      "1-1",
		Dependencies: deps,
		Provides:     provides,
	}
}

func resolveNames(t *testing.T, source fakeSource, names ...string) *resolver.Result {
	t.Helper()

	res, err := resolver.New(source).Resolve(t.Context(), names...)
	if err != nil {
		t.Fatalf("Resolve(%v) returned unexpected error: %v", names, err)
	}

	return res
}

func TestResolveSinglePackageNoDeps(t *testing.T) {
	t.Parallel()

	source := fakeSource{packages: map[string]resolver.PackageInfo{
		pkgKalcBin: pkg(pkgKalcBin, nil, nil),
	}}

	res := resolveNames(t, source, pkgKalcBin)

	if !slices.Equal(res.PackageBases, []string{pkgKalcBin}) {
		t.Errorf("PackageBases = %v, want [kalc-bin]", res.PackageBases)
	}

	if len(res.Edges) != 0 {
		t.Errorf("Edges = %v, want none", res.Edges)
	}
}

func TestResolveLinearChain(t *testing.T) {
	t.Parallel()

	// a -> b -> c
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"a": pkg("a", []string{"b"}, nil),
		"b": pkg("b", []string{"c"}, nil),
		"c": pkg("c", nil, nil),
	}}

	res := resolveNames(t, source, "a")

	// Dependency-first order: c before b before a.
	if !slices.Equal(res.PackageBases, []string{"c", "b", "a"}) {
		t.Errorf("PackageBases = %v, want [c b a]", res.PackageBases)
	}

	wantEdges := []resolver.Edge{{From: "a", To: "b"}, {From: "b", To: "c"}}
	if !slices.Equal(res.Edges, wantEdges) {
		t.Errorf("Edges = %v, want %v", res.Edges, wantEdges)
	}
}

func TestResolveDiamond(t *testing.T) {
	t.Parallel()

	// a -> b, a -> c, b -> d, c -> d
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"a": pkg("a", []string{"b", "c"}, nil),
		"b": pkg("b", []string{"d"}, nil),
		"c": pkg("c", []string{"d"}, nil),
		"d": pkg("d", nil, nil),
	}}

	res := resolveNames(t, source, "a")

	// d must be de-duplicated to a single base.
	if count := countOccurrences(res.PackageBases, "d"); count != 1 {
		t.Errorf("d appears %d times, want 1", count)
	}

	// All four bases present.
	for _, base := range []string{"a", "b", "c", "d"} {
		if !slices.Contains(res.PackageBases, base) {
			t.Errorf("PackageBases missing %q: %v", base, res.PackageBases)
		}
	}

	// d must come before b and c, which must come before a.
	assertBefore(t, res.PackageBases, "d", "b")
	assertBefore(t, res.PackageBases, "d", "c")
	assertBefore(t, res.PackageBases, "b", "a")
	assertBefore(t, res.PackageBases, "c", "a")
}

func TestResolveProvidesAlias(t *testing.T) {
	t.Parallel()

	// a depends on "virtual-x", which no package is named, but package p
	// provides it. Resolution must map the dependency to base p.
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"a": pkg("a", []string{"virtual-x"}, nil),
		"p": pkg("p", nil, []string{"virtual-x"}),
	}}

	// Both a and p are supplied as roots so p's provides is discovered.
	res := resolveNames(t, source, "a", "p")

	if !slices.Contains(res.PackageBases, "p") {
		t.Errorf("PackageBases missing provider base p: %v", res.PackageBases)
	}

	wantEdge := resolver.Edge{From: "a", To: "p"}
	if !slices.Contains(res.Edges, wantEdge) {
		t.Errorf("Edges missing %v (provides alias): %v", wantEdge, res.Edges)
	}

	// virtual-x must not be reported as skipped, since it is satisfied.
	if slices.Contains(res.Skipped, "virtual-x") {
		t.Errorf("virtual-x was skipped but is provided by p: %v", res.Skipped)
	}
}

func TestResolveSkipsOfficialRepoDeps(t *testing.T) {
	t.Parallel()

	// a depends on glibc (not in the AUR) and on b (in the AUR).
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"a": pkg("a", []string{pkgGlibc, "b"}, nil),
		"b": pkg("b", []string{"gcc-libs"}, nil),
	}}

	res := resolveNames(t, source, "a")

	if !slices.Equal(res.PackageBases, []string{"b", "a"}) {
		t.Errorf("PackageBases = %v, want [b a]", res.PackageBases)
	}

	wantSkipped := []string{"gcc-libs", pkgGlibc}
	if !slices.Equal(res.Skipped, wantSkipped) {
		t.Errorf("Skipped = %v, want %v", res.Skipped, wantSkipped)
	}
}

func TestResolveCircular(t *testing.T) {
	t.Parallel()

	// a -> b -> a
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"a": pkg("a", []string{"b"}, nil),
		"b": pkg("b", []string{"a"}, nil),
	}}

	_, err := resolver.New(source).Resolve(t.Context(), "a")
	if !errors.Is(err, resolver.ErrCircularDependency) {
		t.Errorf("Resolve() error = %v, want ErrCircularDependency", err)
	}
}

func TestResolveSplitPackageDependency(t *testing.T) {
	t.Parallel()

	// The dependency name "example-lib" belongs to package base
	// "example-suite" (a split package). Depending on example-lib must resolve
	// to base example-suite, not example-lib.
	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"app": pkg("app", []string{pkgExampleL}, nil),
		pkgExampleL: {
			Name:        pkgExampleL,
			PackageBase: "example-suite",
			Version:     "2-1",
		},
	}}

	res := resolveNames(t, source, "app")

	if !slices.Contains(res.PackageBases, "example-suite") {
		t.Errorf("PackageBases missing base example-suite: %v", res.PackageBases)
	}

	if slices.Contains(res.PackageBases, pkgExampleL) {
		t.Errorf("PackageBases should use base example-suite, not name example-lib: %v", res.PackageBases)
	}

	wantEdge := resolver.Edge{From: "app", To: "example-suite"}
	if !slices.Contains(res.Edges, wantEdge) {
		t.Errorf("Edges missing %v: %v", wantEdge, res.Edges)
	}
}

func TestResolveRootNotInAUR(t *testing.T) {
	t.Parallel()

	source := fakeSource{packages: map[string]resolver.PackageInfo{}}

	res := resolveNames(t, source, "not-an-aur-package")

	if len(res.PackageBases) != 0 {
		t.Errorf("PackageBases = %v, want empty", res.PackageBases)
	}

	if !slices.Equal(res.Skipped, []string{"not-an-aur-package"}) {
		t.Errorf("Skipped = %v, want [not-an-aur-package]", res.Skipped)
	}
}

func TestResolveDeterministic(t *testing.T) {
	t.Parallel()

	source := fakeSource{packages: map[string]resolver.PackageInfo{
		"a": pkg("a", []string{"b", "c"}, nil),
		"b": pkg("b", []string{"d"}, nil),
		"c": pkg("c", []string{"d"}, nil),
		"d": pkg("d", nil, nil),
	}}

	first := resolveNames(t, source, "a")

	for range 5 {
		next := resolveNames(t, source, "a")
		if !slices.Equal(first.PackageBases, next.PackageBases) {
			t.Fatalf("PackageBases not deterministic: %v vs %v", first.PackageBases, next.PackageBases)
		}

		if !slices.Equal(first.Edges, next.Edges) {
			t.Fatalf("Edges not deterministic: %v vs %v", first.Edges, next.Edges)
		}
	}
}

func assertBefore(t *testing.T, order []string, earlier, later string) {
	t.Helper()

	earlierIdx := slices.Index(order, earlier)
	laterIdx := slices.Index(order, later)

	if earlierIdx == -1 || laterIdx == -1 {
		t.Fatalf("order %v missing %q or %q", order, earlier, later)
	}

	if earlierIdx >= laterIdx {
		t.Errorf("expected %q before %q in %v", earlier, later, order)
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
