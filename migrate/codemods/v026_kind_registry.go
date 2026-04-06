package codemods

import (
	"os"
	"path/filepath"
	"strings"
)

// KindRegistryImportCodemod rewrites the old kind registry import path introduced in v0.26.
// In v0.28 the package was moved from grafana-app-sdk/resource/registry to
// grafana-app-sdk/kindsys/registry.
type KindRegistryImportCodemod struct{}

func (c *KindRegistryImportCodemod) ID() string          { return "update-kind-registry-import" }
func (c *KindRegistryImportCodemod) AppliesFrom() string { return "v0.26.0" }
func (c *KindRegistryImportCodemod) AppliesTo() string   { return "v0.28.0" }
func (c *KindRegistryImportCodemod) Description() string {
	return `Rewrites kind registry import path from "grafana-app-sdk/resource/registry" ` +
		`to "grafana-app-sdk/kindsys/registry" (changed in v0.28)`
}

const (
	kindRegistryOld = `"github.com/grafana/grafana-app-sdk/resource/registry"`
	kindRegistryNew = `"github.com/grafana/grafana-app-sdk/kindsys/registry"`
)

func (c *KindRegistryImportCodemod) Apply(dir string, dryRun bool) ([]FileChange, error) {
	return rewriteGoFiles(dir, dryRun, func(path, content string) (string, string, bool) {
		if !strings.Contains(content, kindRegistryOld) {
			return "", "", false
		}
		updated := strings.ReplaceAll(content, kindRegistryOld, kindRegistryNew)
		return updated, "rewrite kind registry import path to kindsys/registry", true
	})
}

// rewriteGoFiles walks dir for .go files and applies fn to each. fn returns
// (newContent, description, changed). If changed is false, the file is skipped.
func rewriteGoFiles(dir string, dryRun bool, fn func(path, content string) (newContent, description string, changed bool)) ([]FileChange, error) {
	var changes []FileChange
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)

		newContent, desc, changed := fn(path, content)
		if !changed {
			return nil
		}

		rel, _ := filepath.Rel(dir, path)
		fc := FileChange{
			Path:        rel,
			Description: desc,
			OldContent:  content,
			NewContent:  newContent,
		}
		changes = append(changes, fc)

		if !dryRun {
			if err := os.WriteFile(path, []byte(newContent), d.Type()); err != nil {
				return err
			}
		}
		return nil
	})
	return changes, err
}
