package scaffold

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidationError represents a single annotation validation issue.
type ValidationError struct {
	Kind    string // Kind name (may be empty if name itself is invalid)
	Field   string // which annotation
	Message string
}

func (e ValidationError) Error() string {
	if e.Kind != "" {
		return fmt.Sprintf("%s: %s: %s", e.Kind, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateKindDef checks that a KindDef has valid annotation values.
// Returns all validation errors found.
//
// Rules:
//   - Name must be non-empty, PascalCase (starts uppercase, no hyphens/underscores/spaces).
//   - Description should be non-empty (recommended for agent tool generation).
//   - Version must be non-empty and start with "v".
//   - Scope must be "Namespaced" or "Cluster".
func ValidateKindDef(kd KindDef) []ValidationError {
	var errs []ValidationError

	if kd.Name == "" {
		errs = append(errs, ValidationError{Field: "+kind", Message: "kind name is required"})
	} else {
		if !unicode.IsUpper(rune(kd.Name[0])) {
			errs = append(errs, ValidationError{Kind: kd.Name, Field: "+kind", Message: "must start with an uppercase letter (PascalCase)"})
		}
		if strings.ContainsAny(kd.Name, "-_ ") {
			errs = append(errs, ValidationError{Kind: kd.Name, Field: "+kind", Message: "must be PascalCase (no hyphens, underscores, or spaces)"})
		}
	}

	if kd.Description == "" {
		errs = append(errs, ValidationError{Kind: kd.Name, Field: "+description", Message: "description is recommended for agent tool generation"})
	}

	if kd.Version == "" {
		errs = append(errs, ValidationError{Kind: kd.Name, Field: "+version", Message: "must not be empty"})
	} else if !strings.HasPrefix(kd.Version, "v") {
		errs = append(errs, ValidationError{Kind: kd.Name, Field: "+version", Message: "must start with 'v' (e.g. v1, v1alpha1)"})
	}

	if kd.Scope != "Namespaced" && kd.Scope != "Cluster" {
		errs = append(errs, ValidationError{Kind: kd.Name, Field: "+scope", Message: fmt.Sprintf("must be %q or %q", "Namespaced", "Cluster")})
	}

	return errs
}
