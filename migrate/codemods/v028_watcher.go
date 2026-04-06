package codemods

import (
	"strings"
)

// RemoveLegacyWatcherCodemod removes usage of the deprecated resource.Watcher option
// that was removed in v0.30. It replaces the deprecated symbol with resource.InformerWatcher
// where an unambiguous substitution is possible.
type RemoveLegacyWatcherCodemod struct{}

func (c *RemoveLegacyWatcherCodemod) ID() string          { return "remove-legacy-watcher" }
func (c *RemoveLegacyWatcherCodemod) AppliesFrom() string { return "v0.28.0" }
func (c *RemoveLegacyWatcherCodemod) AppliesTo() string   { return "v0.30.0" }
func (c *RemoveLegacyWatcherCodemod) Description() string {
	return "Replaces deprecated resource.Watcher with resource.InformerWatcher (removed in v0.30)"
}

func (c *RemoveLegacyWatcherCodemod) Apply(dir string, dryRun bool) ([]FileChange, error) {
	return rewriteGoFiles(dir, dryRun, func(path, content string) (string, string, bool) {
		// Only rewrite the specific deprecated symbol; avoid touching unrelated Watcher types.
		if !strings.Contains(content, "resource.Watcher") {
			return "", "", false
		}
		updated := strings.ReplaceAll(content, "resource.Watcher", "resource.InformerWatcher")
		return updated, "replace deprecated resource.Watcher with resource.InformerWatcher", true
	})
}
