package main

import (
	"github.com/spf13/cobra"

	"github.com/n3tuk/aur-pipelines/internal/cli"
)

// newRootCommand constructs the root Cobra command for aur-pipelines,
// registering all subcommands. The provided build information is threaded into
// any subcommand that needs it.
func newRootCommand(info cli.BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:   "aur-pipelines",
		Short: "Generate Concourse CI pipelines for Arch Linux AUR packages",
		Long: "aur-pipelines is a CLI tool for building Concourse CI pipelines for selected\n" +
			"Arch Linux AUR packages, including recursive resolution of AUR dependencies.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       info.Version,
	}

	root.AddCommand(newVersionCommand(info))

	return root
}
