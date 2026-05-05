package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// TestMCPErrorCodesInTaxonomy scans all Go source files in this package for
// MCPSimpleError, mcpParseError, mcpParseErrorWithFix, and FromJoinedError
// calls with string-literal codes, and asserts every such code is declared in
// diagnostics.AllCodes(). This catches undeclared or inconsistently-cased codes.
func TestMCPErrorCodesInTaxonomy(t *testing.T) {
	allowed := make(map[string]bool)
	for _, c := range diagnostics.AllCodes() {
		allowed[c] = true
	}

	// Patterns that extract the first string argument (the code) from calls.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`MCPSimpleError\("([^"]+)"`),
		regexp.MustCompile(`mcpParseError\("([^"]+)"`),
		regexp.MustCompile(`mcpParseErrorWithFix\("([^"]+)"`),
		regexp.MustCompile(`FromJoinedError\([^,]+,\s*"([^"]+)"`),
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		lines := strings.Split(string(data), "\n")
		for lineNum, line := range lines {
			for _, pat := range patterns {
				matches := pat.FindAllStringSubmatch(line, -1)
				for _, m := range matches {
					code := m[1]
					if !allowed[code] {
						t.Errorf("%s:%d: undeclared error code %q — add it to internal/diagnostics/codes.go",
							f, lineNum+1, code)
					}
				}
			}
		}
	}
}
