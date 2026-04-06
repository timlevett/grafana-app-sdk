package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/grafana/grafana-app-sdk/migrate/deprecations"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint the project for common issues",
	Long: `grafana-app-sdk lint checks your project for common issues.

Use --deprecations to scan for deprecated SDK symbols and get actionable fix suggestions.

Examples:
  grafana-app-sdk lint --deprecations
  grafana-app-sdk lint --deprecations --format=json`,
	SilenceUsage: true,
	RunE:         runLint,
}

func init() {
	lintCmd.Flags().Bool("deprecations", false, "Scan for deprecated SDK symbols and print fix suggestions.")
	lintCmd.Flags().String("format", "text", "Output format: text or json.")
	lintCmd.Flags().String("dir", ".", "Project directory to lint.")
}

func setupLintCmd() {
	rootCmd.AddCommand(lintCmd)
}

func runLint(cmd *cobra.Command, _ []string) error {
	checkDeprecations, _ := cmd.Flags().GetBool("deprecations")
	if !checkDeprecations {
		cmd.Help() //nolint:errcheck
		return nil
	}

	dir, _ := cmd.Flags().GetString("dir")
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving dir: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")

	reg := deprecations.Default()
	scanner := deprecations.NewScanner(reg)
	warnings, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("scanning for deprecations: %w", err)
	}

	if len(warnings) == 0 {
		fmt.Fprintln(os.Stdout, "No deprecation warnings found.")
		return nil
	}

	switch format {
	case "json":
		if err := deprecations.PrintJSON(os.Stdout, warnings); err != nil {
			return err
		}
	default:
		deprecations.PrintText(os.Stdout, warnings)
	}

	// Exit non-zero so CI can use this as a gate.
	return fmt.Errorf("%d deprecation warning(s) found", len(warnings))
}
