package codemods

import (
	"strings"
)

// RenameResourceClientCodemod renames ResourceClient to TypedClient, which was
// introduced as the canonical name in v0.29. ResourceClient was deprecated in v0.27.
type RenameResourceClientCodemod struct{}

func (c *RenameResourceClientCodemod) ID() string          { return "rename-resource-client" }
func (c *RenameResourceClientCodemod) AppliesFrom() string { return "v0.27.0" }
func (c *RenameResourceClientCodemod) AppliesTo() string   { return "v0.29.0" }
func (c *RenameResourceClientCodemod) Description() string {
	return "Renames ResourceClient to TypedClient (deprecated in v0.27, removed in v0.29)"
}

func (c *RenameResourceClientCodemod) Apply(dir string, dryRun bool) ([]FileChange, error) {
	return rewriteGoFiles(dir, dryRun, func(path, content string) (string, string, bool) {
		if !strings.Contains(content, "ResourceClient") {
			return "", "", false
		}
		updated := strings.ReplaceAll(content, "ResourceClient", "TypedClient")
		return updated, "rename ResourceClient → TypedClient", true
	})
}
