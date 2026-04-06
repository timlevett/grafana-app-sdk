package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// KindDef describes a Kind parsed from Go struct tags.
type KindDef struct {
	Name        string // e.g. "Bookmark"
	Group       string // e.g. "bookmarks.grafana.app"
	Version     string // e.g. "v1alpha1"
	Scope       string // "Namespaced" or "Cluster"
	Description string // from +description annotation; used in MCP tool descriptions
	Fields      []FieldDef
}

// FieldDef describes a single field in a Kind spec.
type FieldDef struct {
	GoName      string // Go field name
	JSONName    string // from json tag
	GoType      string // "string", "int", etc.
	Required    bool   // from validate:"required"
	Description string // from description tag
	Omitempty   bool
}

// ParseKindFile reads a Go source file and extracts Kind definitions from
// annotated structs. Annotations are Go comments of the form:
//
//	// +kind:Bookmark
//	// +description:Manages saved links for quick access from dashboards
//	// +group:bookmarks.grafana.app
//	// +version:v1alpha1
//	// +scope:Namespaced
//
// Supported annotations:
//
//	+kind:<Name>          Required. PascalCase Kind name.
//	+description:<text>   Recommended. Human-readable description for agent tool generation.
//	+group:<group>        Optional. API group (default: inferred from kind name).
//	+version:<version>    Optional. API version (default: v1alpha1).
//	+scope:<scope>        Optional. "Namespaced" (default) or "Cluster".
func ParseKindFile(filename string) ([]KindDef, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	var kinds []KindDef

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Collect annotations from the doc comment
			annotations := extractAnnotations(genDecl.Doc)
			kindName, hasKind := annotations["kind"]
			if !hasKind {
				continue
			}

			kind := KindDef{
				Name:        kindName,
				Group:       annotations["group"],
				Version:     annotations["version"],
				Scope:       annotations["scope"],
				Description: annotations["description"],
			}
			if kind.Version == "" {
				kind.Version = "v1alpha1"
			}
			if kind.Scope == "" {
				kind.Scope = "Namespaced"
			}

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue // embedded field
				}
				fd := parseField(field)
				kind.Fields = append(kind.Fields, fd)
			}

			kinds = append(kinds, kind)
		}
	}

	return kinds, nil
}

// ParseKindSource parses Kind definitions from Go source code provided as a string.
func ParseKindSource(src string) ([]KindDef, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "kind.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}

	var kinds []KindDef
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			annotations := extractAnnotations(genDecl.Doc)
			kindName, hasKind := annotations["kind"]
			if !hasKind {
				continue
			}
			kind := KindDef{
				Name:        kindName,
				Group:       annotations["group"],
				Version:     annotations["version"],
				Scope:       annotations["scope"],
				Description: annotations["description"],
			}
			if kind.Version == "" {
				kind.Version = "v1alpha1"
			}
			if kind.Scope == "" {
				kind.Scope = "Namespaced"
			}
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue
				}
				fd := parseField(field)
				kind.Fields = append(kind.Fields, fd)
			}
			kinds = append(kinds, kind)
		}
	}
	return kinds, nil
}

func extractAnnotations(doc *ast.CommentGroup) map[string]string {
	result := make(map[string]string)
	if doc == nil {
		return result
	}
	for _, comment := range doc.List {
		text := strings.TrimPrefix(comment.Text, "//")
		text = strings.TrimSpace(text)
		if !strings.HasPrefix(text, "+") {
			continue
		}
		text = strings.TrimPrefix(text, "+")
		parts := strings.SplitN(text, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func parseField(field *ast.Field) FieldDef {
	fd := FieldDef{
		GoName: field.Names[0].Name,
		GoType: typeString(field.Type),
	}

	if field.Tag != nil {
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

		if jsonTag, ok := tag.Lookup("json"); ok {
			parts := strings.Split(jsonTag, ",")
			fd.JSONName = parts[0]
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					fd.Omitempty = true
				}
			}
		}
		if fd.JSONName == "" {
			fd.JSONName = fd.GoName
		}

		if validate, ok := tag.Lookup("validate"); ok {
			for _, rule := range strings.Split(validate, ",") {
				if rule == "required" {
					fd.Required = true
				}
			}
		}

		if desc, ok := tag.Lookup("description"); ok {
			fd.Description = desc
		}
	}

	if fd.JSONName == "" {
		fd.JSONName = fd.GoName
	}

	return fd
}

func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	default:
		return "interface{}"
	}
}
