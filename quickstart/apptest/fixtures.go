package apptest

import "github.com/grafana/grafana-app-sdk/quickstart/local"

// BookmarkResource is the standard GVR for the Bookmark Manager example app.
var BookmarkResource = local.GroupVersionResource{
	Group:    "bookmarks.grafana.app",
	Version:  "v1alpha1",
	Resource: "bookmarks",
	Kind:     "Bookmark",
}

// BookmarkSpec is the spec for a bookmark resource.
type BookmarkSpec struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// KindValidationRule describes a constraint that Kind struct-tag definitions
// must satisfy. Use ValidateKind to check a slice of rules against a KindDef.
type KindValidationRule struct {
	// Name is shown in test output.
	Name string
	// Check returns an error message if the rule is violated, or "" if OK.
	Check func(name, group, version, scope string, fields []KindField) string
}

// KindField is a field extracted from a Kind definition for validation.
type KindField struct {
	Name     string
	JSONName string
	Required bool
}

// ValidateKind runs rules against the given Kind descriptor. It returns a
// slice of error messages (empty means the Kind is valid).
func ValidateKind(name, group, version, scope string, fields []KindField, rules []KindValidationRule) []string {
	var errs []string
	for _, r := range rules {
		if msg := r.Check(name, group, version, scope, fields); msg != "" {
			errs = append(errs, r.Name+": "+msg)
		}
	}
	return errs
}

// StandardKindRules returns validation rules that every Kind should pass.
var StandardKindRules = []KindValidationRule{
	{
		Name: "name-not-empty",
		Check: func(name, _, _, _ string, _ []KindField) string {
			if name == "" {
				return "Kind name must not be empty"
			}
			return ""
		},
	},
	{
		Name: "group-not-empty",
		Check: func(_, group, _, _ string, _ []KindField) string {
			if group == "" {
				return "Kind group must not be empty"
			}
			return ""
		},
	},
	{
		Name: "version-not-empty",
		Check: func(_, _, version, _ string, _ []KindField) string {
			if version == "" {
				return "Kind version must not be empty"
			}
			return ""
		},
	},
	{
		Name: "group-has-dot",
		Check: func(_, group, _, _ string, _ []KindField) string {
			for _, c := range group {
				if c == '.' {
					return ""
				}
			}
			return "Kind group should contain a dot (e.g. myapp.grafana.app)"
		},
	},
	{
		Name: "fields-not-empty",
		Check: func(_, _, _, _ string, fields []KindField) string {
			if len(fields) == 0 {
				return "Kind must define at least one spec field"
			}
			return ""
		},
	},
	{
		Name: "fields-have-json-names",
		Check: func(_, _, _, _ string, fields []KindField) string {
			for _, f := range fields {
				if f.JSONName == "" {
					return "field " + f.Name + " is missing a json tag"
				}
			}
			return ""
		},
	},
}
