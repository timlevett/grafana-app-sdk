package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateWorkspace(t *testing.T) {
	dir := t.TempDir()

	cfg := WorkspaceConfig{
		Dir:          dir,
		ModulePrefix: "github.com/myorg/myworkspace",
	}

	if err := GenerateWorkspace(cfg); err != nil {
		t.Fatalf("GenerateWorkspace: %v", err)
	}

	// Verify expected files
	expected := []string{
		"go.work",
		filepath.Join("kinds", "cue.mod", "module.cue"),
		"Makefile",
		"README.md",
	}
	for _, rel := range expected {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file not created: %s", rel)
		}
	}

	// go.work should reference kinds module
	goWork, err := os.ReadFile(filepath.Join(dir, "go.work"))
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	if got := string(goWork); !containsAll(got, "./kinds") {
		t.Errorf("go.work missing ./kinds reference:\n%s", got)
	}

	// module.cue should contain the module path with domain
	modCue, err := os.ReadFile(filepath.Join(dir, "kinds", "cue.mod", "module.cue"))
	if err != nil {
		t.Fatalf("read module.cue: %v", err)
	}
	if got := string(modCue); !containsAll(got, "github.com/myorg/myworkspace/kinds") {
		t.Errorf("module.cue missing expected module path:\n%s", got)
	}

	// apps/ directory must exist
	appsDir := filepath.Join(dir, "apps")
	if info, err := os.Stat(appsDir); err != nil || !info.IsDir() {
		t.Errorf("apps/ directory not created")
	}
}

func TestGenerateWorkspace_DomainInsertion(t *testing.T) {
	dir := t.TempDir()
	cfg := WorkspaceConfig{Dir: dir, ModulePrefix: "myorg/myworkspace"}

	if err := GenerateWorkspace(cfg); err != nil {
		t.Fatalf("GenerateWorkspace: %v", err)
	}

	modCue, _ := os.ReadFile(filepath.Join(dir, "kinds", "cue.mod", "module.cue"))
	// The first segment must have a dot injected so CUE treats it as a domain
	if got := string(modCue); !containsAll(got, "myorg.grafana.app") {
		t.Errorf("module.cue expected domain injection, got:\n%s", got)
	}
}

func TestDetectWorkspace_Found(t *testing.T) {
	root := t.TempDir()
	// Write a go.work file
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create apps/myapp
	appDir := filepath.Join(root, "apps", "myapp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Detect from a subdirectory
	subDir := filepath.Join(root, "apps", "myapp", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(subDir)
	if err != nil {
		t.Fatalf("DetectWorkspace: %v", err)
	}
	if ws == nil {
		t.Fatal("expected workspace to be detected, got nil")
	}
	if ws.Dir != root {
		t.Errorf("expected workspace dir %s, got %s", root, ws.Dir)
	}
	if len(ws.Apps) != 1 || ws.Apps[0] != "myapp" {
		t.Errorf("expected apps=[myapp], got %v", ws.Apps)
	}
}

func TestDetectWorkspace_NotFound(t *testing.T) {
	dir := t.TempDir()
	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != nil {
		t.Errorf("expected nil workspace, got %+v", ws)
	}
}

func TestAppendGoWork(t *testing.T) {
	dir := t.TempDir()
	initial := "go 1.22\n\nuse (\n\t./kinds\n)\n"
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := appendGoWork(dir, "apps/myapp"); err != nil {
		t.Fatalf("appendGoWork: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "go.work"))
	if !containsAll(string(got), "./apps/myapp") {
		t.Errorf("go.work after append:\n%s", string(got))
	}

	// Idempotent — calling again should not duplicate
	if err := appendGoWork(dir, "apps/myapp"); err != nil {
		t.Fatalf("second appendGoWork: %v", err)
	}
	got2, _ := os.ReadFile(filepath.Join(dir, "go.work"))
	count := 0
	for i := 0; i < len(got2); {
		j := i + len("./apps/myapp")
		if j <= len(got2) && string(got2[i:j]) == "./apps/myapp" {
			count++
		}
		i++
	}
	if count > 1 {
		t.Errorf("duplicate entry in go.work: found %d occurrences", count)
	}
}

func TestResolveWorkspaceAppDir(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "myapp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveWorkspaceAppDir(root, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != appDir {
		t.Errorf("expected %s, got %s", appDir, got)
	}

	_, err = ResolveWorkspaceAppDir(root, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent app, got nil")
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
