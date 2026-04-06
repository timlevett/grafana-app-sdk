package compat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result is the outcome of a compatibility check.
type Result struct {
	SDKVersion     string
	GrafanaVersion string
	Compatible     bool
	Note           string
	// Unknown is true when no entry was found in the matrix for the version pair.
	Unknown bool
}

// Check determines whether the given SDK and Grafana version are compatible.
// If compatible is indeterminate (not in the matrix) it returns Unknown=true.
func Check(sdkVersion, grafanaVersion string) Result {
	sdkMinor := minorVersion(sdkVersion)
	grafMinor := minorVersion(grafanaVersion)

	for _, p := range matrix {
		if p.SDKVersion == sdkMinor && p.GrafanaVersion == grafMinor {
			return Result{
				SDKVersion:     sdkVersion,
				GrafanaVersion: grafanaVersion,
				Compatible:     p.Compatible,
				Note:           p.Note,
			}
		}
	}
	return Result{
		SDKVersion:     sdkVersion,
		GrafanaVersion: grafanaVersion,
		Compatible:     true, // assume compatible when unknown
		Unknown:        true,
	}
}

// CheckFromGoMod reads go.mod in dir, extracts the grafana-app-sdk version,
// and checks it against the provided Grafana version. If grafanaVersion is empty,
// it attempts to read it from a `grafana-version` file.
func CheckFromGoMod(dir, grafanaVersion string) (*Result, error) {
	sdkVer, err := readSDKVersionFromGoMod(dir)
	if err != nil {
		return nil, err
	}
	if grafanaVersion == "" {
		grafanaVersion, err = readGrafanaVersionFile(dir)
		if err != nil {
			return nil, fmt.Errorf("grafana-version not provided and could not be read: %w", err)
		}
	}
	r := Check(sdkVer, grafanaVersion)
	return &r, nil
}

func readSDKVersionFromGoMod(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "github.com/grafana/grafana-app-sdk ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("grafana-app-sdk not found in go.mod")
}

func readGrafanaVersionFile(dir string) (string, error) {
	for _, name := range []string{"grafana-version", ".grafana-version", "GRAFANA_VERSION"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}
	return "", fmt.Errorf("no grafana-version file found in %s", dir)
}

// minorVersion returns "vMAJOR.MINOR" from a full semver string.
// e.g. "v0.28.3" → "v0.28", "10.2.1" → "10.2"
func minorVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}
