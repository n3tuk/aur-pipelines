// Package resolver resolves an initial set of AUR package names, together with
// their AUR dependencies, into the complete, de-duplicated set of package bases
// that must have pipelines generated for them.
//
// Resolution walks the runtime, build-time, and test-time dependencies of each
// package (that is, depends, makedepends, and checkdepends; optional
// dependencies are not followed), maps each dependency to the AUR package base
// that satisfies it — including dependencies satisfied through another AUR
// package's provides — and ignores dependencies that are not present in the
// AUR (which are assumed to be satisfied by the official repositories). It
// records the dependency relationships as an edge list so that downstream
// pipelines can be chained in the correct order, and halts safely when a
// circular dependency is detected.
package resolver

import (
	"context"
	"errors"
)

type (
	// PackageInfo is the normalised metadata the resolver needs for a single
	// AUR package. Dependency and provides entries must already have any
	// version constraints stripped to bare package names.
	PackageInfo struct {
		// Name is the queried package name.
		Name string
		// PackageBase is the package base the package belongs to.
		PackageBase string
		// Version is the package version string.
		Version string
		// Dependencies is the union of runtime, build-time, and test-time
		// dependency names (depends, makedepends, checkdepends).
		Dependencies []string
		// Provides is the list of names this package provides.
		Provides []string
	}

	// Source supplies normalised package metadata for a set of names. It
	// abstracts the underlying AUR access (RPC and/or git) so the resolver can
	// be tested with a fake. Names that do not exist in the AUR must be absent
	// from the returned map (rather than causing an error), which is how the
	// resolver distinguishes AUR packages from official-repository packages.
	Source interface {
		// Lookup returns metadata for the subset of the given names that exist
		// in the AUR, keyed by the queried name.
		Lookup(ctx context.Context, names ...string) (map[string]PackageInfo, error)
	}

	// Edge records a dependency relationship between two package bases: From
	// depends on To. Both are package bases, not package names.
	Edge struct {
		// From is the dependent package base.
		From string
		// To is the depended-upon package base.
		To string
	}

	// Result is the outcome of resolution: the deterministic, de-duplicated,
	// ordered list of package bases requiring pipelines, the dependency edges
	// between those bases, and the official-repository dependency names that
	// were skipped (retained for logging and diagnostics).
	Result struct {
		// PackageBases is the ordered, de-duplicated set of package bases.
		PackageBases []string
		// Edges is the list of dependency relationships between package bases.
		Edges []Edge
		// Skipped is the sorted, de-duplicated set of dependency names that
		// were not found in the AUR and were therefore skipped.
		Skipped []string
	}
)

// ErrCircularDependency is returned when a circular dependency is detected
// between AUR package bases, which cannot be resolved into an ordered build.
var ErrCircularDependency = errors.New("circular dependency detected")
