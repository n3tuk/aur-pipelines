package aur

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
)

type (
	// Fetcher retrieves and parses the .SRCINFO for an AUR package base.
	// Implementations are expected to be safe for concurrent use.
	Fetcher interface {
		// Fetch retrieves the parsed .SRCINFO for the given package base.
		Fetch(ctx context.Context, packageBase string) (*SrcInfo, error)
	}

	// GitFetcher is a Fetcher that clones an AUR package's git repository into
	// memory and parses its .SRCINFO. The zero value clones from the
	// production AUR git host; set BaseURL to target a different host (for
	// example in tests).
	GitFetcher struct {
		// BaseURL is the base git URL. When empty, the production AUR host is
		// used.
		BaseURL string
	}
)

const (
	// aurGitBaseURL is the base URL under which AUR package git repositories
	// are hosted; a package base "foo" lives at "<base>/foo.git".
	aurGitBaseURL = "https://aur.archlinux.org"
	// srcInfoFile is the name of the metadata file committed at the root of
	// every AUR package repository.
	srcInfoFile = ".SRCINFO"
	// cloneDepth limits the clone to the most recent commit, since only the
	// current .SRCINFO is required.
	cloneDepth = 1
)

var (
	// ErrSrcInfoNotFound is returned when a cloned repository does not contain
	// a .SRCINFO file at its root.
	ErrSrcInfoNotFound = errors.New(".SRCINFO not found in repository")

	// compile-time assertion that GitFetcher satisfies Fetcher.
	_ Fetcher = GitFetcher{}
)

// Fetch clones the repository for the given package base into an in-memory
// filesystem, reads its .SRCINFO, and returns the parsed result.
func (g GitFetcher) Fetch(ctx context.Context, packageBase string) (*SrcInfo, error) {
	base := g.BaseURL
	if base == "" {
		base = aurGitBaseURL
	}

	url := fmt.Sprintf("%s/%s.git", base, packageBase)

	filesystem := memfs.New()

	_, err := git.CloneContext(ctx, memory.NewStorage(), filesystem, &git.CloneOptions{
		URL:   url,
		Depth: cloneDepth,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning %q: %w", url, err)
	}

	file, err := filesystem.Open(srcInfoFile)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSrcInfoNotFound, packageBase)
	}
	defer file.Close()

	srcInfo, err := ParseSrcInfo(file)
	if err != nil {
		return nil, fmt.Errorf("parsing .SRCINFO for %q: %w", packageBase, err)
	}

	return srcInfo, nil
}
