package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/grafana/grafana-app-sdk/quickstart/scaffold"
)

var quickstartCmd = &cobra.Command{
	Use:          "quickstart <app-name>",
	Short:        "Scaffold a new Grafana app and start it locally (no Kubernetes required)",
	Long:         "Scaffold a minimal Grafana app with embedded API server and SQLite storage, then start it locally.\nNo Docker, K3D, Tilt, or Kubernetes required.\n\nExample:\n  grafana-app-sdk quickstart my-bookmarks\n  grafana-app-sdk quickstart my-app --kind Task --port 8080",
	SilenceUsage: true,
	RunE:         runQuickstart,
}

func setupQuickstartCmd() {
	quickstartCmd.Flags().String("kind", "", "Kind name (default: derived from app-name)")
	quickstartCmd.Flags().String("module", "", "Go module path (default: github.com/example/<app-name>)")
	quickstartCmd.Flags().Int("port", 3000, "Local API server port")
	quickstartCmd.Flags().Bool("no-open", false, "Don't auto-open browser after start")
	quickstartCmd.Flags().Bool("no-start", false, "Scaffold only, don't start the server")
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("app-name is required\n\nUsage: grafana-app-sdk quickstart <app-name> [flags]")
	}

	appName := args[0]
	if err := validateQuickstartAppName(appName); err != nil {
		return err
	}

	port, _ := cmd.Flags().GetInt("port")
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	kind, _ := cmd.Flags().GetString("kind")
	if kind == "" {
		kind = quickstartToKindName(appName)
	}

	module, _ := cmd.Flags().GetString("module")
	if module == "" {
		module = "github.com/example/" + appName
	}

	noOpen, _ := cmd.Flags().GetBool("no-open")
	noStart, _ := cmd.Flags().GetBool("no-start")

	// Check if directory already exists
	if _, err := os.Stat(appName); err == nil {
		return fmt.Errorf("directory %q already exists", appName)
	}

	fmt.Printf("Creating %s...\n\n", appName)

	// Step 1: Scaffold
	fmt.Println("-> Scaffolding app structure...")
	sc := scaffold.Config{
		AppName:    appName,
		KindName:   kind,
		ModulePath: module,
		Port:       port,
	}
	if err := scaffold.Generate(sc); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}
	fmt.Println("   Created project files")

	// Step 2: Initialize Go module
	fmt.Println("\n-> Initializing Go module...")
	if err := quickstartRunInDir(appName, "go", "mod", "tidy"); err != nil {
		fmt.Println("   go mod tidy failed (run manually after installing Go)")
	} else {
		fmt.Println("   Dependencies resolved")
	}

	// Step 3: Start the server (unless --no-start)
	if noStart {
		printQuickstartPostScaffold(appName, kind, port)
		return nil
	}

	fmt.Println("\n-> Starting local development server...")
	fmt.Printf("   API server: http://localhost:%d\n", port)
	fmt.Printf("   Health:     http://localhost:%d/healthz\n\n", port)

	if !noOpen {
		go quickstartOpenBrowser(fmt.Sprintf("http://localhost:%d", port))
	}

	runCmd := exec.Command("go", "run", ".")
	runCmd.Dir = appName
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	return runCmd.Run()
}

func printQuickstartPostScaffold(appName, kind string, port int) {
	fmt.Printf(`
App scaffolded successfully!

Next steps:
  cd %s
  go mod tidy
  go run .

Your app will start at http://localhost:%d

Project structure:
  %s/
    main.go              # App entrypoint — starts embedded server
    kinds/
      %s.go          # Kind definition with struct tags
    plugin/
      src/
        App.tsx          # React CRUD UI
        plugin.json      # Plugin metadata
    go.mod
    Makefile
    README.md
`, appName, port, appName, strings.ToLower(kind))
}

func validateQuickstartAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app-name cannot be empty")
	}
	if strings.Contains(name, "..") || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("app-name %q contains invalid path characters", name)
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return fmt.Errorf("app-name %q contains invalid character %q; use letters, digits, hyphens, or underscores", name, r)
		}
	}
	return nil
}

func quickstartToKindName(appName string) string {
	parts := strings.FieldsFunc(appName, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			result.WriteString(string(runes))
		}
	}
	name := result.String()
	if name == "" {
		return "MyApp"
	}
	return name
}

func quickstartRunInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func quickstartOpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	dir, err := filepath.Abs(".")
	if err == nil {
		cmd.Dir = dir
	}
	_ = cmd.Run()
}
