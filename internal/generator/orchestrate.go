package generator

import (
	"context"
	"fmt"

	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

// packageNames returns the configured package names in order.
func packageNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Packages))
	for _, pkg := range cfg.Packages {
		names = append(names, pkg.Name)
	}

	return names
}

// Build resolves the configured packages and their AUR dependencies using the
// given source, then generates the complete set of pipelines: one per resolved
// package base, followed by the daily repository-cleanup pipeline. The result
// is deterministic for a given configuration and source.
//
// The resolver Source is injected so callers can supply the production RPC
// source or a fake in tests.
func Build(ctx context.Context, cfg *config.Config, source resolver.Source) ([]Pipeline, *resolver.Result, error) {
	result, err := resolver.New(source).Resolve(ctx, packageNames(cfg)...)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving packages: %w", err)
	}

	generator := New(cfg)

	pipelines := generator.Generate(result)
	pipelines = append(pipelines, generator.CleanupPipeline())

	return pipelines, result, nil
}
