package main

import (
	"github.com/spf13/cobra"

	"github.com/n3tuk/aur-pipelines/internal/cli"
)

// newVersionCommand constructs the `version` subcommand, which prints the
// build-time version information for the binary.
func newVersionCommand(info cli.BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the build version information",
		Long: "Print the build version information for aur-pipelines, including the branch,\n" +
			"commit, build date, and architecture.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return info.Report(cmd.OutOrStdout())
		},
	}
}
