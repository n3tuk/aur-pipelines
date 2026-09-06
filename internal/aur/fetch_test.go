package aur_test

import (
	"context"
	"errors"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/aur"
)

// fakeFetcher is a test double for aur.Fetcher backed by an in-memory map of
// package base to SrcInfo. It demonstrates how the resolver (a later task) can
// substitute a fake AUR source in unit tests.
type fakeFetcher struct {
	sources map[string]*aur.SrcInfo
}

// compile-time assertion that fakeFetcher satisfies the Fetcher interface.
var _ aur.Fetcher = fakeFetcher{}

func (f fakeFetcher) Fetch(_ context.Context, packageBase string) (*aur.SrcInfo, error) {
	srcInfo, ok := f.sources[packageBase]
	if !ok {
		return nil, aur.ErrSrcInfoNotFound
	}

	return srcInfo, nil
}

func TestFakeFetcher(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		sources: map[string]*aur.SrcInfo{
			pkgKalcBin: {
				PackageBase: pkgKalcBin,
				Version:     "1.5.1-2",
				Packages:    []string{pkgKalcBin},
				Depends:     []string{pkgGlibc, pkgLibgcc},
				Provides:    []string{provKalc},
			},
		},
	}

	got, err := fetcher.Fetch(t.Context(), pkgKalcBin)
	if err != nil {
		t.Fatalf("Fetch() returned unexpected error: %v", err)
	}

	if got.PackageBase != pkgKalcBin {
		t.Errorf("PackageBase = %q, want %q", got.PackageBase, pkgKalcBin)
	}

	_, err = fetcher.Fetch(t.Context(), "nonexistent")
	if !errors.Is(err, aur.ErrSrcInfoNotFound) {
		t.Errorf("Fetch(nonexistent) error = %v, want ErrSrcInfoNotFound", err)
	}
}
