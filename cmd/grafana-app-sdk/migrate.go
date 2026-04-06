package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/grafana-app-sdk/migrate"
	"github.com/grafana/grafana-app-sdk/migrate/codemods"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate project source code between SDK versions",
	Long: `grafana-app-sdk migrate automates source code changes required when upgrading
the SDK to a new version. It runs versioned codemods against your project directory
and produces a migration report.

Examples:
  grafana-app-sdk migrate                        # auto-detect versions from go.mod
  grafana-app-sdk migrate --from v0.27.0 --to v0.29.0
  grafana-app-sdk migrate --dry-run              # preview changes without writing files
  grafana-app-sdk migrate --list-codemods        # list all available codemods`,
	SilenceUsage: true,
	RunE:         runMigrate,
}

func init() {
	migrateCmd.Flags().String("from", "", "Source SDK version (e.g. v0.27.0). Defaults to current version in go.mod.")
	migrateCmd.Flags().String("to", "", "Target SDK version (e.g. v0.29.0). Defaults to latest known version.")
	migrateCmd.Flags().Bool("dry-run", false, "Preview changes without writing any files.")
	migrateCmd.Flags().Bool("list-codemods", false, "Print all available codemods and exit.")
	migrateCmd.Flags().String("dir", ".", "Project directory to migrate.")
}

func setupMigrateCmd() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	listOnly, _ := cmd.Flags().GetBool("list-codemods")
	engine := codemods.NewEngine()

	if listOnly {
		fmt.Println("Available codemods:")
		for _, cm := range engine.List() {
			fmt.Printf("  %-40s %s → %s\n    %s\n\n",
				cm.ID(), cm.AppliesFrom(), cm.AppliesTo(), cm.Description())
		}
		return nil
	}

	dir, _ := cmd.Flags().GetString("dir")
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving dir: %w", err)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	fromVer, _ := cmd.Flags().GetString("from")
	toVer, _ := cmd.Flags().GetString("to")

	if fromVer == "" {
		detected, err := detectSDKVersionFromGoMod(dir)
		if err != nil {
			return fmt.Errorf("--from not set and could not detect SDK version from go.mod: %w", err)
		}
		fromVer = detected
		fmt.Fprintf(os.Stderr, "Detected current SDK version from go.mod: %s\n", fromVer)
	}
	if toVer == "" {
		toVer = latestKnownSDKVersion()
		fmt.Fprintf(os.Stderr, "Using latest known SDK version as target: %s\n", toVer)
	}

	result, err := engine.Run(dir, fromVer, toVer, dryRun)
	if err != nil {
		return err
	}

	report := &migrate.Report{
		From:            fromVer,
		To:              toVer,
		DryRun:          dryRun,
		AppliedCodemods: result.AppliedCodemods,
		SkippedCodemods: result.SkippedCodemods,
		Changes:         result.Changes,
		Errors:          result.Errors,
	}
	report.Print(os.Stdout)

	if report.HasErrors() {
		return fmt.Errorf("migration completed with errors; see report above")
	}
	return nil
}

// detectSDKVersionFromGoMod reads the go.mod in dir and returns the current
// grafana-app-sdk version pinned there.
func detectSDKVersionFromGoMod(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "github.com/grafana/grafana-app-sdk ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("grafana-app-sdk not found in go.mod")
}

// latestKnownSDKVersion returns the version of the currently running CLI binary,
// falling back to a hard-coded latest if build info is unavailable.
func latestKnownSDKVersion() string {
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "v0.30.0"
}
