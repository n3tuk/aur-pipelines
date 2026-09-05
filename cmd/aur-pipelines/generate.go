package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/n3tuk/aur-pipelines/internal/config"
)

// newGenerateCommand constructs the `generate` subcommand. For now it loads
// and validates the structure of the provided configuration file and prints a
// summary of what was parsed; pipeline generation is added in later tasks.
func newGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate <config.yaml>",
		Short: "Generate Concourse CI pipelines from a configuration file",
		Long: "Load an aur-pipelines configuration file and generate the Concourse CI\n" +
			"pipelines for the listed AUR packages and their AUR dependencies.\n\n" +
			"In the current build this command loads the configuration and prints a\n" +
			"summary of what was parsed; pipeline generation is added in later tasks.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(args[0])
			if err != nil {
				return fmt.Errorf("loading configuration: %w", err)
			}

			return cfg.Summary(cmd.OutOrStdout())
		},
	}
}
