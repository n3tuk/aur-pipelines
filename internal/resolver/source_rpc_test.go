package resolver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/n3tuk/aur-pipelines/internal/aur"
	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

func TestRPCSourceLookup(t *testing.T) {
	t.Parallel()

	body := `{"version":5,"type":"multiinfo","resultcount":1,"results":[{
		"Name":"kalc-bin",
		"PackageBase":"kalc-bin",
		"Version":"1.5.1-2",
		"Depends":["glibc","libgcc>=13.0"],
		"MakeDepends":["cargo"],
		"CheckDepends":["gtest=1.14"],
		"OptDepends":["docs: offline documentation"],
		"Provides":["kalc=1.5.1"]
	}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_, err := fmt.Fprint(w, body)
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	source := resolver.NewRPCSource(aur.NewClient(aur.WithBaseURL(server.URL)))

	found, err := source.Lookup(t.Context(), pkgKalcBin)
	if err != nil {
		t.Fatalf("Lookup() returned unexpected error: %v", err)
	}

	pkg, ok := found[pkgKalcBin]
	if !ok {
		t.Fatal("Lookup() missing kalc-bin")
	}

	if pkg.PackageBase != pkgKalcBin {
		t.Errorf("PackageBase = %q, want %q", pkg.PackageBase, pkgKalcBin)
	}

	// depends + makedepends + checkdepends combined, constraints stripped,
	// optdepends excluded.
	wantDeps := []string{pkgGlibc, "libgcc", "cargo", "gtest"}
	if !slices.Equal(pkg.Dependencies, wantDeps) {
		t.Errorf("Dependencies = %v, want %v (optdepends excluded, constraints stripped)", pkg.Dependencies, wantDeps)
	}

	// provides constraint stripped.
	if !slices.Equal(pkg.Provides, []string{"kalc"}) {
		t.Errorf("Provides = %v, want [kalc]", pkg.Provides)
	}
}

func TestRPCSourceOmitsUnknown(t *testing.T) {
	t.Parallel()

	// Server returns an empty result set: nothing is in the AUR.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_, err := fmt.Fprint(w, `{"version":5,"type":"multiinfo","resultcount":0,"results":[]}`)
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	source := resolver.NewRPCSource(aur.NewClient(aur.WithBaseURL(server.URL)))

	found, err := source.Lookup(t.Context(), "glibc")
	if err != nil {
		t.Fatalf("Lookup() returned unexpected error: %v", err)
	}

	if len(found) != 0 {
		t.Errorf("Lookup() = %v, want empty (glibc is not an AUR package)", found)
	}
}
