// Package cli holds the internal, dependency-light logic backing the
// aur-pipelines command-line interface, such as build-version reporting and
// (in later tasks) configuration flag binding. The Cobra command tree itself
// is assembled in the main package under cmd/aur-pipelines, keeping this
// package free of the command framework so it remains easily testable.
package cli

import (
	"fmt"
	"io"
	"strings"
)

// BuildInfo carries the build-time version information injected into the
// binary at link time. It is populated in package main from -ldflags values
// and passed into the CLI so the version subcommand can report it.
type BuildInfo struct {
	// Branch is the Git branch the binary was built from.
	Branch string
	// Commit is the short Git commit hash the binary was built from.
	Commit string
	// Version is the semantic version of the binary.
	Version string
	// BuildDate is the RFC3339 timestamp at which the binary was built.
	BuildDate string
	// Architecture is the target architecture the binary was built for.
	Architecture string
}

// String renders the build information as a single-line, human-readable
// summary suitable for logging or a compact --version style output.
func (b BuildInfo) String() string {
	return fmt.Sprintf(
		"aur-pipelines %s (branch: %s, commit: %s, built: %s, arch: %s)",
		b.Version, b.Branch, b.Commit, b.BuildDate, b.Architecture,
	)
}

// Report renders the build information as an aligned, multi-line block and
// writes it to the provided writer. It returns any error encountered while
// writing.
func (b BuildInfo) Report(w io.Writer) error {
	var builder strings.Builder

	fields := []struct {
		label string
		value string
	}{
		{"Version", b.Version},
		{"Branch", b.Branch},
		{"Commit", b.Commit},
		{"Build Date", b.BuildDate},
		{"Architecture", b.Architecture},
	}

	for _, field := range fields {
		fmt.Fprintf(&builder, "%-13s %s\n", field.label+":", field.value)
	}

	_, err := io.WriteString(w, builder.String())
	if err != nil {
		return fmt.Errorf("writing version report: %w", err)
	}

	return nil
}
