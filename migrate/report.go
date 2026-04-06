// Package migrate provides the runner, report, and helpers for the grafana-app-sdk migrate command.
package migrate

import (
	"fmt"
	"io"
	"strings"

	"github.com/grafana/grafana-app-sdk/migrate/codemods"
)

// Report summarises the outcome of a migration run.
type Report struct {
	From            string
	To              string
	DryRun          bool
	AppliedCodemods []string
	SkippedCodemods []string
	Changes         []codemods.FileChange
	Errors          map[string]error
}

// Print writes a human-readable migration report to w.
func (r *Report) Print(w io.Writer) {
	mode := "applied"
	if r.DryRun {
		mode = "dry-run (no files written)"
	}
	fmt.Fprintf(w, "## Migration Report: %s → %s (%s)\n\n", r.From, r.To, mode)

	if len(r.Errors) > 0 {
		fmt.Fprintf(w, "### Errors (%d)\n", len(r.Errors))
		for id, err := range r.Errors {
			fmt.Fprintf(w, "  - %s: %v\n", id, err)
		}
		fmt.Fprintln(w)
	}

	if len(r.AppliedCodemods) > 0 {
		fmt.Fprintf(w, "### Applied codemods (%d)\n", len(r.AppliedCodemods))
		for _, id := range r.AppliedCodemods {
			fmt.Fprintf(w, "  - %s\n", id)
		}
		fmt.Fprintln(w)
	}

	if len(r.Changes) > 0 {
		fmt.Fprintf(w, "### Changed files (%d)\n", len(r.Changes))
		for _, fc := range r.Changes {
			action := "modified"
			if fc.Added {
				action = "added"
			} else if fc.Deleted {
				action = "deleted"
			}
			fmt.Fprintf(w, "  - [%s] %s — %s\n", action, fc.Path, fc.Description)
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "No file changes.")
	}

	if len(r.SkippedCodemods) > 0 {
		fmt.Fprintf(w, "### Skipped codemods (%d — outside version range)\n", len(r.SkippedCodemods))
		fmt.Fprintf(w, "  %s\n", strings.Join(r.SkippedCodemods, ", "))
	}
}

// HasErrors returns true if any codemod produced an error.
func (r *Report) HasErrors() bool {
	return len(r.Errors) > 0
}
