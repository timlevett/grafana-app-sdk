// Package scaffold generates a complete Grafana app-platform project from
// templates. The generated project uses the local embedded server (SQLite)
// and includes a Kind definition, Go entrypoint, React CRUD UI, and Makefile.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Config describes what to scaffold.
type Config struct {
	AppName    string // directory name, e.g. "my-bookmarks"
	KindName   string // PascalCase Kind, e.g. "Bookmark"
	ModulePath string // Go module path
	Port       int    // local server port
}

// Derived returns computed template values.
func (c Config) KindLower() string  { return strings.ToLower(c.KindName) }
func (c Config) KindPlural() string { return strings.ToLower(c.KindName) + "s" }
func (c Config) GroupName() string  { return strings.ToLower(c.KindName) + "s.grafana.app" }
func (c Config) APIVersion() string { return c.GroupName() + "/v1alpha1" }

// Generate creates the scaffolded project directory.
func Generate(cfg Config) error {
	dirs := []string{
		cfg.AppName,
		filepath.Join(cfg.AppName, "kinds"),
		filepath.Join(cfg.AppName, "plugin", "src"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	files := []struct {
		path string
		tmpl string
	}{
		{filepath.Join(cfg.AppName, "go.mod"), tmplGoMod},
		{filepath.Join(cfg.AppName, "main.go"), tmplMain},
		{filepath.Join(cfg.AppName, "kinds", cfg.KindLower()+".go"), tmplKind},
		{filepath.Join(cfg.AppName, "plugin", "src", "index.html"), tmplIndexHTML},
		{filepath.Join(cfg.AppName, "plugin", "src", "App.tsx"), tmplAppTsx},
		{filepath.Join(cfg.AppName, "plugin", "src", "plugin.json"), tmplPluginJSON},
		{filepath.Join(cfg.AppName, "plugin", "src", "types.ts"), tmplTypesTS},
		{filepath.Join(cfg.AppName, "Makefile"), tmplMakefile},
		{filepath.Join(cfg.AppName, "README.md"), tmplReadme},
	}

	for _, f := range files {
		if err := writeTemplate(f.path, f.tmpl, cfg); err != nil {
			return fmt.Errorf("write %s: %w", f.path, err)
		}
	}

	return nil
}

func writeTemplate(path, tmplStr string, data interface{}) error {
	t, err := template.New(filepath.Base(path)).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, data)
}
