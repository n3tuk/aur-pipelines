package resolver

import (
	"context"
	"fmt"
	"slices"
)

// Resolver resolves package names and their AUR dependencies into the complete
// set of package bases. Construct one with New.
type (
	Resolver struct {
		source Source
	}

	// resolution holds the mutable state of a single Resolve invocation.
	resolution struct {
		source Source
		// info caches the metadata for every AUR package name looked up.
		info map[string]PackageInfo
		// providesToBase maps a provided name to the package base that
		// provides it, built incrementally as packages are discovered.
		providesToBase map[string]string
		// visitedBase records package bases whose dependencies have been
		// walked, keyed by base name, to de-duplicate work and detect cycles.
		visitedBase map[string]bool
		// onStack tracks the package bases currently on the recursion stack,
		// used to detect circular dependencies.
		onStack map[string]bool
		// order is the package bases in the order they were completed (a
		// dependency-first ordering suitable for building upstream-first).
		order []string
		// edges records dependency relationships between package bases.
		edges []Edge
		// edgeSeen de-duplicates edges.
		edgeSeen map[Edge]bool
		// skipped collects dependency names not found in the AUR.
		skipped map[string]bool
	}
)

// New constructs a Resolver backed by the given Source.
func New(source Source) *Resolver {
	return &Resolver{source: source}
}

// Resolve resolves the given initial package names and all of their AUR
// dependencies, returning the complete set of package bases, the dependency
// edges between them, and the official-repository dependencies that were
// skipped. It returns ErrCircularDependency if a dependency cycle is found.
//
// The result is deterministic: package bases are ordered dependency-first
// (every base appears before any base that depends on it), with ties broken by
// the order the initial names are supplied; edges and skipped names are sorted.
func (r *Resolver) Resolve(ctx context.Context, names ...string) (*Result, error) {
	state := &resolution{
		source:         r.source,
		info:           make(map[string]PackageInfo),
		providesToBase: make(map[string]string),
		visitedBase:    make(map[string]bool),
		onStack:        make(map[string]bool),
		edgeSeen:       make(map[Edge]bool),
		skipped:        make(map[string]bool),
	}

	roots := dedupeStrings(names)

	// Prime the cache with the initial names so their bases are known before
	// the walk begins.
	err := state.load(ctx, roots...)
	if err != nil {
		return nil, err
	}

	for _, name := range roots {
		pkg, ok := state.info[name]
		if !ok {
			// A root that is not in the AUR is recorded as skipped; there is
			// nothing to build for it.
			state.skipped[name] = true

			continue
		}

		err = state.walk(ctx, pkg.PackageBase)
		if err != nil {
			return nil, err
		}
	}

	return state.result(), nil
}

// load fetches metadata for the given names that are not already cached,
// updating the info cache and the provides index.
func (s *resolution) load(ctx context.Context, names ...string) error {
	missing := make([]string, 0, len(names))

	for _, name := range names {
		_, cached := s.info[name]
		if !cached {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	found, err := s.source.Lookup(ctx, missing...)
	if err != nil {
		return fmt.Errorf("looking up packages: %w", err)
	}

	for name, pkg := range found {
		s.info[name] = pkg

		for _, provided := range pkg.Provides {
			// Prefer the first package base that provides a name for
			// determinism; do not overwrite an existing mapping.
			_, exists := s.providesToBase[provided]
			if !exists {
				s.providesToBase[provided] = pkg.PackageBase
			}
		}
	}

	return nil
}

// walk performs a depth-first traversal of a package base's dependencies,
// recording edges and detecting cycles. Completed bases are appended to order
// in dependency-first sequence.
func (s *resolution) walk(ctx context.Context, base string) error {
	if s.visitedBase[base] {
		return nil
	}

	if s.onStack[base] {
		return fmt.Errorf("%w: involving package base %q", ErrCircularDependency, base)
	}

	s.onStack[base] = true

	deps, err := s.dependencyBases(ctx, base)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		s.addEdge(base, dep)

		err = s.walk(ctx, dep)
		if err != nil {
			return err
		}
	}

	s.onStack[base] = false
	s.visitedBase[base] = true
	s.order = append(s.order, base)

	return nil
}

// dependencyBases returns the sorted, de-duplicated set of AUR package bases
// that the given package base depends on. Dependencies not present in the AUR
// are recorded as skipped.
func (s *resolution) dependencyBases(ctx context.Context, base string) ([]string, error) {
	depNames := s.collectDependencyNames(base)

	err := s.load(ctx, depNames...)
	if err != nil {
		return nil, err
	}

	bases := make(map[string]bool)

	for _, dep := range depNames {
		depBase, ok := s.resolveToBase(dep)
		if !ok {
			s.skipped[dep] = true

			continue
		}

		// A package base never depends on itself for ordering purposes.
		if depBase != base {
			bases[depBase] = true
		}
	}

	return sortedKeys(bases), nil
}

// collectDependencyNames returns the de-duplicated dependency names declared by
// every cached package belonging to the given package base.
func (s *resolution) collectDependencyNames(base string) []string {
	seen := make(map[string]bool)
	names := make([]string, 0)

	for _, pkg := range s.info {
		if pkg.PackageBase != base {
			continue
		}

		for _, dep := range pkg.Dependencies {
			if !seen[dep] {
				seen[dep] = true
				names = append(names, dep)
			}
		}
	}

	slices.Sort(names)

	return names
}

// resolveToBase maps a dependency name to the AUR package base that satisfies
// it, either directly (the name is an AUR package) or via a provides alias. It
// reports ok=false when no AUR package satisfies the dependency.
func (s *resolution) resolveToBase(dep string) (string, bool) {
	pkg, ok := s.info[dep]
	if ok {
		return pkg.PackageBase, true
	}

	base, ok := s.providesToBase[dep]
	if ok {
		return base, true
	}

	return "", false
}

// addEdge records a dependency edge between two package bases, de-duplicating.
func (s *resolution) addEdge(from, to string) {
	edge := Edge{From: from, To: to}
	if s.edgeSeen[edge] {
		return
	}

	s.edgeSeen[edge] = true
	s.edges = append(s.edges, edge)
}

// result assembles the final, deterministic Result from the resolution state.
func (s *resolution) result() *Result {
	edges := slices.Clone(s.edges)
	slices.SortFunc(edges, func(a, b Edge) int {
		if a.From != b.From {
			return compareStrings(a.From, b.From)
		}

		return compareStrings(a.To, b.To)
	})

	return &Result{
		PackageBases: slices.Clone(s.order),
		Edges:        edges,
		Skipped:      sortedKeys(s.skipped),
	}
}
