package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceConfig describes a monorepo workspace to scaffold.
type WorkspaceConfig struct {
	// Dir is the target directory (created if absent, or initialized in-place).
	Dir string
	// ModulePrefix is the Go module prefix for the workspace, e.g. "github.com/myorg/myworkspace".
	ModulePrefix string
}

// WorkspaceInfo describes a detected workspace root.
type WorkspaceInfo struct {
	// Dir is the absolute path to the workspace root (where go.work lives).
	Dir string
	// Apps lists app directory names found under apps/.
	Apps []string
}

// GenerateWorkspace scaffolds a monorepo workspace layout:
//
//	<dir>/
//	  go.work          # ties together the kinds module and all app modules
//	  kinds/           # shared kinds CUE module (ready for grafana-app-sdk generate)
//	    cue.mod/
//	      module.cue
//	  apps/            # independent app modules go here
//	  Makefile
//	  README.md
func GenerateWorkspace(cfg WorkspaceConfig) error {
	dirs := []string{
		cfg.Dir,
		filepath.Join(cfg.Dir, "kinds", "cue.mod"),
		filepath.Join(cfg.Dir, "apps"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	// Derive CUE module name (must have a domain in the first segment)
	cueMod := cfg.ModulePrefix + "/kinds"
	if segs := strings.Split(cueMod, "/"); !strings.Contains(segs[0], ".") {
		segs[0] += ".grafana.app"
		cueMod = strings.Join(segs, "/")
	}

	files := map[string]string{
		filepath.Join(cfg.Dir, "go.work"):                  workspaceTmplGoWork,
		filepath.Join(cfg.Dir, "kinds", "cue.mod", "module.cue"): fmt.Sprintf("module: %q\nlanguage: version: \"v0.8.2\"\n", cueMod),
		filepath.Join(cfg.Dir, "Makefile"):                 workspaceTmplMakefile,
		filepath.Join(cfg.Dir, "README.md"):                workspaceTmplReadme,
	}

	for path, content := range files {
		if err := writeFile(path, []byte(content)); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}

// appendGoWork adds a new module path to an existing go.work use block.
// It is idempotent — calling it twice with the same relPath is safe.
func appendGoWork(workspaceDir, relPath string) error {
	goWorkPath := filepath.Join(workspaceDir, "go.work")
	data, err := os.ReadFile(goWorkPath)
	if err != nil {
		return fmt.Errorf("read go.work: %w", err)
	}

	content := string(data)
	directive := "./" + relPath
	if strings.Contains(content, directive) {
		return nil // already present
	}

	if idx := strings.Index(content, "use ("); idx >= 0 {
		closeIdx := strings.Index(content[idx:], ")")
		if closeIdx >= 0 {
			insertAt := idx + closeIdx
			content = content[:insertAt] + "\t" + directive + "\n" + content[insertAt:]
		}
	} else {
		content += "\nuse " + directive + "\n"
	}

	return os.WriteFile(goWorkPath, []byte(content), 0644)
}

// DetectWorkspace walks up from dir until it finds a go.work file.
// Returns nil (no error) if no workspace is found within the filesystem root.
func DetectWorkspace(dir string) (*WorkspaceInfo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	current := abs
	for {
		if _, err := os.Stat(filepath.Join(current, "go.work")); err == nil {
			apps, _ := listWorkspaceApps(filepath.Join(current, "apps"))
			return &WorkspaceInfo{Dir: current, Apps: apps}, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil, nil
}

func listWorkspaceApps(appsDir string) ([]string, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ResolveWorkspaceAppDir returns the absolute path to apps/<appName> within the
// workspace, verifying that the directory exists.
func ResolveWorkspaceAppDir(workspaceDir, appName string) (string, error) {
	appDir := filepath.Join(workspaceDir, "apps", appName)
	info, err := os.Stat(appDir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("app %q not found under %s/apps/", appName, workspaceDir)
	}
	return appDir, nil
}

// --- templates ---

const workspaceTmplGoWork = `go 1.22

use (
	./kinds
)
`

const workspaceTmplMakefile = `.PHONY: build test generate clean apps

build:
	@for d in apps/*/; do \
		echo "→ building $$d"; \
		(cd "$$d" && go build ./...) || exit 1; \
	done

test:
	@for d in apps/*/; do \
		echo "→ testing $$d"; \
		(cd "$$d" && go test ./...) || exit 1; \
	done

generate:
	@for d in apps/*/; do \
		app=$$(basename "$$d"); \
		echo "→ generating $$app"; \
		grafana-app-sdk generate --app="$$app" --source="$$d/kinds"; \
	done

apps:
	@ls -1 apps/ 2>/dev/null || echo "(no apps yet)"

clean:
	@for d in apps/*/; do \
		(cd "$$d" && go clean ./...) 2>/dev/null || true; \
	done
`

const workspaceTmplReadme = `# Grafana App Platform Workspace

A monorepo workspace for multiple Grafana app-platform apps.

## Structure

` + "```" + `
.
├── go.work         # Go workspace — ties all modules together
├── kinds/          # Shared Kind definitions (CUE module)
│   └── cue.mod/
├── apps/           # Independent app modules
│   ├── app1/
│   └── app2/
└── Makefile
` + "```" + `

## Adding a New App

` + "```" + `bash
grafana-app-sdk project add-app <app-name> <module-path>
` + "```" + `

Scaffolds ` + "`" + `apps/<app-name>/` + "`" + ` and updates ` + "`" + `go.work` + "`" + ` automatically.

## Selective Codegen

` + "```" + `bash
# Generate code for one app
grafana-app-sdk generate --app=<app-name>

# Generate for all apps
make generate
` + "```" + `

## Common Tasks

` + "```" + `bash
make build      # Build all apps
make test       # Test all modules
make generate   # Run codegen for all apps
make apps       # List apps
` + "```" + `
`
