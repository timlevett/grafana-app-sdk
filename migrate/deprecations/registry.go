// Package deprecations provides a registry of deprecated symbols and a scanner
// that can find usages in a project directory. It powers `grafana-app-sdk lint --deprecations`.
package deprecations

// Entry describes a single deprecated symbol.
type Entry struct {
	// Symbol is the deprecated identifier as it appears in source (e.g. "resource.Watcher").
	Symbol string
	// DeprecatedSince is the SDK version that deprecated this symbol.
	DeprecatedSince string
	// RemovedIn is the SDK version that removes this symbol (may be empty if not yet removed).
	RemovedIn string
	// Replacement is the suggested replacement (may be empty).
	Replacement string
	// MigrationGuide is a URL or relative path to the migration documentation.
	MigrationGuide string
}

// builtinEntries is the canonical set of deprecation records for the grafana-app-sdk.
var builtinEntries = []Entry{
	{
		Symbol:          "resource.Watcher",
		DeprecatedSince: "v0.28.0",
		RemovedIn:       "v0.30.0",
		Replacement:     "resource.InformerWatcher",
		MigrationGuide:  "https://github.com/grafana/grafana-app-sdk/blob/main/docs/migrations/v0.28.md",
	},
	{
		Symbol:          "ResourceClient",
		DeprecatedSince: "v0.27.0",
		RemovedIn:       "v0.29.0",
		Replacement:     "TypedClient",
		MigrationGuide:  "https://github.com/grafana/grafana-app-sdk/blob/main/docs/migrations/v0.27.md",
	},
	{
		Symbol:          `"github.com/grafana/grafana-app-sdk/resource/registry"`,
		DeprecatedSince: "v0.26.0",
		RemovedIn:       "v0.28.0",
		Replacement:     `"github.com/grafana/grafana-app-sdk/kindsys/registry"`,
		MigrationGuide:  "https://github.com/grafana/grafana-app-sdk/blob/main/docs/migrations/v0.26.md",
	},
}

// Registry holds the set of deprecated symbols to scan for.
type Registry struct {
	entries []Entry
}

// Default returns a Registry pre-loaded with all built-in deprecation entries.
func Default() *Registry {
	r := &Registry{}
	r.entries = append(r.entries, builtinEntries...)
	return r
}

// Entries returns all entries in the registry.
func (r *Registry) Entries() []Entry {
	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	return out
}

// Add appends custom entries to the registry.
func (r *Registry) Add(entries ...Entry) {
	r.entries = append(r.entries, entries...)
}
