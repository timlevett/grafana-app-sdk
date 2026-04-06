package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/grafana/grafana-app-sdk/migrate/compat"
)

var compatCmd = &cobra.Command{
	Use:   "compat",
	Short: "Check SDK/Grafana version compatibility",
	Long: `grafana-app-sdk compat reads your go.mod and checks whether the pinned SDK version
is compatible with the target Grafana version.

It exits non-zero when a hard-incompatible version pair is detected, making it suitable
for use as a CI gate before deploying.

Examples:
  grafana-app-sdk compat --grafana-version 11.0
  grafana-app-sdk compat --grafana-version 10.4 --dir /path/to/project`,
	SilenceUsage: true,
	RunE:         runCompat,
}

func init() {
	compatCmd.Flags().String("grafana-version", "", "Target Grafana version (e.g. 10.4). Reads from grafana-version file if not set.")
	compatCmd.Flags().String("dir", ".", "Project directory containing go.mod.")
}

func setupCompatCmd() {
	rootCmd.AddCommand(compatCmd)
}

func runCompat(cmd *cobra.Command, _ []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving dir: %w", err)
	}

	grafanaVersion, _ := cmd.Flags().GetString("grafana-version")

	result, err := compat.CheckFromGoMod(dir, grafanaVersion)
	if err != nil {
		return err
	}

	if result.Unknown {
		fmt.Fprintf(os.Stdout, "UNKNOWN  SDK %s + Grafana %s — not in compatibility matrix (assuming compatible)\n",
			result.SDKVersion, result.GrafanaVersion)
		return nil
	}

	if result.Compatible {
		fmt.Fprintf(os.Stdout, "OK       SDK %s is compatible with Grafana %s\n",
			result.SDKVersion, result.GrafanaVersion)
		return nil
	}

	msg := fmt.Sprintf("INCOMPATIBLE  SDK %s + Grafana %s", result.SDKVersion, result.GrafanaVersion)
	if result.Note != "" {
		msg += ": " + result.Note
	}
	fmt.Fprintln(os.Stderr, msg)
	return fmt.Errorf("incompatible SDK/Grafana version pair")
}
