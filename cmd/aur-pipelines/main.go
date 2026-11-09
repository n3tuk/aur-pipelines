// Command aur-pipelines is a CLI tool for building Concourse CI pipelines for
// selected Arch Linux AUR packages, including recursive resolution of AUR
// dependencies.
package main

import (
	"fmt"
	"os"

	"github.com/n3tuk/aur-pipelines/internal/cli"
)

// Build-time version information, populated via -ldflags by GoReleaser. These
// values must be package-level variables so the linker can overwrite them at
// build time; see .goreleaser.yaml for the ldflags configuration.
//
//nolint:gochecknoglobals // required as -ldflags injection targets for GoReleaser
var (
	// Branch is the Git branch the binary was built from.
	Branch = "unknown"
	// Commit is the short Git commit hash the binary was built from.
	Commit = "unknown"
	// Version is the semantic version of the binary.
	Version = "dev"
	// BuildDate is the RFC3339 timestamp at which the binary was built.
	BuildDate = "unknown"
	// Architecture is the target architecture the binary was built for.
	Architecture = "unknown"
)

func main() {
	root := newRootCommand(cli.BuildInfo{
		Branch:       Branch,
		Commit:       Commit,
		Version:      Version,
		BuildDate:    BuildDate,
		Architecture: Architecture,
	})

	err := root.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
