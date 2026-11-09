package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/n3tuk/aur-pipelines/internal/aur"
	"github.com/n3tuk/aur-pipelines/internal/config"
	"github.com/n3tuk/aur-pipelines/internal/generator"
	"github.com/n3tuk/aur-pipelines/internal/resolver"
)

// defaultOutputDir is the directory generated pipelines are written to when
// --output is not supplied.
const defaultOutputDir = "pipelines"

// newGenerateCommand constructs the `generate` subcommand, which loads a
// configuration file, resolves the listed AUR packages and their AUR
// dependencies, generates the Concourse pipelines (one per package base plus
// the daily repository-cleanup pipeline), and writes them to the output
// directory (or prints them, for a dry run).
func newGenerateCommand() *cobra.Command {
	var (
		output string
		dryRun bool
	)

	command := &cobra.Command{
		Use:   "generate <config.yaml>",
		Short: "Generate Concourse CI pipelines from a configuration file",
		Long: "Load an aur-pipelines configuration file, resolve the listed AUR packages\n" +
			"and their AUR dependencies, and generate the Concourse CI pipelines: one\n" +
			"per package base, plus a daily repository-cleanup pipeline.\n\n" +
			"By default the pipelines are written to the output directory, one YAML file\n" +
			"per pipeline. Use --dry-run to print them to standard output instead.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd, args[0], output, dryRun)
		},
	}

	command.Flags().StringVarP(&output, "output", "o", defaultOutputDir,
		"directory to write generated pipelines to")
	command.Flags().BoolVar(&dryRun, "dry-run", false,
		"print generated pipelines to standard output instead of writing files")

	return command
}

// runGenerate performs the load, resolve, generate, and write steps for the
// generate command.
func runGenerate(cmd *cobra.Command, configPath, output string, dryRun bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	source := resolver.NewRPCSource(aur.NewClient())

	pipelines, result, err := generator.Build(cmd.Context(), cfg, source)
	if err != nil {
		return fmt.Errorf("building pipelines: %w", err)
	}

	if dryRun {
		err = generator.WriteStream(cmd.OutOrStdout(), pipelines)
		if err != nil {
			return fmt.Errorf("writing pipelines: %w", err)
		}

		return nil
	}

	written, err := generator.WriteDir(output, pipelines)
	if err != nil {
		return fmt.Errorf("writing pipelines: %w", err)
	}

	report(cmd, output, written, result)

	return nil
}

// report prints a short summary of what was generated to standard output.
func report(cmd *cobra.Command, output string, written []string, result *resolver.Result) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Generated %d pipeline(s) in %s:\n", len(written), output)

	for _, name := range written {
		fmt.Fprintf(out, "  - %s\n", name)
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintf(out, "Skipped %d official-repository dependency(ies): %v\n",
			len(result.Skipped), result.Skipped)
	}
}
