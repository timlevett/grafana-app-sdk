package deprecations

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Warning is a single deprecation finding.
type Warning struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Symbol     string `json:"symbol"`
	Since      string `json:"deprecated_since"`
	RemovedIn  string `json:"removed_in,omitempty"`
	Replace    string `json:"replacement,omitempty"`
	GuideURL   string `json:"migration_guide,omitempty"`
}

// String returns the human-readable form of the warning.
func (w Warning) String() string {
	msg := fmt.Sprintf("WARNING %s:%d: %s is deprecated since %s", w.File, w.Line, w.Symbol, w.Since)
	if w.RemovedIn != "" {
		msg += fmt.Sprintf(", removed in %s", w.RemovedIn)
	}
	if w.Replace != "" {
		msg += fmt.Sprintf("\n  → Use %s instead", w.Replace)
	}
	if w.GuideURL != "" {
		msg += fmt.Sprintf("\n  → See: %s", w.GuideURL)
	}
	return msg
}

// Scanner scans a project directory for deprecated symbol usage.
type Scanner struct {
	registry *Registry
}

// NewScanner returns a Scanner backed by the given registry.
func NewScanner(r *Registry) *Scanner {
	return &Scanner{registry: r}
}

// Scan walks dir recursively for .go files and returns all deprecation warnings found.
func (s *Scanner) Scan(dir string) ([]Warning, error) {
	var warnings []Warning
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
		ws, err := s.scanFile(path, dir)
		if err != nil {
			return err
		}
		warnings = append(warnings, ws...)
		return nil
	})
	return warnings, err
}

func (s *Scanner) scanFile(path, root string) ([]Warning, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rel, _ := filepath.Rel(root, path)
	var warnings []Warning
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		for _, entry := range s.registry.Entries() {
			if strings.Contains(line, entry.Symbol) {
				warnings = append(warnings, Warning{
					File:      rel,
					Line:      lineNo,
					Symbol:    entry.Symbol,
					Since:     entry.DeprecatedSince,
					RemovedIn: entry.RemovedIn,
					Replace:   entry.Replacement,
					GuideURL:  entry.MigrationGuide,
				})
			}
		}
	}
	return warnings, scanner.Err()
}

// PrintText writes warnings in human-readable form to w.
func PrintText(w io.Writer, warnings []Warning) {
	for _, warn := range warnings {
		fmt.Fprintln(w, warn.String())
	}
}

// PrintJSON writes warnings as a JSON array to w.
func PrintJSON(w io.Writer, warnings []Warning) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(warnings)
}
