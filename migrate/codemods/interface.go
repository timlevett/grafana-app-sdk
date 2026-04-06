// Package codemods provides the interface and engine for automated SDK migration codemods.
// A codemod is a versioned source code transformation that can be applied when upgrading
// from one SDK version to another.
package codemods

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// FileChange records a single file modification made by a codemod.
type FileChange struct {
	// Path is the file path relative to the project root.
	Path string
	// Description is a human-readable summary of the change.
	Description string
	// OldContent is the original file content (may be empty for new files).
	OldContent string
	// NewContent is the replacement content.
	NewContent string
	// Added indicates this is a newly created file.
	Added bool
	// Deleted indicates this file was removed.
	Deleted bool
}

// Codemod is the interface that all automated SDK migration transforms must implement.
// Each codemod handles a specific breaking change over a declared semver range.
type Codemod interface {
	// ID returns a unique, stable identifier for this codemod (e.g. "rename-resource-client").
	ID() string
	// Description returns a human-readable explanation of what the codemod does.
	Description() string
	// AppliesFrom returns the minimum SDK version this codemod applies to (inclusive).
	AppliesFrom() string
	// AppliesTo returns the maximum SDK version this codemod targets (exclusive).
	AppliesTo() string
	// Apply runs the codemod against the project directory. It returns the list of
	// file changes that were made (or would be made in dry-run mode).
	Apply(dir string, dryRun bool) ([]FileChange, error)
}

// AppliesTo returns true if a codemod should run given the provided from/to version range.
// Both from and to must be canonical semver strings (e.g. "v0.28.0").
func AppliesTo(c Codemod, from, to string) (bool, error) {
	if !semver.IsValid(from) {
		return false, fmt.Errorf("invalid from version %q", from)
	}
	if !semver.IsValid(to) {
		return false, fmt.Errorf("invalid to version %q", to)
	}
	cf := canonicalize(c.AppliesFrom())
	ct := canonicalize(c.AppliesTo())
	// Codemod is relevant if its range overlaps with [from, to).
	// Overlap condition: codemod.from < to AND codemod.to > from
	return semver.Compare(cf, to) < 0 && semver.Compare(ct, from) > 0, nil
}

// canonicalize ensures a version string has the "v" prefix required by golang.org/x/mod/semver.
func canonicalize(v string) string {
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
