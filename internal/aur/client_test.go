package aur_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/n3tuk/aur-pipelines/internal/aur"
)

const (
	pkgKalcBin  = "kalc-bin"
	pkgNtfyshB  = "ntfysh-bin"
	pkgKeybase  = "keybase-bin"
	pkgGlibc    = "glibc"
	pkgLibgcc   = "libgcc"
	provKalc    = "kalc"
	contentType = "application/json"
)

// multiInfoBody builds a multiinfo response body for the given packages.
func multiInfoBody(results string) string {
	return `{"version":5,"type":"multiinfo","resultcount":1,"results":[` + results + `]}`
}

func TestInfoDecodesResults(t *testing.T) {
	t.Parallel()

	body := multiInfoBody(`{
		"Name":"kalc-bin",
		"PackageBase":"kalc-bin",
		"Version":"1.2.3-1",
		"Depends":["gcc-libs","glibc"],
		"MakeDepends":["cargo"],
		"Provides":["kalc"]
	}`)

	server := newServer(t, func(_ *http.Request) (int, string) {
		return http.StatusOK, body
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	packages, err := client.Info(t.Context(), pkgKalcBin)
	if err != nil {
		t.Fatalf("Info() returned unexpected error: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("len(packages) = %d, want 1", len(packages))
	}

	pkg := packages[0]
	if pkg.Name != pkgKalcBin {
		t.Errorf("Name = %q, want %q", pkg.Name, pkgKalcBin)
	}

	if pkg.PackageBase != pkgKalcBin {
		t.Errorf("PackageBase = %q, want %q", pkg.PackageBase, pkgKalcBin)
	}

	if pkg.Version != "1.2.3-1" {
		t.Errorf("Version = %q, want %q", pkg.Version, "1.2.3-1")
	}

	if !slices.Equal(pkg.Depends, []string{"gcc-libs", pkgGlibc}) {
		t.Errorf("Depends = %v, want [gcc-libs glibc]", pkg.Depends)
	}

	if !slices.Equal(pkg.MakeDepends, []string{"cargo"}) {
		t.Errorf("MakeDepends = %v, want [cargo]", pkg.MakeDepends)
	}

	if !slices.Equal(pkg.Provides, []string{provKalc}) {
		t.Errorf("Provides = %v, want [kalc]", pkg.Provides)
	}
}

func TestInfoEmptyNames(t *testing.T) {
	t.Parallel()

	called := false

	server := newServer(t, func(_ *http.Request) (int, string) {
		called = true

		return http.StatusOK, multiInfoBody("")
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	packages, err := client.Info(t.Context())
	if err != nil {
		t.Fatalf("Info() returned unexpected error: %v", err)
	}

	if len(packages) != 0 {
		t.Errorf("len(packages) = %d, want 0", len(packages))
	}

	if called {
		t.Error("Info() with no names should not make a request")
	}
}

func TestInfoDeduplicates(t *testing.T) {
	t.Parallel()

	var got []string

	server := newServer(t, func(r *http.Request) (int, string) {
		got = r.URL.Query()["arg[]"]

		return http.StatusOK, multiInfoBody("")
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	_, err := client.Info(t.Context(), pkgKalcBin, pkgKalcBin, "", pkgNtfyshB)
	if err != nil {
		t.Fatalf("Info() returned unexpected error: %v", err)
	}

	want := []string{pkgKalcBin, pkgNtfyshB}
	if !slices.Equal(got, want) {
		t.Errorf("arg[] = %v, want %v (deduped, empty removed)", got, want)
	}
}

func TestInfoBatches(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		requests [][]string
	)

	server := newServer(t, func(r *http.Request) (int, string) {
		args := r.URL.Query()["arg[]"]

		mu.Lock()

		requests = append(requests, args)
		mu.Unlock()

		return http.StatusOK, multiInfoBody("")
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL), aur.WithBatchSize(2))

	_, err := client.Info(t.Context(), pkgKalcBin, pkgNtfyshB, pkgKeybase)
	if err != nil {
		t.Fatalf("Info() returned unexpected error: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("made %d requests, want 2 (batch size 2, 3 names)", len(requests))
	}

	var all []string
	for _, req := range requests {
		all = append(all, req...)
	}

	slices.Sort(all)

	want := []string{pkgKalcBin, pkgKeybase, pkgNtfyshB}
	if !slices.Equal(all, want) {
		t.Errorf("names across batches = %v, want %v", all, want)
	}
}

func TestInfoErrorResponse(t *testing.T) {
	t.Parallel()

	server := newServer(t, func(_ *http.Request) (int, string) {
		return http.StatusOK, `{"version":5,"type":"error","resultcount":0,"results":[],"error":"something failed"}`
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	_, err := client.Info(t.Context(), pkgKalcBin)
	if err == nil {
		t.Fatal("Info() = nil error, want an RPC error")
	}

	if !errors.Is(err, aur.ErrRPC) {
		t.Errorf("Info() error = %v, want it to wrap ErrRPC", err)
	}
}

func TestInfoNon200(t *testing.T) {
	t.Parallel()

	server := newServer(t, func(_ *http.Request) (int, string) {
		return http.StatusInternalServerError, "boom"
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	_, err := client.Info(t.Context(), pkgKalcBin)
	if err == nil {
		t.Fatal("Info() = nil error, want an error for non-200 status")
	}

	if !errors.Is(err, aur.ErrRPC) {
		t.Errorf("Info() error = %v, want it to wrap ErrRPC", err)
	}
}

func TestInfoMalformedJSON(t *testing.T) {
	t.Parallel()

	server := newServer(t, func(_ *http.Request) (int, string) {
		return http.StatusOK, `{not valid json`
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	_, err := client.Info(t.Context(), pkgKalcBin)
	if err == nil {
		t.Fatal("Info() = nil error, want a decode error")
	}
}

func TestInfoContextCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()
	defer close(release)

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Info(ctx, pkgKalcBin)
	if err == nil {
		t.Fatal("Info() = nil error, want a timeout/cancellation error")
	}
}

func TestMembers(t *testing.T) {
	t.Parallel()

	// Only kalc-bin and keybase-bin exist in the AUR; glibc does not.
	server := newServer(t, func(_ *http.Request) (int, string) {
		return http.StatusOK, `{"version":5,"type":"multiinfo","resultcount":2,"results":[` +
			`{"Name":"kalc-bin","PackageBase":"kalc-bin","Version":"1-1"},` +
			`{"Name":"keybase-bin","PackageBase":"keybase-bin","Version":"1-1"}` +
			`]}`
	})
	defer server.Close()

	client := aur.NewClient(aur.WithBaseURL(server.URL))

	members, err := client.Members(t.Context(), pkgKalcBin, pkgGlibc, pkgKeybase)
	if err != nil {
		t.Fatalf("Members() returned unexpected error: %v", err)
	}

	want := []string{pkgKalcBin, pkgKeybase}
	if !slices.Equal(members, want) {
		t.Errorf("Members() = %v, want %v (glibc is not an AUR package)", members, want)
	}
}

func TestByName(t *testing.T) {
	t.Parallel()

	packages := []aur.Package{
		{Name: pkgKalcBin, PackageBase: pkgKalcBin},
		{Name: pkgKeybase, PackageBase: pkgKeybase},
	}

	index := aur.ByName(packages)

	if len(index) != 2 {
		t.Fatalf("len(index) = %d, want 2", len(index))
	}

	pkg, ok := index[pkgKalcBin]
	if !ok {
		t.Fatalf("index missing %q", pkgKalcBin)
	}

	if pkg.PackageBase != pkgKalcBin {
		t.Errorf("index[%q].PackageBase = %q, want %q", pkgKalcBin, pkg.PackageBase, pkgKalcBin)
	}
}

// newServer starts a test HTTP server whose handler delegates to respond,
// which returns the status code and body to write. The server is registered
// for cleanup with the test.
func newServer(t *testing.T, respond func(r *http.Request) (int, string)) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := respond(r)

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)

		_, err := fmt.Fprint(w, body)
		if err != nil {
			t.Errorf("writing test response: %v", err)
		}
	}))
}
