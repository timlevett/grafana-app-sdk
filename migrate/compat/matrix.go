// Package compat provides SDK/Grafana version compatibility checks.
// Use grafana-app-sdk compat to run a pre-flight check before a migration or CI deploy.
package compat

// Pair represents a known compatibility relationship between an SDK version and a Grafana core version.
type Pair struct {
	SDKVersion     string
	GrafanaVersion string
	Compatible     bool
	Note           string
}

// matrix is the known compatibility matrix.
// Add new entries here as new SDK versions are released.
var matrix = []Pair{
	{SDKVersion: "v0.26", GrafanaVersion: "10.0", Compatible: true},
	{SDKVersion: "v0.26", GrafanaVersion: "10.1", Compatible: true},
	{SDKVersion: "v0.27", GrafanaVersion: "10.1", Compatible: true},
	{SDKVersion: "v0.27", GrafanaVersion: "10.2", Compatible: true},
	{SDKVersion: "v0.28", GrafanaVersion: "10.2", Compatible: true},
	{SDKVersion: "v0.28", GrafanaVersion: "10.3", Compatible: true},
	{SDKVersion: "v0.29", GrafanaVersion: "10.3", Compatible: true},
	{SDKVersion: "v0.29", GrafanaVersion: "10.4", Compatible: true},
	{SDKVersion: "v0.30", GrafanaVersion: "10.4", Compatible: true},
	{SDKVersion: "v0.30", GrafanaVersion: "11.0", Compatible: true},
	{SDKVersion: "v0.26", GrafanaVersion: "11.0", Compatible: false, Note: "SDK v0.26 does not support Grafana 11 APIs"},
	{SDKVersion: "v0.27", GrafanaVersion: "11.0", Compatible: false, Note: "SDK v0.27 does not support Grafana 11 APIs"},
}

// Matrix returns a copy of the compatibility matrix.
func Matrix() []Pair {
	out := make([]Pair, len(matrix))
	copy(out, matrix)
	return out
}
