package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/n3tuk/aur-pipelines/internal/aur"
)

// RPCSource is a Source backed by the AUR RPC interface. A single batched info
// query returns a package's base, its dependency lists, and its provides, so
// the RPC source satisfies membership, base mapping, dependency data, and
// provides aliasing without cloning each repository.
type RPCSource struct {
	client *aur.Client
}

// compile-time assertion that RPCSource satisfies Source.
var _ Source = (*RPCSource)(nil)

// NewRPCSource constructs an RPCSource using the given AUR client.
func NewRPCSource(client *aur.Client) *RPCSource {
	return &RPCSource{client: client}
}

// Lookup queries the AUR RPC info endpoint for the given names and returns the
// normalised metadata for those that exist in the AUR, keyed by name. Names not
// present in the AUR are absent from the result. Version constraints on
// dependency and provides entries are stripped to bare names.
func (s *RPCSource) Lookup(ctx context.Context, names ...string) (map[string]PackageInfo, error) {
	packages, err := s.client.Info(ctx, names...)
	if err != nil {
		return nil, fmt.Errorf("looking up packages via rpc: %w", err)
	}

	result := make(map[string]PackageInfo, len(packages))

	for _, pkg := range packages {
		result[pkg.Name] = PackageInfo{
			Name:         pkg.Name,
			PackageBase:  pkg.PackageBase,
			Version:      pkg.Version,
			Dependencies: dependencyNames(pkg),
			Provides:     stripConstraints(pkg.Provides),
		}
	}

	return result, nil
}

// dependencyNames returns the combined, constraint-stripped runtime,
// build-time, and test-time dependency names for a package. Optional
// dependencies are intentionally excluded.
func dependencyNames(pkg aur.Package) []string {
	combined := make([]string, 0, len(pkg.Depends)+len(pkg.MakeDepends)+len(pkg.CheckDepends))
	combined = append(combined, pkg.Depends...)
	combined = append(combined, pkg.MakeDepends...)
	combined = append(combined, pkg.CheckDepends...)

	return stripConstraints(combined)
}

// stripConstraints removes version constraints and optional-dependency
// descriptions from each entry, returning bare package names with empty entries
// removed.
func stripConstraints(entries []string) []string {
	stripped := make([]string, 0, len(entries))

	for _, entry := range entries {
		name := stripConstraint(entry)
		if name != "" {
			stripped = append(stripped, name)
		}
	}

	return stripped
}

// stripConstraint reduces a single dependency or provides entry to its bare
// package name by removing any optional-dependency description and version
// comparison.
func stripConstraint(entry string) string {
	name, _, found := strings.Cut(entry, ":")
	if found {
		entry = name
	}

	idx := strings.IndexAny(entry, "<>=")
	if idx >= 0 {
		entry = entry[:idx]
	}

	return strings.TrimSpace(entry)
}
