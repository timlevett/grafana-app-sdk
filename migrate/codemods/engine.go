package codemods

import (
	"fmt"
	"sort"

	"golang.org/x/mod/semver"
)

// RunResult contains the aggregate output of running a set of codemods.
type RunResult struct {
	// AppliedCodemods lists the IDs of codemods that were executed.
	AppliedCodemods []string
	// SkippedCodemods lists codemods that were registered but did not apply to the version range.
	SkippedCodemods []string
	// Changes is the flat list of all file changes across all applied codemods.
	Changes []FileChange
	// Errors maps codemod ID to the error it produced (if any).
	Errors map[string]error
}

// Engine resolves and applies codemods for a given version range.
type Engine struct {
	registry []Codemod
}

// NewEngine returns an engine pre-loaded with all built-in codemods.
func NewEngine() *Engine {
	e := &Engine{}
	e.register(
		&KindRegistryImportCodemod{},
		&RenameResourceClientCodemod{},
		&RemoveLegacyWatcherCodemod{},
	)
	return e
}

// register adds codemods to the engine's registry.
func (e *Engine) register(cms ...Codemod) {
	e.registry = append(e.registry, cms...)
}

// List returns all registered codemods, sorted by their AppliesFrom version.
func (e *Engine) List() []Codemod {
	out := make([]Codemod, len(e.registry))
	copy(out, e.registry)
	sort.Slice(out, func(i, j int) bool {
		return semver.Compare(canonicalize(out[i].AppliesFrom()), canonicalize(out[j].AppliesFrom())) < 0
	})
	return out
}

// Run resolves which codemods apply to the [from, to) range and executes them in order.
// If dryRun is true, no files are written but changes are still computed and returned.
func (e *Engine) Run(dir, from, to string, dryRun bool) (*RunResult, error) {
	from = canonicalize(from)
	to = canonicalize(to)

	if !semver.IsValid(from) {
		return nil, fmt.Errorf("invalid from version %q", from)
	}
	if !semver.IsValid(to) {
		return nil, fmt.Errorf("invalid to version %q", to)
	}
	if semver.Compare(from, to) >= 0 {
		return nil, fmt.Errorf("from version %s must be less than to version %s", from, to)
	}

	result := &RunResult{
		Errors: make(map[string]error),
	}

	// Sort codemods by AppliesFrom so they run in order.
	ordered := e.List()

	for _, cm := range ordered {
		applies, err := AppliesTo(cm, from, to)
		if err != nil {
			return nil, fmt.Errorf("checking codemod %s: %w", cm.ID(), err)
		}
		if !applies {
			result.SkippedCodemods = append(result.SkippedCodemods, cm.ID())
			continue
		}

		changes, err := cm.Apply(dir, dryRun)
		if err != nil {
			result.Errors[cm.ID()] = err
			// Continue running remaining codemods even if one fails.
			continue
		}
		result.AppliedCodemods = append(result.AppliedCodemods, cm.ID())
		result.Changes = append(result.Changes, changes...)
	}

	return result, nil
}
