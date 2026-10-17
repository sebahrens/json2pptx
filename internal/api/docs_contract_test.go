package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Transport codes are a public recovery contract: adding one requires both
// human-readable status guidance and the machine-readable OpenAPI enum.
func TestHTTPAuditErrorDocumentation(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), filepath.Join("errors", "errors.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "docs", "api", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	openapi, err := os.ReadFile(filepath.Join("..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Enum []string `yaml:"enum"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(openapi, &spec); err != nil {
		t.Fatal(err)
	}
	enums := map[string]bool{}
	for _, code := range spec.Components.Schemas["ErrorDetail"].Properties["code"].Enum {
		enums[code] = true
	}
	count := 0
	ast.Inspect(source, func(node ast.Node) bool {
		decl, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range decl.Names {
			if !strings.HasPrefix(name.Name, "Code") {
				continue
			}
			lit, ok := decl.Values[i].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Fatalf("%s has non-string error code", name.Name)
			}
			code, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			count++
			if !strings.Contains(string(readme), "| `"+code+"` |") {
				t.Errorf("README error table missing %s", code)
			}
			if !enums[code] {
				t.Errorf("OpenAPI error enum missing %s", code)
			}
		}
		return true
	})
	if count == 0 {
		t.Fatal("no transport error codes checked")
	}
}
